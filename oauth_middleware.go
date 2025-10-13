package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

// OAuthConfig holds OAuth configuration
type OAuthConfig struct {
	AuthzServerURL string
	JwksURL        string
	ResourceURL    string
	jwks           keyfunc.Keyfunc
}

// InitJWKS initializes the JWKS client
func (c *OAuthConfig) InitJWKS() error {
	jwks, err := keyfunc.NewDefault([]string{c.JwksURL})
	if err != nil {
		return fmt.Errorf("failed to create JWKS client: %w", err)
	}
	c.jwks = jwks
	log.Printf("Initialized JWKS from: %s", c.JwksURL)
	return nil
}

// OAuthMiddleware is a middleware that performs OAuth 2.1 authorization
func (c *OAuthConfig) OAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			c.sendUnauthorized(w, r)
			return
		}

		// Extract Bearer token
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			c.sendUnauthorized(w, r)
			return
		}

		// Validate JWT token using JWKS with algorithm validation
		token, err := jwt.Parse(tokenString, c.jwks.Keyfunc, jwt.WithValidMethods([]string{"RS256"}))
		if err != nil {
			log.Printf("Failed to parse token: %v", err)
			c.sendUnauthorized(w, r)
			return
		}

		if !token.Valid {
			log.Printf("Invalid token")
			c.sendUnauthorized(w, r)
			return
		}

		// Get claims for validation
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			log.Printf("Invalid claims type")
			c.sendUnauthorized(w, r)
			return
		}

		// Debug: Dump JWT access token before validation
		log.Printf("=== JWT Access Token Debug ===")
		log.Printf("Raw Token: %s", tokenString)
		claimsJSON, _ := json.MarshalIndent(claims, "", "  ")
		log.Printf("Claims: %s", string(claimsJSON))
		log.Printf("===============================")

		// Validate audience (MUST): Verify this resource server is in the audience
		if !c.validateAudience(claims) {
			log.Printf("Invalid audience")
			c.sendUnauthorized(w, r)
			return
		}

		// Validate issuer (MUST): Verify token is issued by expected authorization server
		if !c.validateIssuer(claims) {
			log.Printf("Invalid issuer")
			c.sendUnauthorized(w, r)
			return
		}

		// Validate expiration (MUST): Ensure token is not expired
		// Note: jwt.Parse already validates exp by default, but we explicitly check here for clarity
		if !c.validateExpiration(claims) {
			log.Printf("Token expired")
			c.sendUnauthorized(w, r)
			return
		}

		// Validate scope: Verify token has required scopes (optional, depends on your requirements)
		if !c.validateScope(claims) {
			log.Printf("Insufficient scope")
			c.sendUnauthorized(w, r)
			return
		}

		// Authorization successful - proceed to next handler
		next.ServeHTTP(w, r)
	})
}

// validateAudience validates that the token's audience matches this resource server
func (c *OAuthConfig) validateAudience(claims jwt.MapClaims) bool {
	aud, ok := claims["aud"]
	if !ok {
		return false
	}

	// aud can be a string or array of strings
	switch v := aud.(type) {
	case string:
		return v == c.ResourceURL
	case []interface{}:
		for _, a := range v {
			if audStr, ok := a.(string); ok && audStr == c.ResourceURL {
				return true
			}
		}
		return false
	default:
		return false
	}
}

// validateIssuer validates that the token's issuer matches the expected authorization server
func (c *OAuthConfig) validateIssuer(claims jwt.MapClaims) bool {
	iss, ok := claims["iss"].(string)
	if !ok {
		return false
	}
	return iss == c.AuthzServerURL
}

// validateExpiration validates that the token has not expired
func (c *OAuthConfig) validateExpiration(claims jwt.MapClaims) bool {
	exp, ok := claims["exp"].(float64)
	if !ok {
		return false
	}
	// Allow 60 seconds of clock skew
	return time.Now().Unix() < int64(exp)+60
}

// validateScope validates that the token has required scopes
func (c *OAuthConfig) validateScope(claims jwt.MapClaims) bool {
	scope, ok := claims["scope"].(string)
	if !ok {
		return false
	}
	// Scope is a space-separated string (OAuth 2.0 standard)
	// Check if "mcp:tools" is present
	for _, s := range strings.Split(scope, " ") {
		if s == "mcp:tools" {
			return true
		}
	}
	return false
}

// sendUnauthorized sends a 401 response with WWW-Authenticate header
func (c *OAuthConfig) sendUnauthorized(w http.ResponseWriter, r *http.Request) {
	metadataURL := c.ResourceURL + "/.well-known/oauth-protected-resource"
	w.Header().Set("WWW-Authenticate",
		fmt.Sprintf(`Bearer resource_metadata="%s"`, metadataURL))
	http.Error(w, "Unauthorized", http.StatusUnauthorized)
}

// HandleProtectedResourceMetadata handles the protected resource metadata endpoint
func (c *OAuthConfig) HandleProtectedResourceMetadata(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	metadata := oauthex.ProtectedResourceMetadata{
		Resource:             c.ResourceURL,
		ScopesSupported:      []string{"mcp:tools"},
		AuthorizationServers: []string{c.AuthzServerURL},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metadata)
}

// LoggingMiddleware logs HTTP requests including method, path, and POST body
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Log basic request info
		log.Printf("[%s] %s %s", r.Method, r.URL.Path, r.RemoteAddr)

		// Log POST body if present
		if r.Method == "POST" && r.Body != nil {
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				log.Printf("Error reading body: %v", err)
			} else {
				// Log the body
				log.Printf("Body: %s", string(bodyBytes))
				// Restore the body for the next handler
				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			}
		}

		next.ServeHTTP(w, r)

		log.Printf("Request completed in %v", time.Since(start))
	})
}
