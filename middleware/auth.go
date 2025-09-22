package middleware

import (
	"log"
	"net/http"
	"strings"

	"imageapi/config"

	"github.com/gorilla/sessions"
)

const (
	// SessionName is the key for the cookie session.
	SessionName = "imageapi-session"
	// UserSessionKey is the key used to store the authenticated status in the session.
	UserSessionKey = "authenticated"
)

// Store will hold the session cookie store.
var Store *sessions.CookieStore

// InitSessionStore initializes the session store.
// It should be called once during application startup.
func InitSessionStore() {
	// The session key should be a long, random string.
	// It's read from an environment variable for security.
	sessionKey := config.AppConfig.Settings.SessionSecret
	if sessionKey == "a_very_long_and_random_secret_string" {
		log.Println("Warning: SESSION_SECRET is not set or is the default. Using a default, insecure key. Please set a strong secret in your .env file for production.")
	}
	Store = sessions.NewCookieStore([]byte(sessionKey))

	Store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7, // 7 days
		HttpOnly: true,
		Secure:   false, // Set to true if using HTTPS
		SameSite: http.SameSiteLaxMode,
	}
}

// WebAuthMiddleware protects web routes that require authentication.
func WebAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webPassword := config.AppConfig.Settings.WebPassword
		// If no password is set, authentication is disabled.
		if webPassword == "" {
			next.ServeHTTP(w, r)
			return
		}

		session, err := Store.Get(r, SessionName)
		if err != nil {
			// This could happen if the cookie secret changes.
			// In this case, we treat them as unauthenticated.
			log.Printf("Session error: %v. Forcing login.", err)
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		// Check if the user is authenticated.
		if auth, ok := session.Values[UserSessionKey].(bool); !ok || !auth {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		// User is authenticated, proceed to the next handler.
		next.ServeHTTP(w, r)
	})
}

// APIKeyAuthMiddleware protects API routes with an API key.
func APIKeyAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := config.AppConfig.APIKeys.ImageAPI
		if apiKey == "" {
			log.Println("Error: IMAGEAPI_API_KEY is not set. API is disabled.")
			http.Error(w, "API is not configured on the server.", http.StatusServiceUnavailable)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header is required", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			http.Error(w, "Invalid Authorization header format. Expected 'Bearer <api_key>'", http.StatusUnauthorized)
			return
		}

		providedKey := parts[1]
		if providedKey != apiKey {
			http.Error(w, "Invalid API Key", http.StatusUnauthorized)
			return
		}

		// API key is valid, proceed to the next handler.
		next.ServeHTTP(w, r)
	})
}
