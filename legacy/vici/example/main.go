package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/Emperor42/vici"
)

func main() {
	// Create a simple static directory if it doesn't exist
	os.MkdirAll("static", 0755)

	// Create a dummy index.html
	indexHTML := `
	<!DOCTYPE html>
	<html>
	<head><title>VICI Demo</title></head>
	<body>
		<h1>VICI Middleware Demo</h1>
		<p>Check the browser console or network tab to see the session cookie.</p>
		<form action="/submit" method="POST">
			<input type="text" name="username" placeholder="Username">
			<button type="submit">Submit</button>
		</form>
		<p>Try accessing with ?auth_token=valid_random_string to authenticate.</p>
	</body>
	</html>
	`
	os.WriteFile("static/index.html", []byte(indexHTML), 0644)

	// Configure VICI
	cfg := vici.DefaultConfig()
	cfg.SecretKey = []byte("change-this-in-production")
	cfg.EnableLogging = true

	// Create a custom handler to demonstrate context usage
	demoHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, isAuthenticated, metadata := vici.GetUserContext(r)

		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, "<h1>Hello!</h1>")
		fmt.Fprintf(w, "<p>User ID: <strong>%s</strong></p>", userID)
		fmt.Fprintf(w, "<p>Authenticated: <strong>%v</strong></p>", isAuthenticated)
		fmt.Fprintf(w, "<p>Metadata: <code>%v</code></p>", metadata)
	})

	// Layer the middleware
	// 1. VICI wraps the handler
	handler := vici.Middleware(cfg, demoHandler)

	log.Println("Starting VICI Demo Server on :8080")
	log.Println("Visit http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", handler))
}
