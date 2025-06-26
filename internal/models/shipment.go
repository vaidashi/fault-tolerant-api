package models

import (
	"time"
)

// Shipment represents a shipment in our system
type Shipment struct {
	ID             string    `db:"id" json:"id"`
	OrderID        string    `db:"order_id" json:"order_id"`
	ShipmentID     string    `db:"shipment_id" json:"shipment_id"`
	TrackingNumber string    `db:"tracking_number" json:"tracking_number"`
	Status         string    `db:"status" json:"status"`
	CreatedAt      time.Time `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time `db:"updated_at" json:"updated_at"`
}

// ShipmentStatus defines the possible statuses for a shipment
type ShipmentStatus string

const (
	ShipmentStatusPending   ShipmentStatus = "pending"
	ShipmentStatusShipped   ShipmentStatus = "shipped"
	ShipmentStatusDelivered ShipmentStatus = "delivered"
	ShipmentStatusFailed    ShipmentStatus = "failed"
)

// NewShipment creates a new shipment with default values
func NewShipment(orderID, shipmentID, trackingNumber, status string) *Shipment {
	now := time.Now()
	return &Shipment{
		ID:             GenerateID("shp"),
		OrderID:        orderID,
		ShipmentID:     shipmentID,
		TrackingNumber: trackingNumber,
		Status:         status,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}