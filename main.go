package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"html/template"
	"math/big"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/redis/go-redis/v9"
	"github.com/yeqown/go-qrcode/v2"
	"github.com/yeqown/go-qrcode/writer/standard"
)

var (
	ctx = context.Background()
	rdb *redis.Client
)

type URLRequest struct {
	URL string `json:"url"`
}

type URLResponse struct {
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
	ShortCode   string `json:"short_code"`
}
type QRResponse struct {
	OriginalURL string `json:"original_url`
	QRFile      string `json:"qr_file"`
}

func main() {
	// Initialize Redis
	rdb = redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	r := chi.NewRouter()
	r.Use(middleware.Logger)

	// Serve static files
	fileServer := http.FileServer(http.Dir("static"))
	r.Handle("/static/*", http.StripPrefix("/static/", fileServer))

	// API Routes
	r.Post("/api/shorten", shortenHandler)
	r.Post("/api/qr", qrCodeHandler)
	r.Get("/api/stats/{code}", statsHandler)

	// Redirect route
	r.Get("/{code}", redirectHandler)

	r.Get("/", homeHandler)

	println("Server running on http://localhost:8080")
	http.ListenAndServe(":8080", r)
}

func shortenHandler(w http.ResponseWriter, r *http.Request) {
	var req URLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate URL
	if !strings.HasPrefix(req.URL, "http://") && !strings.HasPrefix(req.URL, "https://") {
		req.URL = "https://" + req.URL
	}

	shortCode := generateShortCode(6)

	err := rdb.Set(ctx, shortCode, req.URL, 0).Err()
	if err != nil {
		http.Error(w, "Failed to store URL", http.StatusInternalServerError)
		return
	}

	// store short URL clicks count
	rdb.Set(ctx, "clicks:"+shortCode, 0, 0)

	// Get base URL from environment or use request host
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://" + r.Host
	}

	response := URLResponse{
		ShortURL:    baseURL + "/" + shortCode,
		OriginalURL: req.URL,
		ShortCode:   shortCode,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func redirectHandler(w http.ResponseWriter, r *http.Request) {
	shortCode := chi.URLParam(r, "code")

	originalURL, err := rdb.Get(ctx, shortCode).Result()
	if err == redis.Nil {
		http.Error(w, "Short URL not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Increment click counter
	rdb.Incr(ctx, "clicks:"+shortCode)

	// Redirect to original URL
	http.Redirect(w, r, originalURL, http.StatusMovedPermanently)
}

func statsHandler(w http.ResponseWriter, r *http.Request) {
	shortCode := chi.URLParam(r, "code")

	// Get original URL
	originalURL, err := rdb.Get(ctx, shortCode).Result()
	if err == redis.Nil {
		http.Error(w, "Short URL not found", http.StatusNotFound)
		return
	}

	// Get click count
	clicks, _ := rdb.Get(ctx, "clicks:"+shortCode).Int()

	stats := map[string]interface{}{
		"short_code":   shortCode,
		"original_url": originalURL,
		"clicks":       clicks,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	t, err := template.ParseFiles("templates/index.html")
	if err != nil {
		http.Error(w, "Error While Loading Home Page", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	t.Execute(w, nil)
}
func qrCodeHandler(w http.ResponseWriter, r *http.Request) {
	var req URLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	qr, err := generateQRCode(req.URL)
	if err != nil {
		http.Error(w, "Cannot Generate QR Code", http.StatusInternalServerError)
		return
	}
	response := QRResponse{
		OriginalURL: req.URL,
		QRFile:      qr,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)

}

func generateShortCode(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	result := make([]byte, length)
	for i := range result {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		result[i] = charset[num.Int64()]
	}
	return string(result)
}

func generateQRCode(url string) (string, error) {
	qrc, err := qrcode.New(url)
	if err != nil {
		fmt.Printf("could not generate QRCode: %v", err)
		return "", err
	}
	qrFilePath := fmt.Sprintf("qrcodes/%s", generateShortCode(8))

	w, err := standard.New("./static/" + qrFilePath)
	if err != nil {
		fmt.Printf("standard.New failed: %v", err)
		return "", err
	}

	// save file
	if err = qrc.Save(w); err != nil {
		fmt.Printf("could not save image: %v", err)
		return "", err

	}
	return qrFilePath, nil
}
