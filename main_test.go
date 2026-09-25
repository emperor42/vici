package main

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "vici-test-")
	if err != nil {
		panic(err)
	}
	originalDataFile := dataFile
	dataFile = filepath.Join(dir, "cards.json")
	code := m.Run()
	cardsMu.Lock()
	cards = nil
	cardsMu.Unlock()
	dataFile = originalDataFile
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func resetCards(t *testing.T) {
	t.Helper()
	cardsMu.Lock()
	cards = []Card{}
	cardsMu.Unlock()
	t.Cleanup(func() {
		cardsMu.Lock()
		cards = []Card{}
		cardsMu.Unlock()
	})
}

func validKinds() map[string]string {
	return map[string]string{
		"Event":   "#c62828",
		"Faction": "#ef6c00",
		"Nation":  "#1565c0",
		"Species": "#2e7d32",
		"Leader":  "#7b1fa2",
	}
}

func TestKindColourMapping(t *testing.T) {
	colours := validKinds()
	expected := map[string]string{
		"Event":   "#c62828",
		"Faction": "#ef6c00",
		"Nation":  "#1565c0",
		"Species": "#2e7d32",
		"Leader":  "#7b1fa2",
	}
	for kind, clr := range expected {
		got, ok := colours[kind]
		if !ok {
			t.Errorf("Missing colour mapping for kind %q", kind)
			continue
		}
		if got != clr {
			t.Errorf("Colour for kind %q = %q, want %q", kind, got, clr)
		}
	}
	if len(colours) != 5 {
		t.Errorf("Expected 5 kind colours, got %d", len(colours))
	}
}

func TestExtractTags(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"empty", "", nil},
		{"short words skipped", "a an the cat", nil},
		{"basic extraction", "The quick brown fox jumps", []string{"quick", "brown", "jumps"}},
		{"punctuation stripped", "hello, world! test;", []string{"hello", "world", "test"}},
		{"lowercased", "HELLO World", []string{"hello", "world"}},
		{"deduplicated", "run run run", nil},
		{"max 10 tags", "one two three four five six seven eight nine ten eleven twelve", []string{"three", "four", "five", "seven", "eight", "nine", "eleven", "twelve"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTags(tt.input)
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("extractTags(%q) = %v (len=%d), want %v (len=%d)", tt.input, got, len(got), tt.want, len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("extractTags(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestSafeGet(t *testing.T) {
	row := []string{"a", "b", "c"}
	tests := []struct {
		name string
		idx  int
		want string
	}{
		{"first", 0, "a"},
		{"middle", 1, "b"},
		{"last", 2, "c"},
		{"out of bounds negative", -1, ""},
		{"out of bounds high", 10, ""},
		{"empty row", 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var r []string
			if tt.name == "empty row" {
				r = []string{}
			} else {
				r = row
			}
			got := safeGet(r, tt.idx)
			if got != tt.want {
				t.Errorf("safeGet(%v, %d) = %q, want %q", r, tt.idx, got, tt.want)
			}
		})
	}
}

func TestFindLinksLocked(t *testing.T) {
	cardsMu.Lock()
	cards = []Card{
		{ID: "1", Name: "Alpha"},
		{ID: "2", Name: "Beta"},
		{ID: "3", Name: "Gamma"},
	}
	cardsMu.Unlock()

	tests := []struct {
		name string
		text string
		want []string
	}{
		{"no matches", "nothing here", nil},
		{"single match", "Alpha is here", []string{"Alpha"}},
		{"multiple matches", "Alpha and Beta", []string{"Alpha", "Beta"}},
		{"case insensitive", "Alpha and BETA", []string{"Alpha", "Beta"}},
		{"no partial matches", "Alph and Bet", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cardsMu.RLock()
			got := findLinksLocked(tt.text)
			cardsMu.RUnlock()
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("findLinksLocked(%q) = %v, want %v", tt.text, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("findLinksLocked(%q)[%d] = %q, want %q", tt.text, i, got[i], tt.want[i])
				}
			}
		})
	}

	cardsMu.Lock()
	cards = nil
	cardsMu.Unlock()
}

func TestCardCreation(t *testing.T) {
	card := Card{
		ID:      "test_1",
		Name:    "Test Card",
		Kind:    "Event",
		Faction: "Test Faction",
		Nation:  "Test Nation",
		Species: "Test Species",
		Text:    "Some description text",
		Tags:    []string{"tag1", "tag2"},
		Links:   []string{"Other Card"},
	}

	if card.ID != "test_1" {
		t.Errorf("card.ID = %q, want %q", card.ID, "test_1")
	}
	if card.Name != "Test Card" {
		t.Errorf("card.Name = %q, want %q", card.Name, "Test Card")
	}
	if card.Kind != "Event" {
		t.Errorf("card.Kind = %q, want %q", card.Kind, "Event")
	}
	if len(card.Tags) != 2 {
		t.Errorf("len(card.Tags) = %d, want 2", len(card.Tags))
	}
}

func TestCardJSONRoundTrip(t *testing.T) {
	original := Card{
		ID:        "test_json_1",
		Name:      "JSON Card",
		Kind:      "Species",
		Faction:   "",
		Nation:    "Test Nation",
		Species:   "",
		Text:      "Some text with a reference to Another Card",
		Tags:      []string{"reference", "text"},
		Links:     []string{"Another Card"},
		CreatedAt: "2026-01-01T00:00:00Z",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if !strings.Contains(string(data), `"kind":"Species"`) {
		t.Errorf("JSON output missing kind field: %s", string(data))
	}
	if !strings.Contains(string(data), `"name":"JSON Card"`) {
		t.Errorf("JSON output missing name field: %s", string(data))
	}

	var decoded Card
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	if decoded.ID != original.ID {
		t.Errorf("decoded.ID = %q, want %q", decoded.ID, original.ID)
	}
	if decoded.Kind != original.Kind {
		t.Errorf("decoded.Kind = %q, want %q", decoded.Kind, original.Kind)
	}
	if decoded.Name != original.Name {
		t.Errorf("decoded.Name = %q, want %q", decoded.Name, original.Name)
	}
}

func TestAutoLinkLocked(t *testing.T) {
	cardsMu.Lock()
	cards = []Card{
		{ID: "1", Name: "Alpha", Text: "Referencing Beta here"},
		{ID: "2", Name: "Beta", Text: "Referencing Gamma here"},
		{ID: "3", Name: "Gamma", Text: "No references"},
	}
	cardsMu.Unlock()

	cardsMu.Lock()
	autoLinkLocked()
	cardsMu.Unlock()

	cardsMu.RLock()
	if len(cards[0].Links) != 1 || cards[0].Links[0] != "Beta" {
		t.Errorf("card[0].Links = %v, want [Beta]", cards[0].Links)
	}
	if len(cards[1].Links) != 1 || cards[1].Links[0] != "Gamma" {
		t.Errorf("card[1].Links = %v, want [Gamma]", cards[1].Links)
	}
	if len(cards[2].Links) != 0 {
		t.Errorf("card[2].Links = %v, want []", cards[2].Links)
	}
	cardsMu.RUnlock()

	cardsMu.Lock()
	cards = nil
	cardsMu.Unlock()
}

func TestAPIRequiresTokenWhenConfigured(t *testing.T) {
	t.Setenv("VICI_API_TOKEN", "test-token")
	resetCards(t)
	handler := newHandler()
	body := `{"name":"Protected card","kind":"Event","text":"no"}`
	req := httptest.NewRequest(http.MethodPost, "/api/cards", strings.NewReader(body))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated POST status = %d, want 401", rr.Code)
	}
	cardsMu.RLock()
	count := len(cards)
	cardsMu.RUnlock()
	if count != 0 {
		t.Fatalf("unauthenticated request changed cards: %d", count)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/cards", strings.NewReader(body))
	req.Header.Set("X-Vici-Token", "test-token")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("authenticated POST status = %d, want 201", rr.Code)
	}
}

func TestNonLoopbackListenRequiresToken(t *testing.T) {
	t.Setenv("VICI_API_TOKEN", "")
	if err := validateListenSecurity("0.0.0.0:8085"); err == nil {
		t.Fatal("non-loopback listener without token was accepted")
	}
	t.Setenv("VICI_API_TOKEN", "test-token")
	if err := validateListenSecurity("0.0.0.0:8085"); err != nil {
		t.Fatalf("non-loopback listener with token rejected: %v", err)
	}
}

func TestCardsAPIEndpoint(t *testing.T) {
	cardsMu.Lock()
	cards = []Card{
		{ID: "api_test_1", Name: "API Card", Kind: "Nation", Text: "test", CreatedAt: "now"},
	}
	cardsMu.Unlock()

	req := httptest.NewRequest("GET", "/api/cards", nil)
	w := httptest.NewRecorder()
	cardsHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /api/cards status = %d, want 200", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var result []Card
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("Expected 1 card, got %d", len(result))
	}
	if result[0].Name != "API Card" {
		t.Errorf("Card name = %q, want %q", result[0].Name, "API Card")
	}
	if result[0].Kind != "Nation" {
		t.Errorf("Card kind = %q, want %q", result[0].Kind, "Nation")
	}

	cardsMu.Lock()
	cards = nil
	cardsMu.Unlock()
}

func TestCardsAPIPost(t *testing.T) {
	cardsMu.Lock()
	cards = nil
	cardsMu.Unlock()

	body := `{"name":"New Card","kind":"Leader","text":"A new leader"}`
	req := httptest.NewRequest("POST", "/api/cards", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	cardsHandler(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("POST /api/cards status = %d, want 201", w.Code)
	}

	var result Card
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if result.Name != "New Card" {
		t.Errorf("Card name = %q, want %q", result.Name, "New Card")
	}
	if result.Kind != "Leader" {
		t.Errorf("Card kind = %q, want %q", result.Kind, "Leader")
	}
	if result.ID == "" {
		t.Error("Card ID should not be empty")
	}
	if result.CreatedAt == "" {
		t.Error("Card CreatedAt should not be empty")
	}
	if len(result.Tags) == 0 {
		t.Error("Card Tags should not be empty (extracted from text)")
	}

	cardsMu.Lock()
	cards = nil
	cardsMu.Unlock()
}

func TestCardsAPIPostMissingName(t *testing.T) {
	body := `{"kind":"Event","text":"no name"}`
	req := httptest.NewRequest("POST", "/api/cards", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	cardsHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("POST missing name status = %d, want 400", w.Code)
	}
}

func TestCardsAPIDelete(t *testing.T) {
	cardsMu.Lock()
	cards = []Card{{ID: "del_test", Name: "Delete Me", Kind: "Event"}}
	cardsMu.Unlock()

	req := httptest.NewRequest("DELETE", "/api/cards", nil)
	w := httptest.NewRecorder()
	cardsHandler(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("DELETE /api/cards status = %d, want 204", w.Code)
	}

	cardsMu.RLock()
	if len(cards) != 0 {
		t.Errorf("Expected empty cards after DELETE, got %d", len(cards))
	}
	cardsMu.RUnlock()
}

func TestCardByIDHandler(t *testing.T) {
	cardsMu.Lock()
	cards = []Card{
		{ID: "id_get_1", Name: "Get Test", Kind: "Event", Text: "test"},
		{ID: "id_get_2", Name: "Other", Kind: "Faction"},
	}
	cardsMu.Unlock()

	t.Run("get existing", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/cards/id_get_1", nil)
		w := httptest.NewRecorder()
		cardByIDHandler(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		var c Card
		json.Unmarshal(w.Body.Bytes(), &c)
		if c.Name != "Get Test" {
			t.Errorf("name = %q, want %q", c.Name, "Get Test")
		}
	})

	t.Run("get nonexistent", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/cards/nonexistent", nil)
		w := httptest.NewRecorder()
		cardByIDHandler(w, req)
		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", w.Code)
		}
	})

	t.Run("put update", func(t *testing.T) {
		body := `{"name":"Updated Card","kind":"Nation","text":"updated"}`
		req := httptest.NewRequest("PUT", "/api/cards/id_get_2", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		cardByIDHandler(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		var c Card
		json.Unmarshal(w.Body.Bytes(), &c)
		if c.Name != "Updated Card" {
			t.Errorf("name = %q, want %q", c.Name, "Updated Card")
		}
		if c.Kind != "Nation" {
			t.Errorf("kind = %q, want %q", c.Kind, "Nation")
		}
		if c.ID != "id_get_2" {
			t.Errorf("id should be preserved = %q, want %q", c.ID, "id_get_2")
		}
	})

	cardsMu.Lock()
	cards = nil
	cardsMu.Unlock()
}

func TestCardKindFilterBehaviour(t *testing.T) {
	cardsMu.Lock()
	cards = []Card{
		{ID: "f1", Name: "Event Card", Kind: "Event"},
		{ID: "f2", Name: "Faction Card", Kind: "Faction"},
		{ID: "f3", Name: "Nation Card", Kind: "Nation"},
		{ID: "f4", Name: "Species Card", Kind: "Species"},
		{ID: "f5", Name: "Leader Card", Kind: "Leader"},
	}
	cardsMu.Unlock()

	kinds := map[string]int{
		"Event":   1,
		"Faction": 1,
		"Nation":  1,
		"Species": 1,
		"Leader":  1,
	}

	for kind, expected := range kinds {
		var filtered []Card
		cardsMu.RLock()
		for _, c := range cards {
			if c.Kind == kind {
				filtered = append(filtered, c)
			}
		}
		cardsMu.RUnlock()
		if len(filtered) != expected {
			t.Errorf("kind=%q: got %d cards, want %d", kind, len(filtered), expected)
		}
	}

	cardsMu.RLock()
	allKinds := len(cards)
	cardsMu.RUnlock()
	if allKinds != 5 {
		t.Errorf("total cards = %d, want 5", allKinds)
	}

	cardsMu.Lock()
	cards = nil
	cardsMu.Unlock()
}

func TestExportHandler(t *testing.T) {
	cardsMu.Lock()
	cards = []Card{
		{ID: "exp1", Name: "Export A", Kind: "Event", Text: "desc a", Tags: []string{"a", "b"}, Links: []string{"Export B"}, CreatedAt: "now"},
		{ID: "exp2", Name: "Export B", Kind: "Species", Text: "desc b", CreatedAt: "then"},
	}
	cardsMu.Unlock()

	req := httptest.NewRequest("GET", "/api/export", nil)
	w := httptest.NewRecorder()
	exportHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "text/csv" {
		t.Errorf("Content-Type = %q, want text/csv", ct)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Export A") {
		t.Error("CSV missing card name Export A")
	}
	if !strings.Contains(body, "Species") {
		t.Error("CSV missing kind Species")
	}
	if !strings.Contains(body, "a;b") {
		t.Error("CSV missing tags")
	}

	cardsMu.Lock()
	cards = nil
	cardsMu.Unlock()
}

func TestImportHandlerAssignsServerFields(t *testing.T) {
	resetCards(t)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "cards.csv")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	csvData := "ID,Name,Kind,Faction,Nation,Species,Text,Tags,Links,CreatedAt\n" +
		"client-id-1,Alpha,Event,Faction,Nation,Species,References Beta,tags,links,client-time\n" +
		"client-id-2,Beta,Leader,,,,Mentions Alpha,tags,links,client-time\n"
	if _, err := part.Write([]byte(csvData)); err != nil {
		t.Fatalf("write CSV: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/import", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	importHandler(w, req)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusSeeOther)
	}

	got := cardsSnapshot()
	if len(got) != 2 {
		t.Fatalf("imported %d cards, want 2", len(got))
	}
	for i, card := range got {
		if card.ID == "" || card.ID == "client-id-1" || card.ID == "client-id-2" {
			t.Errorf("card %d retained a client ID: %q", i, card.ID)
		}
		if card.CreatedAt == "" {
			t.Errorf("card %d has no server timestamp", i)
		}
		if len(card.Tags) == 0 {
			t.Errorf("card %d has no derived tags", i)
		}
	}
	if len(got[0].Links) != 1 || got[0].Links[0] != "Beta" {
		t.Errorf("first card links = %v, want [Beta]", got[0].Links)
	}
	if len(got[1].Links) != 1 || got[1].Links[0] != "Alpha" {
		t.Errorf("second card links = %v, want [Alpha]", got[1].Links)
	}
}

func TestCardsAPIRejectsMalformedJSON(t *testing.T) {
	resetCards(t)
	req := httptest.NewRequest(http.MethodPost, "/api/cards", bytes.NewBufferString("{"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	cardsHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestNormalizeCardOwnsServerFields(t *testing.T) {
	card := Card{
		ID:        "client-id",
		Name:      "  Example  ",
		Kind:      "",
		Text:      "A description",
		Tags:      []string{"client-tag"},
		CreatedAt: "client-time",
		Links:     []string{"client-link"},
	}
	if err := normalizeCard(&card); err != nil {
		t.Fatalf("normalizeCard returned error: %v", err)
	}
	if card.Name != "Example" || card.Kind != "Event" {
		t.Fatalf("normalized name/kind = %q/%q", card.Name, card.Kind)
	}
	if card.ID != "" || card.CreatedAt != "" || card.Tags != nil || card.Links != nil {
		t.Fatalf("server-owned fields were retained: %+v", card)
	}
}

func TestSaveAndLoadCards(t *testing.T) {
	resetCards(t)
	cardsMu.Lock()
	cards = []Card{{ID: "roundtrip", Name: "Round Trip", Kind: "Event", Text: "saved"}}
	if err := saveCardsLocked(); err != nil {
		cardsMu.Unlock()
		t.Fatalf("saveCardsLocked returned error: %v", err)
	}
	cardsMu.Unlock()

	cardsMu.Lock()
	cards = nil
	cardsMu.Unlock()
	if err := loadCards(); err != nil {
		t.Fatalf("loadCards returned error: %v", err)
	}
	got := cardsSnapshot()
	if len(got) != 1 || got[0].ID != "roundtrip" || got[0].Name != "Round Trip" {
		t.Fatalf("loaded cards = %+v", got)
	}
}

func TestCardByIDRejectsUnsupportedMethod(t *testing.T) {
	resetCards(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/cards/does-not-matter", nil)
	w := httptest.NewRecorder()

	cardByIDHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
	if got := w.Header().Get("Allow"); got != "GET, PUT" {
		t.Fatalf("Allow = %q, want %q", got, "GET, PUT")
	}
}
