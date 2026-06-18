package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

type Card struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Faction   string   `json:"faction"`
	Nation    string   `json:"nation"`
	Species   string   `json:"species"`
	Text      string   `json:"text"`
	Tags      []string `json:"tags"`
	CreatedAt string   `json:"created_at"`
	Links     []string `json:"links"`
}

var (
	cards    []Card
	cardsMu  sync.RWMutex
	dataFile = "cards.json"
	nameRe   = regexp.MustCompile(`\b([A-Z][a-zA-Z]+)\b`)
)

func main() {
	loadCards()
	http.HandleFunc("/", rootHandler)
	http.HandleFunc("/api/cards", cardsHandler)
	http.HandleFunc("/api/cards/", cardByIDHandler)
	http.HandleFunc("/api/export", exportHandler)
	http.HandleFunc("/api/import", importHandler)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.ListenAndServe(":8083", nil)
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	cardsMu.RLock()
	defer cardsMu.RUnlock()
	tmpl := template.Must(template.ParseFiles("templates/index.html"))
	tmpl.Execute(w, cards)
}

func cardsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		cardsMu.RLock()
		json.NewEncoder(w).Encode(cards)
		cardsMu.RUnlock()
	case http.MethodPost:
		var c Card
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &c)
		if c.Name == "" {
			http.Error(w, "name required", http.StatusBadRequest)
			return
		}
		c.ID = fmt.Sprintf("card_%d", time.Now().UnixNano())
		c.CreatedAt = time.Now().Format(time.RFC3339)
		c.Tags = extractTags(c.Text)
		c.Links = findLinks(c.Text)
		cardsMu.Lock()
		cards = append(cards, c)
		saveCardsLocked()
		cardsMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(c)
	case http.MethodDelete:
		cardsMu.Lock()
		cards = nil
		os.WriteFile(dataFile, []byte("[]"), 0644)
		cardsMu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}
}

func cardByIDHandler(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/cards/")
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		cardsMu.RLock()
		defer cardsMu.RUnlock()
		for _, c := range cards {
			if c.ID == id {
				json.NewEncoder(w).Encode(c)
				return
			}
		}
		http.NotFound(w, r)
	case http.MethodPut:
		var updated Card
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &updated)
		if updated.Name == "" {
			http.Error(w, "name required", http.StatusBadRequest)
			return
		}
		cardsMu.Lock()
		defer cardsMu.Unlock()
		for i, c := range cards {
			if c.ID == id {
				updated.ID = id
				updated.CreatedAt = c.CreatedAt
				updated.Tags = extractTags(updated.Text)
				cards[i] = updated
				autoLinkLocked()
				saveCardsLocked()
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(updated)
				return
			}
		}
		http.NotFound(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func exportHandler(w http.ResponseWriter, r *http.Request) {
	cardsMu.RLock()
	defer cardsMu.RUnlock()
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=cards.csv")
	wr := csv.NewWriter(w)
	wr.Write([]string{"ID", "Name", "Kind", "Faction", "Nation", "Species", "Text", "Tags", "Links", "CreatedAt"})
	for _, c := range cards {
		wr.Write([]string{
			c.ID, c.Name, c.Kind, c.Faction, c.Nation, c.Species,
			c.Text,
			strings.Join(c.Tags, ";"),
			strings.Join(c.Links, ";"),
			c.CreatedAt,
		})
	}
	wr.Flush()
}

func importHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file required", http.StatusBadRequest)
		return
	}
	defer file.Close()
	rd := csv.NewReader(file)
	records, _ := rd.ReadAll()
	var imported []Card
	for i, row := range records {
		if i == 0 {
			continue
		}
		if len(row) < 3 {
			continue
		}
		c := Card{
			ID:        fmt.Sprintf("card_%d_%d", time.Now().UnixNano(), i),
			Name:      row[1],
			Kind:      safeGet(row, 2),
			Faction:   safeGet(row, 3),
			Nation:    safeGet(row, 4),
			Species:   safeGet(row, 5),
			Text:      safeGet(row, 6),
			CreatedAt: time.Now().Format(time.RFC3339),
		}
		if len(row) > 7 && row[7] != "" {
			c.Tags = strings.Split(row[7], ";")
		}
		c.Links = findLinks(c.Text)
		imported = append(imported, c)
	}
	cardsMu.Lock()
	cards = append(cards, imported...)
	autoLinkLocked()
	saveCardsLocked()
	cardsMu.Unlock()
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func findLinks(text string) []string {
	cardsMu.RLock()
	defer cardsMu.RUnlock()
	return findLinksLocked(text)
}

func findLinksLocked(text string) []string {
	matches := nameRe.FindAllString(text, -1)
	seen := make(map[string]bool)
	var links []string
	for _, m := range matches {
		for _, c := range cards {
			if strings.EqualFold(c.Name, m) && !seen[c.Name] {
				seen[c.Name] = true
				links = append(links, c.Name)
			}
		}
	}
	return links
}

func autoLink() {
	cardsMu.Lock()
	defer cardsMu.Unlock()
	autoLinkLocked()
}

func autoLinkLocked() {
	for i, c := range cards {
		cards[i].Links = findLinksLocked(c.Text)
	}
}

func extractTags(text string) []string {
	words := strings.Fields(text)
	seen := make(map[string]bool)
	var tags []string
	for _, w := range words {
		w = strings.Trim(w, ".,!?;:'\"()")
		if len(w) > 3 && !seen[w] {
			seen[w] = true
			tags = append(tags, strings.ToLower(w))
		}
	}
	if len(tags) > 10 {
		tags = tags[:10]
	}
	return tags
}

func loadCards() {
	cardsMu.Lock()
	defer cardsMu.Unlock()
	data, err := os.ReadFile(dataFile)
	if err != nil {
		cards = []Card{}
		return
	}
	json.Unmarshal(data, &cards)
}

func saveCardsLocked() {
	data, _ := json.MarshalIndent(cards, "", "  ")
	os.WriteFile(dataFile, data, 0644)
}

func safeGet(row []string, idx int) string {
	if idx >= 0 && idx < len(row) {
		return row[idx]
	}
	return ""
}
