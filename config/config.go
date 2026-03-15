package config

import (
	"log"
	"os"
)

type Config struct{
	Port string
	DatabaseURL string
}

func Load() *Config {
	return &Config{
		Port: getEnv("PORT", "8080"),
		DatabaseURL: mustGetEnv("DATABASE_URL"),
	}
}


func getEnv(key, fallback string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return fallback
}

func mustGetEnv(key string) string {
	v := os.Getenv(key)
	if v == ""{
		log.Fatalf("Required environment variable %s not set", key)
	}
	return v
}