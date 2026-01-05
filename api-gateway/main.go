package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Data models
type NewsShortDetailed struct {
	ID      int       `json:"id"`
	Title   string    `json:"title"`
	Content string    `json:"content"`
	PubTime time.Time `json:"pub_time"`
}

type NewsFullDetailed struct {
	NewsShortDetailed
	Comments []Comment `json:"comments"`
}

type Comment struct {
	ID       int    `json:"id"`
	NewsID   int    `json:"news_id"`
	ParentID *int   `json:"parent_id,omitempty"`
	Text     string `json:"text"`
}

type Pagination struct {
	Page       int `json:"page"`
	TotalPages int `json:"total_pages"`
}

type NewsResponse struct {
	News       []NewsShortDetailed `json:"news"`
	Pagination Pagination          `json:"pagination"`
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
	mux.HandleFunc("/news", func(w http.ResponseWriter, r *http.Request) {
		loggingMiddleware(http.HandlerFunc(newsHandler)).ServeHTTP(w, r)
	})
	mux.HandleFunc("/news/", func(w http.ResponseWriter, r *http.Request) {
		loggingMiddleware(http.HandlerFunc(newsDetailHandler)).ServeHTTP(w, r)
	})
	mux.HandleFunc("/comment", func(w http.ResponseWriter, r *http.Request) {
		loggingMiddleware(http.HandlerFunc(commentHandler)).ServeHTTP(w, r)
	})

	// Create server
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("[*] HTTP server is started on localhost:8080")
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

func newsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pageStr := getRequestParam(r, "page")
	page := 1
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	searchQuery := getRequestParam(r, "s")
	if searchQuery == "" {
		searchQuery = getRequestParam(r, "search") // support both parameter names
	}

	// In a real implementation, this would call the news aggregator service
	// For now, we'll return mock data
	news := []NewsShortDetailed{
		{
			ID:      1,
			Title:   "First News",
			Content: "This is the content of the first news article",
			PubTime: time.Now().Add(-24 * time.Hour),
		},
		{
			ID:      2,
			Title:   "Second News",
			Content: "This is the content of the second news article",
			PubTime: time.Now().Add(-12 * time.Hour),
		},
	}

	// Apply search filter if query is provided
	if searchQuery != "" {
		var filtered []NewsShortDetailed
		for _, n := range news {
			if strings.Contains(strings.ToLower(n.Title), strings.ToLower(searchQuery)) {
				filtered = append(filtered, n)
			}
		}
		news = filtered
	}

	totalPages := 1
	if len(news) > 10 { // assuming 10 items per page
		totalPages = len(news)/10 + 1
		if len(news)%10 == 0 {
			totalPages--
		}
	}

	response := NewsResponse{
		News: news,
		Pagination: Pagination{
			Page:       page,
			TotalPages: totalPages,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func newsDetailHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract news ID from URL path
	path := strings.TrimPrefix(r.URL.Path, "/news/")
	if path == "" {
		http.Error(w, "News ID is required", http.StatusBadRequest)
		return
	}

	newsID, err := strconv.Atoi(path)
	if err != nil {
		http.Error(w, "Invalid news ID", http.StatusBadRequest)
		return
	}

	// Get news details (mock implementation)
	news := NewsShortDetailed{
		ID:      newsID,
		Title:   fmt.Sprintf("News %d", newsID),
		Content: fmt.Sprintf("Content of news %d", newsID),
		PubTime: time.Now(),
	}

	// Get comments for this news from CommentService
	comments, err := getCommentsForNews(newsID, getRequestID(r))
	if err != nil {
		log.Printf("Error fetching comments for news %d: %v", newsID, err)
		// Continue with empty comments array
		comments = []Comment{}
	}

	detailedNews := NewsFullDetailed{
		NewsShortDetailed: news,
		Comments:          comments,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(detailedNews)
}

func getCommentsForNews(newsID int, requestID string) ([]Comment, error) {
	// Create HTTP request to CommentService to get comments for news
	client := &http.Client{Timeout: 10 * time.Second}
	
	// Build the URL to get comments for specific news
	url := fmt.Sprintf("http://localhost:8081/comments?news_id=%d", newsID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	
	// Add request ID header
	req.Header.Set("X-Request-ID", requestID)
	
	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("comment service returned status: %d", resp.StatusCode)
	}
	
	// Decode the response
	var comments []Comment
	if err := json.NewDecoder(resp.Body).Decode(&comments); err != nil {
		return nil, err
	}
	
	return comments, nil
}

func commentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var comment Comment
	if err := json.NewDecoder(r.Body).Decode(&comment); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Send comment text to CensorService for validation
	requestID := getRequestID(r)
	if err := validateCommentWithCensorService(comment.Text, requestID); err != nil {
		// If censorship fails, return error to client
		http.Error(w, "Comment contains prohibited content", http.StatusBadRequest)
		return
	}

	// If censorship passes, save comment to CommentService
	if err := saveCommentToService(comment, requestID); err != nil {
		http.Error(w, "Failed to save comment", http.StatusInternalServerError)
		return
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Comment created successfully",
		"id":      1, // In a real implementation, this would be the actual ID
	})
}

func validateCommentWithCensorService(text, requestID string) error {
	// Create the request payload
	payload := map[string]string{
		"text": text,
	}
	
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// Create HTTP request to CensorService
	req, err := http.NewRequest("POST", "http://localhost:8082/check", strings.NewReader(string(jsonData)))
	if err != nil {
		return err
	}
	
	// Add request ID header
	req.Header.Set("X-Request-ID", requestID)
	req.Header.Set("Content-Type", "application/json")

	// Make the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode == http.StatusBadRequest {
		return fmt.Errorf("censor service rejected the comment")
	} else if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("censor service returned unexpected status: %d", resp.StatusCode)
	}

	return nil
}

func saveCommentToService(comment Comment, requestID string) error {
	// Create the request payload
	payload := map[string]interface{}{
		"news_id":   comment.NewsID,
		"parent_id": comment.ParentID,
		"text":      comment.Text,
	}
	
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// Create HTTP request to CommentService
	req, err := http.NewRequest("POST", "http://localhost:8081/comments", strings.NewReader(string(jsonData)))
	if err != nil {
		return err
	}
	
	// Add request ID header
	req.Header.Set("X-Request-ID", requestID)
	req.Header.Set("Content-Type", "application/json")

	// Make the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("comment service returned status: %d", resp.StatusCode)
	}

	return nil
}