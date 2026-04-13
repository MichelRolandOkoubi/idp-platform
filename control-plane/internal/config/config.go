package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	AppEnv       string
	Port         int
	DatabaseURL  string
	RedisURL     string
	JWTSecret    string
	MLEngineURL  string
	OtelEndpoint string
	KubeConfig   string
	K8sInCluster bool
	LogLevel     string
}

func Load() (*Config, error) {
	port, err := strconv.Atoi(getEnv("PORT", "8080"))
	if err != nil {
		return nil, fmt.Errorf("invalid PORT: %w", err)
	}

	dbURL := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		getEnv("DB_USER", "idp"),
		getEnv("DB_PASSWORD", "idppassword"),
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_PORT", "5432"),
		getEnv("DB_NAME", "idp"),
	)

	return &Config{
		AppEnv:       getEnv("APP_ENV", "development"),
		Port:         port,
		DatabaseURL:  getEnv("DATABASE_URL", dbURL),
		RedisURL:     getEnv("REDIS_URL", "redis://localhost:6379"),
		JWTSecret:    mustEnv("JWT_SECRET"),
		MLEngineURL:  getEnv("ML_ENGINE_URL", "http://localhost:8000"),
		OtelEndpoint: getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4317"),
		KubeConfig:   getEnv("KUBECONFIG", ""),
		K8sInCluster: getEnv("K8S_IN_CLUSTER", "false") == "true",
		LogLevel:     getEnv("LOG_LEVEL", "info"),
	}, nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func mustEnv(key string) string {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		panic(fmt.Sprintf("required environment variable %s is not set", key))
	}
	return v
}
