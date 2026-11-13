package main

import (
    "encoding/json"
    "fmt"
    "net/http"
    "strings"
)

type Card struct {
    ID          string   `json:"id"`
    Title       string   `json:"title"`
    Content     string   `json:"content"`
    Date        string   `json:"date"`
    Tags        []string `json:"tags"`
}

var cards = []Card{
    {
        ID: "welcome",
        Title: "Welcome",
        Content: "This is the beginning of a new journey",
        Date: "2024-01-01",
        Tags: []string{"intro", "welcome", "start"},
    },
    {
        ID: "project-kickoff",
        Title: "Project Kickoff",
        Content: "Planning and setup phase",
        Date: "2024-01-02",
        Tags: []string{"planning", "kickoff", "project"},
    },
}

func cardsHandler(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodGet:
        handleGetCards(w, r)
    case http.MethodPost:
        handlePostCard(w, r)
    default:
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    }
}

func handleGetCards(w http.ResponseWriter, r *http.Request) {
    // Query parameters for filtering
    query := strings.TrimSpace(r.URL.Query().Get("q"))
    var filtered []Card
    
    for _, card := range cards {
        if query == "" || strings.Contains(strings.ToLower(card.Title), strings.ToLower(query)) ||
           strings.Contains(strings.ToLower(card.Content), strings.ToLower(query)) {
            filtered = append(filtered, card)
        }
    }
    
    w.Header().Set("Content-Type", "application/json")
    response := map[string]interface{}{
        "cards": filtered,
        "total": len(filtered),
        "query": query,
    }
    
    if err := json.NewEncoder(w).Encode(response); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
    }
}

func handlePostCard(w http.ResponseWriter, r *http.Request) {
    var newCard Card
    if err := json.NewDecoder(r.Body).Decode(&newCard); err != nil {
        http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
        return
    }
    
    // Validation
    if newCard.Title == "" {
        http.Error(w, "Title is required", http.StatusBadRequest)
        return
    }
    
    if newCard.Content == "" {
        http.Error(w, "Content is required", http.StatusBadRequest)
        return
    }
    
    // Add to collection
    newCard.ID = fmt.Sprintf("card-%d", len(cards)+1)
    newCard.Date = "2024-01-01" // Simplified - in real app use time.Now()
    cards = append(cards, newCard)
    
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    if err := json.NewEncoder(w).Encode(newCard); err != nil {
        fmt.Printf("Error encoding response: %v\n", err)
    }
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
    health := map[string]interface{}{
        "status": "healthy",
        "service": "vici",
        "version": "2.0.0",
        "card_count": len(cards),
        "last_updated": "2024-01-01T00:00:00Z",
        "features": []string{"cards", "search", "tags", "api"},
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(health)
}

func main() {
    // Setup routes
    http.HandleFunc("/api/cards", cardsHandler)
    http.HandleFunc("/health", healthHandler)
    
    // Start server
    fmt.Printf("VICI timeline service starting on port 8083\n")
    if err := http.ListenAndServe(":8083", nil); err != nil {
        fmt.Printf("Error starting server: %v\n", err)
    }
}