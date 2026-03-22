package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	dockerclient "github.com/docker/docker/client"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

// node mirrors the API server's node JSON response
type node struct {
	ID        uuid.UUID `json:"ID"`
	ClusterID uuid.UUID `json:"ClusterID"`
	Name      string    `json:"Name"`
	Status    string    `json:"Status"`
	CpuMillis int32     `json:"CpuMillis"`
	MemoryMb  int32     `json:"MemoryMb"`
}

// pod mirrors the API server's pod JSON response
type pod struct {
	ID            uuid.UUID  `json:"ID"`
	ClusterID     uuid.UUID  `json:"ClusterID"`
	NodeID        *uuid.UUID `json:"NodeID"`
	Name          string     `json:"Name"`
	Image         string     `json:"Image"`
	Status        string     `json:"Status"`
	CpuRequest    int32      `json:"CpuRequest"`
	MemoryRequest int32      `json:"MemoryRequest"`
}

func main() {
	_ = godotenv.Load()

	apiURL := getEnv("API_URL", "http://localhost:7777")
	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "node"
		}
		nodeName = fmt.Sprintf("%s-%s", hostname, uuid.New().String()[:8])
		log.Printf("NODE_NAME not set, using auto-generated name: %s", nodeName)
	}
	clusterID := os.Getenv("CLUSTER_ID") // optional

	cpuMillis := getEnvInt("CPU_MILLIS", 1000)
	memoryMb := getEnvInt("MEMORY_MB", 1024)

	// graceful shutdown via SIGINT/SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// connect to the local Docker daemon
	docker, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatalf("docker connect failed: %v", err)
	}
	defer docker.Close()

	// register with the API server
	n, err := registerNode(apiURL, nodeName, clusterID, int32(cpuMillis), int32(memoryMb))
	if err != nil {
		log.Fatalf("failed to register node: %v", err)
	}
	log.Printf("kubelet started: node %s (%s) in cluster %s with %dm CPU, %dMB memory",
		n.Name, n.ID, n.ClusterID, n.CpuMillis, n.MemoryMb)

	// mark node as not-ready on shutdown
	defer func() {
		if err := updateNodeStatus(apiURL, n.ID, "not-ready"); err != nil {
			log.Printf("failed to mark node as not-ready on shutdown: %v", err)
		} else {
			log.Printf("node %s marked as not-ready", n.Name)
		}
	}()

	// control loop
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	runScheduledPods(ctx, apiURL, docker, n)
	monitorRunningPods(ctx, apiURL, docker, n)
	sendHeartbeat(apiURL, n)

	for {
		select {
		case <-ctx.Done():
			log.Println("shutting down kubelet...")
			return
		case <-ticker.C:
			runScheduledPods(ctx, apiURL, docker, n)
			monitorRunningPods(ctx, apiURL, docker, n)
			sendHeartbeat(apiURL, n)
		}
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return fallback
	}
	return n
}

// --- API client functions ---

func registerNode(apiURL, nodeName, clusterID string, cpuMillis, memoryMb int32) (node, error) {
	body := map[string]interface{}{
		"node_name": nodeName,
		"cpu_millis": cpuMillis,
		"memory_mb":  memoryMb,
	}
	if clusterID != "" {
		body["cluster_id"] = clusterID
	}

	data, err := json.Marshal(body)
	if err != nil {
		return node{}, err
	}

	resp, err := http.Post(apiURL+"/api/v1/nodes/register", "application/json", bytes.NewReader(data))
	if err != nil {
		return node{}, fmt.Errorf("cannot reach API server at %s: %v", apiURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		msg, _ := io.ReadAll(resp.Body)
		return node{}, fmt.Errorf("registration failed (%d): %s", resp.StatusCode, string(msg))
	}

	var n node
	if err := json.NewDecoder(resp.Body).Decode(&n); err != nil {
		return node{}, err
	}
	return n, nil
}

func updateNodeStatus(apiURL string, nodeID uuid.UUID, status string) error {
	body, _ := json.Marshal(map[string]string{"status": status})
	req, err := http.NewRequest(http.MethodPatch, fmt.Sprintf("%s/api/v1/nodes/%s", apiURL, nodeID), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status update failed (%d): %s", resp.StatusCode, string(msg))
	}
	return nil
}

func fetchPods(apiURL string, nodeID uuid.UUID, status string) ([]pod, error) {
	url := fmt.Sprintf("%s/api/v1/nodes/%s/pods?status=%s", apiURL, nodeID, status)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch pods failed: %d", resp.StatusCode)
	}
	var pods []pod
	if err := json.NewDecoder(resp.Body).Decode(&pods); err != nil {
		return nil, err
	}
	return pods, nil
}

func updatePodStatus(apiURL string, clusterID, podID uuid.UUID, status string) error {
	body, _ := json.Marshal(map[string]string{"status": status})
	url := fmt.Sprintf("%s/api/v1/clusters/%s/pods/%s", apiURL, clusterID, podID)
	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("pod status update failed: %d", resp.StatusCode)
	}
	return nil
}

func sendHeartbeat(apiURL string, n node) {
	if err := updateNodeStatus(apiURL, n.ID, "ready"); err != nil {
		log.Printf("heartbeat failed: %v", err)
	}
}

// --- Pod management ---

func runScheduledPods(ctx context.Context, apiURL string, docker *dockerclient.Client, n node) {
	pods, err := fetchPods(apiURL, n.ID, "scheduled")
	if err != nil || len(pods) == 0 {
		return
	}

	for _, p := range pods {
		err := startContainer(ctx, docker, p)
		if err != nil {
			log.Printf("failed to start pod %s: %v", p.Name, err)
			if updateErr := updatePodStatus(apiURL, p.ClusterID, p.ID, "failed"); updateErr != nil {
				log.Printf("failed to mark pod %s as failed: %v", p.Name, updateErr)
			}
			continue
		}

		if err := updatePodStatus(apiURL, p.ClusterID, p.ID, "running"); err != nil {
			log.Printf("failed to update status for pod %s: %v", p.Name, err)
			continue
		}
		log.Printf("started pod %s (image: %s)", p.Name, p.Image)
	}
}

func monitorRunningPods(ctx context.Context, apiURL string, docker *dockerclient.Client, n node) {
	pods, err := fetchPods(apiURL, n.ID, "running")
	if err != nil || len(pods) == 0 {
		return
	}

	containers, err := docker.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		log.Printf("failed to list containers: %v", err)
		return
	}

	// build a lookup: container name -> state
	containerStates := make(map[string]string)
	for _, c := range containers {
		for _, name := range c.Names {
			containerStates[name] = c.State
		}
	}

	for _, p := range pods {
		expectedName := "/" + containerNameForPod(p)
		state, found := containerStates[expectedName]

		if !found {
			log.Printf("pod %s container not found, marking as failed", p.Name)
			updatePodStatus(apiURL, p.ClusterID, p.ID, "failed")
		} else if state != "running" {
			log.Printf("pod %s container exited (state: %s), marking as failed", p.Name, state)
			updatePodStatus(apiURL, p.ClusterID, p.ID, "failed")
		}
	}
}

func containerNameForPod(p pod) string {
	return fmt.Sprintf("%s-%s", p.Name, p.ID.String()[:8])
}

func startContainer(ctx context.Context, docker *dockerclient.Client, p pod) error {
	reader, err := docker.ImagePull(ctx, p.Image, image.PullOptions{})
	if err != nil {
		return err
	}
	io.Copy(io.Discard, reader)
	reader.Close()

	containerName := containerNameForPod(p)

	resp, err := docker.ContainerCreate(ctx, &container.Config{
		Image: p.Image,
		Labels: map[string]string{
			"mini-k8s.pod-id":   p.ID.String(),
			"mini-k8s.pod-name": p.Name,
		},
	}, &container.HostConfig{
		Resources: container.Resources{
			NanoCPUs: int64(p.CpuRequest) * 1e6,
			Memory:   int64(p.MemoryRequest) * 1024 * 1024,
		},
	}, nil, nil, containerName)
	if err != nil {
		return err
	}

	return docker.ContainerStart(ctx, resp.ID, container.StartOptions{})
}
