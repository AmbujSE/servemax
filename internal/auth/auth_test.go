package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestMakeAndValidateJWT(t *testing.T) {
	// Test data
	userID := uuid.New()
	tokenSecret := "supersecretkey"
	expiresIn := time.Minute

	// Create a JWT
	token, err := MakeJWT(userID, tokenSecret, expiresIn)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	// Validate the JWT
	parsedUserID, err := ValidateJWT(token, tokenSecret)
	assert.NoError(t, err)
	assert.Equal(t, userID, parsedUserID)
}

func TestExpiredJWT(t *testing.T) {
	// Test data
	userID := uuid.New()
	tokenSecret := "supersecretkey"
	expiresIn := -time.Minute // Token already expired

	// Create an expired JWT
	token, err := MakeJWT(userID, tokenSecret, expiresIn)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	// Validate the expired JWT
	_, err = ValidateJWT(token, tokenSecret)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "token is expired")
}

func TestInvalidJWTSignature(t *testing.T) {
	// Test data
	userID := uuid.New()
	tokenSecret := "supersecretkey"
	wrongSecret := "wrongsecretkey"
	expiresIn := time.Minute

	// Create a JWT
	token, err := MakeJWT(userID, tokenSecret, expiresIn)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	// Validate the JWT with the wrong secret
	_, err = ValidateJWT(token, wrongSecret)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "signature is invalid")
}
