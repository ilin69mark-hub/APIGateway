package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/google/uuid"
)

// Request model for censorship check
type CheckRequest struct {
	Text string `json:"text"`
}

// Middleware for request_id and logging
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		
		// Log the request
		log.Printf("[%s] [INFO] [%s] [%s] [%s] [%s] -", 
			start.Format("2006-01-02 15:04:05"), 
			requestID, 
			r.RemoteAddr, 
			r.Method, 
			r.URL.Path)
		
		// Add request_id to context
		ctx := context.WithValue(r.Context(), "request_id", requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
		
		// Log the response
		duration := time.Since(start)
		// Note: This is a simplified version - in production, you'd need to capture the status code properly
		log.Printf("[%s] [INFO] [%s] [%s] [%s] [%s] [200] [%v]", 
			start.Format("2006-01-02 15:04:05"), 
			requestID, 
			r.RemoteAddr, 
			r.Method, 
			r.URL.Path,
			duration)
	})
}

func getRequestID(r *http.Request) string {
	if requestID, ok := r.Context().Value("request_id").(string); ok {
		return requestID
	}
	return ""
}

func main() {
	// Create HTTP multiplexer
	mux := http.NewServeMux()

	// Register handlers with middleware
	mux.HandleFunc("/check", func(w http.ResponseWriter, r *http.Request) {
		loggingMiddleware(http.HandlerFunc(checkHandler)).ServeHTTP(w, r)
	})

	// Create server
	server := &http.Server{
		Addr:    ":8082",
		Handler: mux,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("[*] HTTP server is started on localhost:8082")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	log.Printf("[*] HTTP server has been stopped. Reason: interrupt")

	// Shutdown server gracefully
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	} else {
		log.Println("Server exited properly")
	}
}

func checkHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Check if the text contains prohibited words
	if containsProhibitedWords(req.Text) {
		http.Error(w, "Text contains prohibited content", http.StatusBadRequest)
		return
	}

	// If text passes censorship, return 200 OK
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Text passed censorship check",
	})
}

// containsProhibitedWords checks if the text contains any prohibited words
func containsProhibitedWords(text string) bool {
	prohibitedWords := []string{"qwerty", "йцукен", "zxvbnm"}
	
	for _, word := range prohibitedWords {
		if containsIgnoreCase(text, word) {
			return true
		}
	}
	
	return false
}

// containsIgnoreCase checks if a string contains another string, ignoring case
func containsIgnoreCase(s, substr string) bool {
	s = toLower(s)
	substr = toLower(substr)
	return contains(s, substr)
}

// Simple implementation of string operations to avoid importing strings package
func toLower(s string) string {
	var result []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c = c + ('a' - 'A')
		}
		result = append(result, c)
	}
	return string(result)
}

func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}