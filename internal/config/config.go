package config 

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port     int
	LogLevel string
	Env 	string
}

// getEnv retrieves the value of an environment variable or returns a default value if not set.
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}

	return defaultValue
}

func Load() (*Config, error) {
	port, err := strconv.Atoi(getEnv("PORT", "8080"))

	if err != nil {
		return nil, fmt.Errorf("invalid port: %w", err)
	}

	logLevel := getEnv("LOG_LEVEL", "info")
	env := getEnv("ENV", "development")

	return &Config{
		Port:     port,
		LogLevel: logLevel,
		Env:      env,
	}, nil
}