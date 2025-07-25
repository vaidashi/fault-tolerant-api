package config 

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port     int
	LogLevel string
	Env 	string
	DB DBConfig
	Kafka KafkaConfig
	WarehouseURL string
}

// DBConfig holds the database configuration
type DBConfig struct {
	Host     string
	Port     int
	User    string
	Password string
	Name     string
	SSLMode string
}

// KafkaConfig holds the Kafka configuration
type KafkaConfig struct {
	Brokers []string
	OrdersTopic string
	ConsumerGroup string
}

// getEnv retrieves the value of an environment variable or returns a default value if not set.
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}

	return defaultValue
}

// Load reads the configuration from environment variables and returns a Config struct.
func Load() (*Config, error) {
	port, err := strconv.Atoi(getEnv("PORT", "8080"))

	if err != nil {
		return nil, fmt.Errorf("invalid port: %w", err)
	}

	dbPort, err := strconv.Atoi(getEnv("DB_PORT", "5432"))

	if err != nil {
		return nil, fmt.Errorf("invalid DB_PORT: %w", err)
	}

	return &Config{
		Port:     port,
		LogLevel: getEnv("LOG_LEVEL", "info"),
		Env:      getEnv("APP_ENV", "development"),
		DB: DBConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     dbPort,
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", "postgres"),
			Name:     getEnv("DB_NAME", "ftapi"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Kafka: KafkaConfig{
			Brokers:      strings.Split(getEnv("KAFKA_BROKERS", "localhost:9092"), ","),
			OrdersTopic:  getEnv("KAFKA_ORDERS_TOPIC", "orders"),
			ConsumerGroup: getEnv("KAFKA_CONSUMER_GROUP", "orders-consumer"),
		},
		WarehouseURL: getEnv("WAREHOUSE_URL", "http://localhost:8081"),
	}, nil
}

// GetDBConnString returns the database connection string
func (c *Config) GetDBConnString() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.DB.Host, c.DB.Port, c.DB.User, c.DB.Password, c.DB.Name, c.DB.SSLMode)
}