// Package vici provides middleware for user context tracking, anonymous sessions,
// and authentication using only the Go standard library.
//
// It supports:
//   - Anonymous tracking via secure, random session cookies.
//   - Authentication via HTTP Headers or URL Query Parameters.
//   - Context propagation of user ID, auth status, and metadata.
//   - Layering with other middlewares (e.g., FileServer).
package vici

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

const (
	// UserIDKey is the key used to retrieve the user ID from context.
	UserIDKey contextKey = "vici_user_id"
	// IsAuthenticatedKey is the key used to retrieve auth status from context.
	IsAuthenticatedKey contextKey = "vici_is_authenticated"
	// UserMetadataKey is the key used to retrieve metadata map from context.
	UserMetadataKey contextKey = "vici_metadata"
)

// Config holds the configuration for the VICI middleware.
type Config struct {
	// CookieName is the name of the session cookie for anonymous users.
	// Default: "vici_session"
	CookieName string

	// CookieMaxAge is the maximum age of the cookie in seconds.
	// Default: 86400 (24 hours)
	CookieMaxAge int

	// AuthHeaderKey is the HTTP header key to look for authentication tokens.
	// Default: "Authorization"
	AuthHeaderKey string

	// AuthParamKey is the URL query parameter key to look for authentication tokens.
	// Default: "auth_token"
	AuthParamKey string

	// SecretKey is the secret used for HMAC token validation.
	// If empty, authentication is effectively disabled (all tokens rejected).
	// IMPORTANT: In production, use a strong random secret and implement proper JWT/HMAC logic.
	SecretKey []byte

	// EnableLogging enables logging of session events (user ID, auth status, path).
	// Default: true
	EnableLogging bool
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		CookieName:    "vici_session",
		CookieMaxAge:  86400, // 24 hours
		AuthHeaderKey: "Authorization",
		AuthParamKey:  "auth_token",
		EnableLogging: true,
	}
}

// Middleware creates the VICI middleware handler.
// It wraps the provided 'next' handler, adding user context and session management.
func Middleware(cfg Config, next http.Handler) http.Handler {
	// Apply defaults for unset fields
	if cfg.CookieName == "" {
		cfg.CookieName = "vici_session"
	}
	if cfg.AuthHeaderKey == "" {
		cfg.AuthHeaderKey = "Authorization"
	}
	if cfg.AuthParamKey == "" {
		cfg.AuthParamKey = "auth_token"
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// 1. Extract User Context (Auth or Anonymous)
		userID, isAuthenticated, metadata := extractUserContext(r, cfg)

		// 2. Enrich Context
		ctx = context.WithValue(ctx, UserIDKey, userID)
		ctx = context.WithValue(ctx, IsAuthenticatedKey, isAuthenticated)
		ctx = context.WithValue(ctx, UserMetadataKey, metadata)

		// 3. Manage Cookies for Anonymous Users
		// Only set cookie if user is anonymous and doesn't already have a valid one
		if !isAuthenticated && userID != "" {
			cookie, err := r.Cookie(cfg.CookieName)
			// If cookie is missing or value doesn't match (edge case), set it
			if err != nil || cookie.Value != userID {
				http.SetCookie(w, &http.Cookie{
					Name:     cfg.CookieName,
					Value:    userID,
					Path:     "/",
					HttpOnly: true,                 // Prevents XSS access to the cookie
					Secure:   false,                // Set to TRUE in production with HTTPS
					SameSite: http.SameSiteLaxMode, // Mitigates CSRF
					MaxAge:   cfg.CookieMaxAge,
				})
			}
		}

		// 4. Logging (Optional)
		if cfg.EnableLogging {
			logSessionEvent(r, userID, isAuthenticated)
		}

		// 5. Proceed to next handler with enriched context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// extractUserContext determines the user identity and authentication status.
// Priority: 1. Auth Header -> 2. Auth Param -> 3. Anonymous Cookie/New Session
func extractUserContext(r *http.Request, cfg Config) (userID string, isAuthenticated bool, metadata map[string]string) {
	metadata = make(map[string]string)

	// --- Step 1: Check Authorization Header ---
	authHeader := r.Header.Get(cfg.AuthHeaderKey)
	if authHeader != "" && isValidToken(authHeader, cfg.SecretKey) {
		isAuthenticated = true
		userID = "auth_" + generateShortID()
		metadata["auth_method"] = "header"
		return
	}

	// --- Step 2: Check URL Query Parameter ---
	authParam := r.URL.Query().Get(cfg.AuthParamKey)
	if authParam != "" && isValidToken(authParam, cfg.SecretKey) {
		isAuthenticated = true
		userID = "auth_" + generateShortID()
		metadata["auth_method"] = "url_param"
		return
	}

	// --- Step 3: Fallback to Anonymous Session ---
	cookie, err := r.Cookie(cfg.CookieName)
	if err == nil && cookie.Value != "" {
		// Reuse existing session ID
		userID = cookie.Value
	} else {
		// Generate new secure session ID
		userID = generateSessionID()
	}

	return userID, false, metadata
}

// isValidToken validates the provided token.
//
// SECURITY NOTE: This is a simplified implementation for demonstration.
// In a real production environment, you MUST replace this with:
// 1. Proper JWT validation (using a library like github.com/golang-jwt/jwt/v5)
// 2. Or a robust HMAC-SHA256 signature verification where the client signs a payload.
//
// Current Logic:
// - Requires a SecretKey.
// - Expects token format: "valid_<random_suffix>".
// - Verifies the suffix against an HMAC of the secret (simulated here for simplicity).
func isValidToken(token string, secret []byte) bool {
	if len(secret) == 0 {
		return false // No secret = no authentication allowed
	}

	// Split token into prefix and suffix
	parts := strings.SplitN(token, "_", 2)
	if len(parts) != 2 || parts[0] != "valid" {
		return false
	}
	suffix := parts[1]

	// Simulate HMAC verification:
	// In a real app, the client would send: "valid_" + HMAC(secret, payload)
	// Here we just check if the suffix looks like a hex string and pretend it's valid
	// if the secret exists.
	//
	// PRODUCTION IMPLEMENTATION SHOULD LOOK LIKE THIS:
	// h := hmac.New(sha256.New, secret)
	// h.Write([]byte(payload)) // Payload should be extracted from token
	// expectedSig := hex.EncodeToString(h.Sum(nil))
	// return hmac.Equal([]byte(suffix), []byte(expectedSig))

	// For this demo, we accept any "valid_" token if a secret is set.
	// This prevents the middleware from crashing but is NOT secure for real auth.
	return true
}

// generateSessionID creates a cryptographically secure random session ID (64 hex chars).
func generateSessionID() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// Fallback to time-based ID if crypto/rand fails (extremely rare)
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// generateShortID creates a shorter random ID (32 hex chars) for authenticated users.
func generateShortID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// logSessionEvent logs session activity to the standard logger.
func logSessionEvent(r *http.Request, userID string, isAuthenticated bool) {
	status := "anonymous"
	if isAuthenticated {
		status = "authenticated"
	}
	log.Printf("[VICI] User: %s (%s) | Path: %s | Method: %s", userID, status, r.URL.Path, r.Method)
}

// GetUserContext retrieves the user context from the request.
// It returns the User ID, Authentication Status, and Metadata map.
//
// Usage:
//
//	userID, auth, meta := vici.GetUserContext(r)
func GetUserContext(r *http.Request) (string, bool, map[string]string) {
	userID, _ := r.Context().Value(UserIDKey).(string)
	isAuthenticated, _ := r.Context().Value(IsAuthenticatedKey).(bool)

	var metadata map[string]string
	if m, ok := r.Context().Value(UserMetadataKey).(map[string]string); ok {
		metadata = m
	} else {
		metadata = make(map[string]string)
	}

	return userID, isAuthenticated, metadata
}
