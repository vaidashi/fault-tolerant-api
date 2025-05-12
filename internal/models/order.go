package models

import (
	"time"
)

// Order represents an order in the system
type Order struct {
	ID          string    `db:"id" json:"id"`
	CustomerID  string    `db:"customer_id" json:"customer_id"`
	Amount      float64   `db:"amount" json:"amount"`
	Status      string    `db:"status" json:"status"`
	Description string    `db:"description" json:"description,omitempty"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

// OrderStatus represents the status of an order
type OrderStatus string
const (
	OrderStatusPending   OrderStatus = "pending"
	OrderStatusApproved  OrderStatus = "approved"
	OrderStatusRejected  OrderStatus = "rejected"
	OrderStatusShipped   OrderStatus = "shipped"
	OrderStatusDelivered OrderStatus = "delivered"
	OrderStatusCancelled  OrderStatus = "cancelled"
)

// NewOrder creates a new order
func NewOrder(customerID string, amount float64, description string) *Order {
	now := time.Now()

	return &Order{
		ID:          GenerateID("ord"),
		CustomerID:  customerID,
		Amount:      amount,
		Status:      string(OrderStatusPending),
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}