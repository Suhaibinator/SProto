package response

import (
	"encoding/json"
	"log"
	"net/http"
)

// ErrorResponse defines the structure for JSON error responses.
type ErrorResponse struct {
	Error string `json:"error"`
}

// JSON sends a standard JSON response.
func JSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			// Log the error, but don't try to write another header as it's already sent.
			log.Printf("Error encoding JSON response: %v", err)
			// Optionally write a plain text error to the body if possible
			// http.Error(w, "Internal Server Error", http.StatusInternalServerError) // Avoid this after WriteHeader
		}
	}
}

// Error sends a JSON error response.
func Error(w http.ResponseWriter, statusCode int, message string) {
	log.Printf("API Error: Status %d, Message: %s", statusCode, message) // Log the error being sent
	JSON(w, statusCode, ErrorResponse{Error: message})
}
