package models

import (
	"time"
	"fmt"
	"github.com/google/uuid"
)

// GenerateID generates a new unique ID for an order
func GenerateID(prefix string) string {
	id := uuid.New().String()
	
	// Format the ID with the prefix
	return fmt.Sprintf("%s-%s", prefix, id[:8])
}

// GetCurrentTime returns the current time in UTC
func GetCurrentTime() time.Time {
	return time.Now().UTC()
}