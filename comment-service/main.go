package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/google/uuid"
)

// Comment model
type Comment struct {
	ID       int    `json:"id"`
	NewsID   int    `json:"news_id"`
	ParentID *int   `json:"parent_id,omitempty"`
	Text     string `json:"text"`
}

type CreateCommentRequest struct {
	NewsID   int    `json:"news_id"`
	ParentID *int   `json:"parent_id,omitempty"`
	Text     string `json:"text"`
}

// In-memory storage for comments (for demonstration)
var comments []Comment
var nextID = 1

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

func getRequestParam(r *http.Request, key string) string {
	return r.URL.Query().Get(key)
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
	mux.HandleFunc("/comments", func(w http.ResponseWriter, r *http.Request) {
		loggingMiddleware(http.HandlerFunc(commentsHandler)).ServeHTTP(w, r)
	})

	// Create server
	server := &http.Server{
		Addr:    ":8081",
		Handler: mux,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("[*] HTTP server is started on localhost:8081")
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

func commentsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getCommentsHandler(w, r)
	case http.MethodPost:
		createCommentHandler(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func createCommentHandler(w http.ResponseWriter, r *http.Request) {
	var req CreateCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Add comment to in-memory storage
	comment := Comment{
		ID:       nextID,
		NewsID:   req.NewsID,
		ParentID: req.ParentID,
		Text:     req.Text,
	}
	comments = append(comments, comment)
	nextID++

	// Return the created comment ID
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id": comment.ID,
	})
}

func getCommentsHandler(w http.ResponseWriter, r *http.Request) {
	newsIDStr := getRequestParam(r, "news_id")
	if newsIDStr == "" {
		http.Error(w, "news_id parameter is required", http.StatusBadRequest)
		return
	}

	newsID, err := strconv.Atoi(newsIDStr)
	if err != nil {
		http.Error(w, "Invalid news_id parameter", http.StatusBadRequest)
		return
	}

	// Filter comments for the given news ID
	var filteredComments []Comment
	for _, comment := range comments {
		if comment.NewsID == newsID {
			filteredComments = append(filteredComments, comment)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filteredComments)
}