package main

import (
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
)

func TestCardsHandlerGet(t *testing.T) {
    req, err := http.NewRequest("GET", "/api/cards", nil)
    if err != nil {
        t.Fatal(err)
    }
    
    rr := httptest.NewRecorder()
    handler := http.HandlerFunc(cardsHandler)
    
    handler.ServeHTTP(rr, req)
    
    if status := rr.Code; status != http.StatusOK {
        t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
    }
    
    var response map[string]interface{}
    if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
        t.Errorf("Error decoding response: %v", err)
    }
    
    cards, ok := response["cards"].([]interface{})
    if !ok {
        t.Errorf("Expected 'cards' field in response")
    }
    
    if len(cards) != 2 {
        t.Errorf("Expected 2 cards, got %d", len(cards))
    }
    
    total, ok := response["total"].(float64)
    if !ok {
        t.Errorf("Expected 'total' field in response")
    }
    
    if int(total) != 2 {
        t.Errorf("Expected total 2, got %v", total)
    }
}

func TestCardsHandlerPost(t *testing.T) {
    newCard := Card{
        ID: "test-card",
        Title: "Test Card",
        Content: "This is a test card",
        Date: "2024-01-01",
        Tags: []string{"test", "example"},
    }
    
    cardJSON, err := json.Marshal(newCard)
    if err != nil {
        t.Fatal(err)
    }
    
    req, err := http.NewRequest("POST", "/api/cards", strings.NewReader(string(cardJSON)))
    if err != nil {
        t.Fatal(err)
    }
    req.Header.Set("Content-Type", "application/json")
    
    rr := httptest.NewRecorder()
    handler := http.HandlerFunc(cardsHandler)
    
    handler.ServeHTTP(rr, req)
    
    if status := rr.Code; status != http.StatusCreated {
        t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusCreated)
    }
    
    var response Card
    if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
        t.Errorf("Error decoding response: %v", err)
    }
    
    if response.Title != "Test Card" {
        t.Errorf("Expected card title 'Test Card', got %v", response.Title)
    }
}

func TestHealthHandler(t *testing.T) {
    req, err := http.NewRequest("GET", "/health", nil)
    if err != nil {
        t.Fatal(err)
    }
    
    rr := httptest.NewRecorder()
    handler := http.HandlerFunc(healthHandler)
    
    handler.ServeHTTP(rr, req)
    
    if status := rr.Code; status != http.StatusOK {
        t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
    }
    
    var health map[string]interface{}
    if err := json.NewDecoder(rr.Body).Decode(&health); err != nil {
        t.Errorf("Error decoding response: %v", err)
    }
    
    if health["service"] != "vici" {
        t.Errorf("Expected service 'vici', got %v", health["service"])
    }
    
    if health["status"] != "healthy" {
        t.Errorf("Expected status 'healthy', got %v", health["status"])
    }
    
    if health["card_count"] != 2 {
        t.Errorf("Expected card count 2, got %v", health["card_count"])
    }
}

func TestCardsHandlerInvalidMethod(t *testing.T) {
    req, err := http.NewRequest("DELETE", "/api/cards", nil)
    if err != nil {
        t.Fatal(err)
    }
    
    rr := httptest.NewRecorder()
    handler := http.HandlerFunc(cardsHandler)
    
    handler.ServeHTTP(rr, req)
    
    if status := rr.Code; status != http.StatusMethodNotAllowed {
        t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusMethodNotAllowed)
    }
}