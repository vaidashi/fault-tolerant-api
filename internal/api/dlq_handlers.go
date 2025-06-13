package api

import (
	"encoding/json"
	"net/http"
	"errors"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/vaidashi/fault-tolerant-api/internal/repository"
	"github.com/vaidashi/fault-tolerant-api/internal/models"
)

// getDeadLettersHandler returns a list of dead letter messages
func (s *Server) getDeadLettersHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse pagination parameters
	page, err := strconv.Atoi(r.URL.Query().Get("page"))

	if err != nil || page < 1 {
		page = 1
	}

	pageSize, err := strconv.Atoi(r.URL.Query().Get("pageSize"))

	if err != nil || pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	// Calculate offset
	offset := (page - 1) * pageSize

	status := r.URL.Query().Get("status")

	// Just get pending messages for simplicity
	messages, err := s.dlqRepo.GetPendingMessages(ctx, pageSize)

	if err != nil {
		s.logger.Error("Failed to fetch dead letter messages", "error", err)
		s.respondWithError(w, http.StatusInternalServerError, "Failed to fetch dead letter messages")
		return
	}

	response := PaginationResponse{
		Items: messages,
		TotalCount: len(messages),
		Page: page,
		PageSize: pageSize,
		Offset: offset,
		Status: status,
	}

	s.respondWithJSON(w, http.StatusOK, ApiResponse{Success: true, Data: response})
}

// retryDeadLetterHandler attempts to retry a dead letter message
func (s *Server) retryDeadLetterHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.ParseInt(idStr, 10, 64)

	if err != nil {
		s.respondWithError(w, http.StatusBadRequest, "Invalid message ID")
		return
	}

	message, err := s.dlqRepo.GetMessage(ctx, id)

	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			s.respondWithError(w, http.StatusNotFound, "Dead letter message not found")
			return
		}
		s.logger.Error("Failed to fetch dead letter message", "error", err, "messageID", id)
		s.respondWithError(w, http.StatusInternalServerError, "Failed to fetch dead letter message")
		return
	}

	// Allow only pending messages to be retried
	if message.Status != string(models.DeadLetterStatusPending) {
		s.respondWithError(w, http.StatusBadRequest, "Only pending messages can be retried")
		return
	}

	// Mark as retrying
	if err := s.dlqRepo.MarkAsRetrying(ctx, id); err != nil {
		s.logger.Error("Failed to mark message as retrying", "error", err, "messageID", id)
		s.respondWithError(w, http.StatusInternalServerError, "Failed to mark message for retry")
		return
	}
	
	s.respondWithJSON(w, http.StatusOK, ApiResponse{
		Success: true,
		Data: map[string]string{
			"message": "Dead letter message marked for retry",
			"id":      idStr,
		},
	})
}

// discardDeadLetterHandler discards a dead letter message
func (s *Server) discardDeadLetterHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.ParseInt(idStr, 10, 64)

	if err != nil {
		s.respondWithError(w, http.StatusBadRequest, "Invalid message ID")
		return
	}

	// Parse request body for discard reason
	var req struct {
		Reason string `json:"reason"`
	}

	decoder := json.NewDecoder(r.Body)

	if err := decoder.Decode(&req); err != nil {
		s.respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()

	if req.Reason == "" {
		req.Reason = "No reason provided"
	}

		// Mark as discarded
	if err := s.dlqRepo.MarkAsDiscarded(ctx, id, req.Reason); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			s.respondWithError(w, http.StatusNotFound, "Dead letter message not found")
			return
		}
		s.logger.Error("Failed to discard message", "error", err, "messageID", id)
		s.respondWithError(w, http.StatusInternalServerError, "Failed to discard message")
		return
	}
	
	s.respondWithJSON(w, http.StatusOK, ApiResponse{
		Success: true,
		Data: map[string]string{
			"message": "Dead letter message discarded",
			"id":      idStr,
		},
	})
}