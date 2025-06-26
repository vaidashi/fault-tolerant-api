package clients

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
	"context"

	"github.com/vaidashi/fault-tolerant-api/pkg/errors"
	"github.com/vaidashi/fault-tolerant-api/pkg/logger"
	"github.com/vaidashi/fault-tolerant-api/pkg/retry"
)

// WarehouseClient is a client for interacting with the warehouse service.
type WarehouseClient struct {
	baseURL string
	httpClient *http.Client
	logger     logger.Logger
	retryConfig *retry.RetryConfig
}

// InventoryResponse represents the response from the inventory check endpoint
type InventoryResponse struct {
	ProductID        string `json:"product_id,omitempty"`
	AvailableQuantity int    `json:"available_quantity,omitempty"`
	WarehouseID      string `json:"warehouse_id,omitempty"`
	Error            string `json:"error,omitempty"`
	Code             string `json:"code,omitempty"`
	Timestamp        string `json:"timestamp,omitempty"`
}

// ShipmentRequest represents the request to create a shipment
type ShipmentRequest struct {
	OrderID    string `json:"order_id"`
	CustomerID string `json:"customer_id"`
	Products   []struct {
		ProductID string `json:"product_id"`
		Quantity  int    `json:"quantity"`
	} `json:"products"`
	ShippingAddress string `json:"shipping_address,omitempty"`
}

// ShipmentResponse represents the response from the create shipment endpoint
type ShipmentResponse struct {
	ShipmentID     string `json:"shipment_id,omitempty"`
	OrderID        string `json:"order_id,omitempty"`
	Status         string `json:"status,omitempty"`
	TrackingNumber string `json:"tracking_number,omitempty"`
	Error          string `json:"error,omitempty"`
	Code           string `json:"code,omitempty"`
	Timestamp      string `json:"timestamp,omitempty"`
}

// NewWarehouseClient creates a new WarehouseClient instance
func NewWarehouseClient(baseURL string, logger logger.Logger) *WarehouseClient {
	httpClient := &http.Client{
		Timeout: 5 * time.Second,
	}

	// Create retry config
	retryConfig := &retry.RetryConfig{
		MaxAttempts: 3,
		BackoffStrategy: &retry.ExponentialBackoff{
			InitialInterval: 500 * time.Millisecond,
			MaxInterval:     5 * time.Second,
			Multiplier:      1.5,
			JitterFactor:    0.2,
		},
		Logger: logger,
		RetryableErrors: []error{
			errors.ErrTimeout,
			errors.ErrTemporaryFailure,
			errors.ErrServiceUnavailable,
		},
	}

	return &WarehouseClient{
		baseURL:    baseURL,
		httpClient: httpClient,
		logger:     logger,
		retryConfig: retryConfig,
	}
}

// CheckInventory checks the inventory for a product
func (c *WarehouseClient) CheckInventory(ctx context.Context, productID string) (*InventoryResponse, error) {
	url := fmt.Sprintf("%s/api/v1/inventory/%s", c.baseURL, productID)

	var response *InventoryResponse

	// Retry logic
	retryFunc := func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)

		if err != nil {
			return errors.NewInternalError(fmt.Sprintf("failed to create request: %v", err))
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)

		if err != nil {
			// Check for timeout
			if err, ok := err.(net.Error); ok && err.Timeout() {
				return errors.NewTimeoutError("request timed out")
			}
			return errors.NewTemporaryError(fmt.Sprintf("failed to make request: %v", err))
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		
		if err != nil {
			return errors.NewInternalError(fmt.Sprintf("failed to read response body: %v", err))
		}

		// Check for non-200 status codes
		if resp.StatusCode >= 400 {
			if resp.StatusCode == http.StatusRequestTimeout || resp.StatusCode == http.StatusInternalServerError {
				return errors.NewTimeoutError("inventory check request timed out")
			}

			if resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == http.StatusInternalServerError {
				return errors.NewTemporaryError(fmt.Sprintf("warehouse service error: %d", resp.StatusCode))
			}

			return errors.NewAppError(
				errors.ErrInternal,
				fmt.Sprintf("warehouse service returned error: %d", resp.StatusCode),
				resp.StatusCode,
				false,
			)
		}

		// Parse the response
		response = &InventoryResponse{}

		if err := json.Unmarshal(body, response); err != nil {
			return errors.NewInternalError(fmt.Sprintf("failed to parse response: %v", err))
		}
		
		// Check for error in the response body
		if response.Error != "" {
			if response.Code == "TIMEOUT" {
				return errors.NewTimeoutError(response.Error)
			}
			return errors.NewTemporaryError(response.Error)
		}
		
		return nil
	}

	// Execute with retry
	err := retry.Retry(ctx, retryFunc, c.retryConfig)

	if err != nil {
		c.logger.Error("Failed to check inventory after retries", 
			"error", err, 
			"productID", productID)
		return nil, err
	}
	
	return response, nil
}

// CreateShipment creates a shipment for an order
func (c *WarehouseClient) CreateShipment(ctx context.Context, request *ShipmentRequest) (*ShipmentResponse, error) {
	url := fmt.Sprintf("%s/api/v1/shipments", c.baseURL)

	var response *ShipmentResponse

	// Retry logic
	retryFunc := func() error {
		reqBody, err := json.Marshal(request)

		if err != nil {
			return errors.NewInternalError(fmt.Sprintf("failed to marshal request: %v", err))
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(reqBody))

		if err != nil {
			return errors.NewInternalError(fmt.Sprintf("failed to create request: %v", err))
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)

		if err != nil {
			// Check for timeout
			if err, ok := err.(net.Error); ok && err.Timeout() {
				return errors.NewTimeoutError("shipment request timed out")
			}
			return errors.NewTemporaryError(fmt.Sprintf("failed to send request: %v", err))
		}
		defer resp.Body.Close()

		// Read the response body
		body, err := io.ReadAll(resp.Body)

		if err != nil {
			return errors.NewInternalError(fmt.Sprintf("failed to read response body: %v", err))
		}

		// Check for non-200 status codes
		if resp.StatusCode >= 400 {
			if resp.StatusCode == http.StatusRequestTimeout || resp.StatusCode == http.StatusGatewayTimeout {
				return errors.NewTimeoutError("shipment request timed out")
			}

			if resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == http.StatusInternalServerError {
				return errors.NewTemporaryError(fmt.Sprintf("warehouse service error: %d", resp.StatusCode))
			}

			return errors.NewAppError(
				errors.ErrInternal,
				fmt.Sprintf("warehouse service returned error: %d", resp.StatusCode),
				resp.StatusCode,
				false,
			)
		}

		// Parse the response
		response = &ShipmentResponse{}

		if err := json.Unmarshal(body, response); err != nil {
			return errors.NewInternalError(fmt.Sprintf("failed to parse response: %v", err))
		}

		// Check for error in the response body
		if response.Error != "" {
			if response.Code == "TIMEOUT" {
				return errors.NewTimeoutError(response.Error)
			}
			return errors.NewTemporaryError(response.Error)
		}
		
		return nil
	}

	// Execute with retry
	err := retry.Retry(ctx, retryFunc, c.retryConfig)

	if err != nil {
		c.logger.Error("Failed to create shipment after retries", 
			"error", err, 
			"orderID", request.OrderID)
		return nil, err
	}
	return response, nil
}

// GetShipmentStatus gets the status of a shipment
func (c *WarehouseClient) GetShipmentStatus(ctx context.Context, shipmentID string) (*ShipmentResponse, error) {
	url := fmt.Sprintf("%s/api/shipments/%s", c.baseURL, shipmentID)
	
	var response *ShipmentResponse
	
	// Define the function to retry
	retryFunc := func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)

		if err != nil {
			return errors.NewInternalError(fmt.Sprintf("failed to create request: %v", err))
		}
		
		req.Header.Set("Content-Type", "application/json")
		
		resp, err := c.httpClient.Do(req)

		if err != nil {
			// Check for timeout
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				return errors.NewTimeoutError("shipment status request timed out")
			}
			return errors.NewTemporaryError(fmt.Sprintf("failed to send request: %v", err))
		}
		defer resp.Body.Close()
		
		// Read the response body
		body, err := io.ReadAll(resp.Body)

		if err != nil {
			return errors.NewInternalError(fmt.Sprintf("failed to read response body: %v", err))
		}
		
		// Check for error response codes
		if resp.StatusCode >= 400 {
			if resp.StatusCode == http.StatusRequestTimeout || resp.StatusCode == http.StatusGatewayTimeout {
				return errors.NewTimeoutError("shipment status request timed out")
			}

			if resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == http.StatusInternalServerError {
				return errors.NewTemporaryError(fmt.Sprintf("warehouse service error: %d", resp.StatusCode))
			}

			return errors.NewAppError(
				errors.ErrInternal,
				fmt.Sprintf("warehouse service returned error: %d", resp.StatusCode),
				resp.StatusCode,
				false,
			)
		}
		
		// Parse the response
		response = &ShipmentResponse{}

		if err := json.Unmarshal(body, response); err != nil {
			return errors.NewInternalError(fmt.Sprintf("failed to parse response: %v", err))
		}
		
		// Check for error in the response body
		if response.Error != "" {
			if response.Code == "TIMEOUT" {
				return errors.NewTimeoutError(response.Error)
			}
			return errors.NewTemporaryError(response.Error)
		}
		
		return nil
	}
	
	// Execute with retry
	err := retry.Retry(ctx, retryFunc, c.retryConfig)
	
	if err != nil {
		c.logger.Error("Failed to get shipment status after retries", 
			"error", err, 
			"shipmentID", shipmentID)
		return nil, err
	}
	
	return response, nil
}