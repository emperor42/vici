package main

import (
	"crypto/subtle"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
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

const (
	defaultHost         = "127.0.0.1"
	defaultPort         = "8085"
	maxJSONBody         = 1 << 20
	maxImportBody       = 8 << 20
	maxImportedCards    = 10000
	maxCards            = 10000
	maxCardNameLength   = 200
	maxCardTextLength   = 100000
	maxCardMetadataSize = 2000
)

var (
	cards    []Card
	cardsMu  sync.RWMutex
	dataFile = configuredDataFile()
	nameRe   = regexp.MustCompile(`\b([A-Z][a-zA-Z]+)\b`)
	idSerial uint64
)

var allowedKinds = map[string]struct{}{
	"Event":   {},
	"Faction": {},
	"Nation":  {},
	"Species": {},
	"Leader":  {},
}

func main() {
	if err := loadCards(); err != nil {
		log.Fatalf("vici: load %s: %v", dataFile, err)
	}
	addr := listenAddress()
	if err := validateListenSecurity(addr); err != nil {
		log.Fatal(err)
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           newHandler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Printf("vici card demo listening on http://%s", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func newHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", rootHandler)
	mux.HandleFunc("/api/cards", cardsHandler)
	mux.HandleFunc("/api/cards/", cardByIDHandler)
	mux.HandleFunc("/api/export", exportHandler)
	mux.HandleFunc("/api/import", importHandler)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	return demoSecurityHeaders(apiAuth(mux))
}

func apiToken() string { return strings.TrimSpace(os.Getenv("VICI_API_TOKEN")) }

func apiAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := apiToken()
		if want == "" || strings.HasPrefix(r.URL.Path, "/static/") {
			// No token is the local-development mode. main() prevents that
			// mode from being combined with a non-loopback listener.
			next.ServeHTTP(w, r)
			return
		}
		got := strings.TrimSpace(r.Header.Get("X-Vici-Token"))
		if auth := strings.TrimSpace(r.Header.Get("Authorization")); strings.HasPrefix(strings.ToLower(auth), "bearer ") {
			got = strings.TrimSpace(auth[len("Bearer "):])
		}
		if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="vici"`)
			http.Error(w, "card API authentication required", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func demoSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

func validateListenSecurity(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid listen address %q: %w", addr, err)
	}
	host = strings.Trim(host, "[]")
	if host == "localhost" {
		return nil
	}
	ip := net.ParseIP(host)
	if ip != nil && ip.IsLoopback() {
		return nil
	}
	if apiToken() == "" {
		return fmt.Errorf("refusing non-loopback listen address %q without VICI_API_TOKEN", addr)
	}
	return nil
}

func configuredDataFile() string {
	if value := strings.TrimSpace(os.Getenv("VICI_DATA_FILE")); value != "" {
		return value
	}
	return "cards.json"
}

func listenAddress() string {
	host := strings.TrimSpace(os.Getenv("VICI_HOST"))
	if host == "" {
		host = defaultHost
	}
	port := strings.TrimSpace(os.Getenv("VICI_PORT"))
	if port == "" {
		port = defaultPort
	}
	return net.JoinHostPort(host, port)
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	snapshot := cardsSnapshot()
	tmpl, err := template.ParseFiles("templates/index.html")
	if err != nil {
		http.Error(w, "template unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, snapshot); err != nil {
		log.Printf("vici: render home page: %v", err)
	}
}

func cardsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, cardsSnapshot())
	case http.MethodPost:
		var card Card
		if err := decodeJSON(w, r, &card); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
		if err := normalizeCard(&card); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		card.ID = nextCardID()
		card.CreatedAt = time.Now().UTC().Format(time.RFC3339)
		card.Tags = extractTags(card.Text)

		cardsMu.Lock()
		if len(cards) >= maxCards {
			cardsMu.Unlock()
			http.Error(w, "card storage limit reached", http.StatusRequestEntityTooLarge)
			return
		}
		previous := cloneCards(cards)
		cards = append(cards, card)
		autoLinkLocked()
		card = cards[len(cards)-1]
		if err := saveCardsLocked(); err != nil {
			cards = previous
			cardsMu.Unlock()
			http.Error(w, "could not save card data", http.StatusInternalServerError)
			return
		}
		cardsMu.Unlock()
		writeJSON(w, http.StatusCreated, card)
	case http.MethodDelete:
		cardsMu.Lock()
		previous := cloneCards(cards)
		cards = []Card{}
		if err := saveCardsLocked(); err != nil {
			cards = previous
			cardsMu.Unlock()
			http.Error(w, "could not save card data", http.StatusInternalServerError)
			return
		}
		cardsMu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	default:
		w.Header().Set("Allow", "GET, POST, DELETE")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func cardByIDHandler(w http.ResponseWriter, r *http.Request) {
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/cards/"), "/")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		for _, card := range cardsSnapshot() {
			if card.ID == id {
				writeJSON(w, http.StatusOK, card)
				return
			}
		}
		http.NotFound(w, r)
	case http.MethodPut:
		var updated Card
		if err := decodeJSON(w, r, &updated); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
		if err := normalizeCard(&updated); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		updated.ID = id
		updated.Tags = extractTags(updated.Text)

		cardsMu.Lock()
		previous := cloneCards(cards)
		found := false
		for i, card := range cards {
			if card.ID != id {
				continue
			}
			found = true
			updated.CreatedAt = card.CreatedAt
			cards[i] = updated
			break
		}
		if !found {
			cardsMu.Unlock()
			http.NotFound(w, r)
			return
		}
		autoLinkLocked()
		for _, card := range cards {
			if card.ID == id {
				updated = card
				break
			}
		}
		if err := saveCardsLocked(); err != nil {
			cards = previous
			cardsMu.Unlock()
			http.Error(w, "could not save card data", http.StatusInternalServerError)
			return
		}
		cardsMu.Unlock()
		writeJSON(w, http.StatusOK, updated)
	default:
		w.Header().Set("Allow", "GET, PUT")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func exportHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	snapshot := cardsSnapshot()
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=cards.csv")
	writer := csv.NewWriter(w)
	if err := writer.Write([]string{"ID", "Name", "Kind", "Faction", "Nation", "Species", "Text", "Tags", "Links", "CreatedAt"}); err != nil {
		log.Printf("vici: write CSV header: %v", err)
		return
	}
	for _, card := range snapshot {
		if err := writer.Write([]string{
			card.ID,
			card.Name,
			card.Kind,
			card.Faction,
			card.Nation,
			card.Species,
			card.Text,
			strings.Join(card.Tags, ";"),
			strings.Join(card.Links, ";"),
			card.CreatedAt,
		}); err != nil {
			log.Printf("vici: write CSV row: %v", err)
			return
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		log.Printf("vici: flush CSV: %v", err)
	}
}

func importHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxImportBody)
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	imported := make([]Card, 0)
	rowNumber := 0
	for {
		row, readErr := reader.Read()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			http.Error(w, fmt.Sprintf("invalid CSV near row %d", rowNumber+1), http.StatusBadRequest)
			return
		}
		rowNumber++
		if rowNumber > maxImportedCards+1 {
			http.Error(w, "too many CSV rows", http.StatusRequestEntityTooLarge)
			return
		}
		if rowNumber == 1 {
			continue
		}
		if len(row) == 0 || allEmpty(row) {
			continue
		}
		card := Card{
			Name:    safeGet(row, 1),
			Kind:    safeGet(row, 2),
			Faction: safeGet(row, 3),
			Nation:  safeGet(row, 4),
			Species: safeGet(row, 5),
			Text:    safeGet(row, 6),
		}
		if err := normalizeCard(&card); err != nil {
			http.Error(w, fmt.Sprintf("row %d: %s", rowNumber, err), http.StatusBadRequest)
			return
		}
		card.ID = nextCardID()
		card.CreatedAt = time.Now().UTC().Format(time.RFC3339)
		card.Tags = extractTags(card.Text)
		imported = append(imported, card)
	}

	cardsMu.Lock()
	if len(cards)+len(imported) > maxCards {
		cardsMu.Unlock()
		http.Error(w, "card storage limit reached", http.StatusRequestEntityTooLarge)
		return
	}
	previous := cloneCards(cards)
	cards = append(cards, imported...)
	autoLinkLocked()
	if err := saveCardsLocked(); err != nil {
		cards = previous
		cardsMu.Unlock()
		http.Error(w, "could not save imported cards", http.StatusInternalServerError)
		return
	}
	cardsMu.Unlock()
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, value any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func normalizeCard(card *Card) error {
	card.Name = strings.TrimSpace(card.Name)
	card.Kind = strings.TrimSpace(card.Kind)
	card.Faction = strings.TrimSpace(card.Faction)
	card.Nation = strings.TrimSpace(card.Nation)
	card.Species = strings.TrimSpace(card.Species)
	if card.Kind == "" {
		card.Kind = "Event"
	}
	if card.Name == "" {
		return errors.New("name required")
	}
	if len(card.Name) > maxCardNameLength {
		return fmt.Errorf("name must be at most %d characters", maxCardNameLength)
	}
	if len(card.Text) > maxCardTextLength {
		return fmt.Errorf("text must be at most %d characters", maxCardTextLength)
	}
	if len(card.Faction) > maxCardMetadataSize || len(card.Nation) > maxCardMetadataSize || len(card.Species) > maxCardMetadataSize {
		return fmt.Errorf("card metadata is too long")
	}
	if _, ok := allowedKinds[card.Kind]; !ok {
		return fmt.Errorf("kind must be Event, Faction, Nation, Species, or Leader")
	}
	// Client-supplied IDs, timestamps, tags, and links are server-owned.
	card.ID = ""
	card.CreatedAt = ""
	card.Tags = nil
	card.Links = nil
	return nil
}

func nextCardID() string {
	return fmt.Sprintf("card_%d_%d", time.Now().UnixNano(), atomic.AddUint64(&idSerial, 1))
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("vici: encode JSON response: %v", err)
	}
}

func cardsSnapshot() []Card {
	cardsMu.RLock()
	defer cardsMu.RUnlock()
	return cloneCards(cards)
}

func cloneCards(source []Card) []Card {
	if source == nil {
		return []Card{}
	}
	out := make([]Card, len(source))
	for i, card := range source {
		out[i] = card
		if card.Tags != nil {
			out[i].Tags = append([]string(nil), card.Tags...)
		}
		if card.Links != nil {
			out[i].Links = append([]string(nil), card.Links...)
		}
	}
	return out
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
	for _, match := range matches {
		for _, card := range cards {
			if strings.EqualFold(card.Name, match) && !seen[card.Name] {
				seen[card.Name] = true
				links = append(links, card.Name)
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
	for i, card := range cards {
		cards[i].Links = findLinksLocked(card.Text)
	}
}

func extractTags(text string) []string {
	words := strings.Fields(text)
	seen := make(map[string]bool)
	var tags []string
	for _, word := range words {
		word = strings.Trim(word, ".,!?;:'\"()")
		if len(word) > 3 && !seen[word] {
			seen[word] = true
			tags = append(tags, strings.ToLower(word))
		}
	}
	if len(tags) > 10 {
		tags = tags[:10]
	}
	return tags
}

func loadCards() error {
	cardsMu.Lock()
	defer cardsMu.Unlock()
	data, err := os.ReadFile(dataFile)
	if errors.Is(err, os.ErrNotExist) {
		cards = []Card{}
		return nil
	}
	if err != nil {
		return err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		cards = []Card{}
		return nil
	}
	var loaded []Card
	if err := json.Unmarshal(data, &loaded); err != nil {
		return err
	}
	if loaded == nil {
		loaded = []Card{}
	}
	if len(loaded) > maxCards {
		return fmt.Errorf("card file contains %d cards; maximum is %d", len(loaded), maxCards)
	}
	cards = loaded
	return nil
}

func saveCardsLocked() error {
	data, err := json.MarshalIndent(cards, "", "  ")
	if err != nil {
		return err
	}
	path := dataFile
	if strings.TrimSpace(path) == "" {
		path = "cards.json"
	}
	dir := filepath.Dir(path)
	if dir == "" {
		dir = "."
	}
	temp, err := os.CreateTemp(dir, ".vici-cards-*.tmp")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)

	if err := temp.Chmod(0600); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempName, path)
}

func allEmpty(row []string) bool {
	for _, value := range row {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}

func safeGet(row []string, index int) string {
	if index >= 0 && index < len(row) {
		return row[index]
	}
	return ""
}
