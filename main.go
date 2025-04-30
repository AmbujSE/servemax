package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"go-server/internal/auth"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

// Struct to hold stateful data
type apiConfig struct {
	fileserverHits atomic.Int32
	db             *sql.DB
	platform       string
}

// Middleware to increment file server hits
func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

// Handler to return admin metrics
func (cfg *apiConfig) adminMetricsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	metrics := map[string]interface{}{
		"file_server_hits": cfg.fileserverHits.Load(),
	}

	respondWithJSON(w, http.StatusOK, metrics)
}

// Helper function to respond with an error
func respondWithError(w http.ResponseWriter, code int, message string) {
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// Helper function to respond with JSON
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(payload)
}

// Struct for User
type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}

// Handler to create a user
func (cfg *apiConfig) createUserHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Parse the JSON body
	var requestBody struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	err := json.NewDecoder(r.Body).Decode(&requestBody)
	if err != nil || requestBody.Email == "" || requestBody.Password == "" {
		respondWithError(w, http.StatusBadRequest, "Invalid JSON or missing fields")
		return
	}

	// Hash the password
	hashedPassword, err := auth.HashPassword(requestBody.Password)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to hash password")
		return
	}

	// Insert the user into the database
	var user User
	query := `
        INSERT INTO users (id, created_at, updated_at, email, hashed_password)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING id, created_at, updated_at, email`
	err = cfg.db.QueryRowContext(r.Context(), query,
		uuid.New(), time.Now(), time.Now(), requestBody.Email, hashedPassword,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt, &user.Email)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create user")
		return
	}

	// Respond with the created user (excluding the password)
	respondWithJSON(w, http.StatusCreated, user)
}

// Handler to reset all users
func (cfg *apiConfig) resetUsersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Check if the platform is "dev"
	if cfg.platform != "dev" {
		respondWithError(w, http.StatusForbidden, "Forbidden")
		return
	}

	// Delete all users from the database
	_, err := cfg.db.ExecContext(r.Context(), "DELETE FROM users")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to reset users")
		return
	}

	// Respond with success
	respondWithJSON(w, http.StatusOK, map[string]string{"status": "All users deleted"})
}

// Handler to create a chirp
func (cfg *apiConfig) createChirpHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		fmt.Println("Invalid method:", r.Method)
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	fmt.Println("POST /api/chirps handler invoked")
	// Parse the JSON body
	var requestBody struct {
		Body   string    `json:"body"`
		UserID uuid.UUID `json:"user_id"`
	}
	err := json.NewDecoder(r.Body).Decode(&requestBody)
	if err != nil || requestBody.Body == "" || requestBody.UserID == uuid.Nil {
		respondWithError(w, http.StatusBadRequest, "Invalid JSON or missing fields")
		return
	}

	// Validate the chirp length
	if len(requestBody.Body) > 140 {
		respondWithError(w, http.StatusBadRequest, "Chirp exceeds 140 characters")
		return
	}

	// Insert the chirp into the database
	var chirp struct {
		ID        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Body      string    `json:"body"`
		UserID    uuid.UUID `json:"user_id"`
	}
	query := `
    INSERT INTO chirps (id, created_at, updated_at, body, user_id)
    VALUES ($1, $2, $3, $4, $5)
    RETURNING id, created_at, updated_at, body, user_id`
	err = cfg.db.QueryRowContext(r.Context(), query,
		uuid.New(), time.Now(), time.Now(), requestBody.Body, requestBody.UserID,
	).Scan(&chirp.ID, &chirp.CreatedAt, &chirp.UpdatedAt, &chirp.Body, &chirp.UserID)
	if err != nil {
		// Log the error for debugging
		http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Respond with the created chirp
	respondWithJSON(w, http.StatusCreated, chirp)
}

// Handler to get all chirps
func (cfg *apiConfig) getAllChirpsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Retrieve all chirps from the database
	rows, err := cfg.db.QueryContext(r.Context(), `
        SELECT id, created_at, updated_at, body, user_id
        FROM chirps
        ORDER BY created_at ASC`)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve chirps")
		return
	}
	defer rows.Close()

	// Parse the rows into a slice of chirps
	var chirps []struct {
		ID        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Body      string    `json:"body"`
		UserID    uuid.UUID `json:"user_id"`
	}
	for rows.Next() {
		var chirp struct {
			ID        uuid.UUID `json:"id"`
			CreatedAt time.Time `json:"created_at"`
			UpdatedAt time.Time `json:"updated_at"`
			Body      string    `json:"body"`
			UserID    uuid.UUID `json:"user_id"`
		}
		err := rows.Scan(&chirp.ID, &chirp.CreatedAt, &chirp.UpdatedAt, &chirp.Body, &chirp.UserID)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to parse chirps")
			return
		}
		chirps = append(chirps, chirp)
	}

	// Respond with the chirps
	respondWithJSON(w, http.StatusOK, chirps)
}

// Handler to get a single chirp by ID
func (cfg *apiConfig) getChirpByIDHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Extract the chirpID from the URL path
	path := r.URL.Path
	// Expected path format: /api/chirps/{chirpID}
	parts := strings.Split(path, "/")
	if len(parts) != 4 || parts[3] == "" {
		respondWithError(w, http.StatusBadRequest, "Invalid chirp ID")
		return
	}
	chirpID := parts[3]

	// Query the database for the chirp
	var chirp struct {
		ID        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Body      string    `json:"body"`
		UserID    uuid.UUID `json:"user_id"`
	}
	query := `
        SELECT id, created_at, updated_at, body, user_id
        FROM chirps
        WHERE id = $1`
	err := cfg.db.QueryRowContext(r.Context(), query, chirpID).Scan(
		&chirp.ID, &chirp.CreatedAt, &chirp.UpdatedAt, &chirp.Body, &chirp.UserID,
	)
	if err == sql.ErrNoRows {
		respondWithError(w, http.StatusNotFound, "Chirp not found")
		return
	} else if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve chirp")
		return
	}

	// Respond with the chirp
	respondWithJSON(w, http.StatusOK, chirp)
}

// Combined handler for /api/chirps
func (cfg *apiConfig) chirpsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		cfg.createChirpHandler(w, r) // Handle POST requests
	case http.MethodGet:
		cfg.getAllChirpsHandler(w, r) // Handle GET requests
	default:
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (cfg *apiConfig) loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Parse the JSON body
	var requestBody struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	err := json.NewDecoder(r.Body).Decode(&requestBody)
	if err != nil || requestBody.Email == "" || requestBody.Password == "" {
		respondWithError(w, http.StatusBadRequest, "Invalid JSON or missing fields")
		return
	}

	// Look up the user by email
	var user struct {
		ID             uuid.UUID `json:"id"`
		CreatedAt      time.Time `json:"created_at"`
		UpdatedAt      time.Time `json:"updated_at"`
		Email          string    `json:"email"`
		HashedPassword string    `json:"-"`
	}
	query := `
        SELECT id, created_at, updated_at, email, hashed_password
        FROM users
        WHERE email = $1`
	err = cfg.db.QueryRowContext(r.Context(), query, requestBody.Email).Scan(
		&user.ID, &user.CreatedAt, &user.UpdatedAt, &user.Email, &user.HashedPassword,
	)
	if err == sql.ErrNoRows {
		respondWithError(w, http.StatusUnauthorized, "Incorrect email or password")
		return
	} else if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve user")
		return
	}

	// Check the password
	err = auth.CheckPasswordHash(user.HashedPassword, requestBody.Password)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Incorrect email or password")
		return
	}

	// Respond with the user (excluding the password)
	respondWithJSON(w, http.StatusOK, struct {
		ID        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Email     string    `json:"email"`
	}{
		ID:        user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email:     user.Email,
	})
}

func main() {
	// Load environment variables from .env file
	godotenv.Load()

	// Read the PLATFORM environment variable
	platform := os.Getenv("PLATFORM")

	// Connect to the database
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		panic(err)
	}

	// Create a new ServeMux
	mux := http.NewServeMux()

	// Create an instance of apiConfig
	apiCfg := &apiConfig{
		db:       db,
		platform: platform,
	}

	// Add the readiness endpoint (GET only) under /api
	mux.HandleFunc("/api/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		respondWithJSON(w, http.StatusOK, map[string]string{"status": "OK"})
	})

	// Use http.FileServer to serve files from the /app/ path
	fileServer := http.FileServer(http.Dir("."))
	mux.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app", fileServer)))

	// Add the admin metrics endpoint (GET only) under /admin
	mux.HandleFunc("/admin/metrics", apiCfg.adminMetricsHandler)

	// Add the admin reset endpoint (POST only) under /admin
	mux.HandleFunc("/admin/reset", apiCfg.resetUsersHandler)

	// Add the create user endpoint (POST only) under /api
	mux.HandleFunc("/api/users", apiCfg.createUserHandler)
	mux.HandleFunc("/api/chirps", apiCfg.chirpsHandler)
	mux.HandleFunc("/api/chirps/", apiCfg.getChirpByIDHandler)
	mux.HandleFunc("/api/login", apiCfg.loginHandler)

	// Create a new HTTP server
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	// Start the server
	err = server.ListenAndServe()
	if err != nil {
		panic(err)
	}
}
