package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestCreateMonitor_AuthHeaderAndDecode проверяет, что клиент шлёт Bearer и
// корректно разбирает обёртку {"monitor": {...}}.
func TestCreateMonitor_AuthHeaderAndDecode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer pv_test" {
			t.Fatalf("Authorization header = %q, want Bearer pv_test", got)
		}
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/monitors" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var in MonitorCreateInput
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if in.Type != "http" || in.Target != "https://example.com" {
			t.Fatalf("unexpected body: %+v", in)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"monitor": Monitor{PublicID: "mon_abc", Type: "http", Name: in.Name, Target: in.Target, Enabled: true},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "pv_test")
	mon, err := c.CreateMonitor(context.Background(), MonitorCreateInput{Type: "http", Name: "test", Target: "https://example.com"})
	if err != nil {
		t.Fatalf("CreateMonitor: %v", err)
	}
	if mon.PublicID != "mon_abc" {
		t.Fatalf("PublicID = %q, want mon_abc", mon.PublicID)
	}
}

// TestFindMonitor_NotFound проверяет поведение при отсутствии монитора в
// списке — Read-путь ресурса полагается на found=false, чтобы убрать ресурс
// из state (resp.State.RemoveResource), а не на HTTP 404 (единичного GET нет).
func TestFindMonitor_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"monitors": []Monitor{{PublicID: "mon_other"}}})
	}))
	defer srv.Close()

	c := New(srv.URL, "pv_test")
	_, found, err := c.FindMonitor(context.Background(), "mon_missing")
	if err != nil {
		t.Fatalf("FindMonitor: %v", err)
	}
	if found {
		t.Fatal("expected found=false for absent public_id")
	}
}

// TestDeleteMonitor_APIErrorIsNotFound проверяет разбор ошибки 404 в
// формате {"error": "..."} и распознавание через IsNotFound.
func TestDeleteMonitor_APIErrorIsNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "monitor not found"})
	}))
	defer srv.Close()

	c := New(srv.URL, "pv_test")
	err := c.DeleteMonitor(context.Background(), "mon_gone")
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsNotFound(err) {
		t.Fatalf("IsNotFound(%v) = false, want true", err)
	}
}

// TestListStatusPages_ResponseKey проверяет фактический ключ ответа
// "status_pages" (сверен с internal/api/statuspages.go handleStatusPageList).
func TestListStatusPages_ResponseKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status_pages": []StatusPage{{PublicID: "stp_1", Slug: "example"}},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "pv_test")
	pages, err := c.ListStatusPages(context.Background())
	if err != nil {
		t.Fatalf("ListStatusPages: %v", err)
	}
	if len(pages) != 1 || pages[0].Slug != "example" {
		t.Fatalf("unexpected pages: %+v", pages)
	}
}
