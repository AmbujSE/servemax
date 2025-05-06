package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// HashPassword hashes a password using bcrypt.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPasswordHash compares a hashed password with a plain password.
func CheckPasswordHash(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// MakeJWT creates and returns a signed JWT for the given user ID.
func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	// Define the claims
	claims := jwt.RegisteredClaims{
		Issuer:    "chirpy",
		IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
		ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(expiresIn)),
		Subject:   userID.String(),
	}

	// Create the token with claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign the token with the secret key
	signedToken, err := token.SignedString([]byte(tokenSecret))
	if err != nil {
		return "", err
	}

	return signedToken, nil
}

// ValidateJWT validates a JWT and extracts the user ID from its claims.
func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	// Parse the token and validate its signature
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Ensure the signing method is HMAC
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(tokenSecret), nil
	})
	if err != nil {
		return uuid.Nil, err
	}

	// Extract the claims
	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok || !token.Valid {
		return uuid.Nil, errors.New("invalid token")
	}

	// Parse the user ID from the Subject field
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, errors.New("invalid user ID in token")
	}

	return userID, nil
}
