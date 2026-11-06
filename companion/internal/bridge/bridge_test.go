package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/ryan-flan/DOINK/companion/internal/savedvars"
)

func death() savedvars.Event {
	return savedvars.Event{
		Type: "death", Char: "Fenrir", Realm: "Forever", Level: 20,
		TS: 1790000000, HasTS: true, Seq: 42,
		Data: map[string]any{"zone": "Westfall", "killer": "Hogger"},
	}
}

func TestConvertDeath(t *testing.T) {
	req, err := Convert(death())
	if err != nil {
		t.Fatal(err)
	}
	if req.Type != "player_died" || req.Timestamp != "2026-09-21T14:13:20Z" {
		t.Fatalf("incorrect envelope: %+v", req)
	}
	want := map[string]any{"character": "Fenrir", "level": 20, "location": "Westfall"}
	if !reflect.DeepEqual(req.Data, want) {
		t.Fatalf("data: got %v want %v", req.Data, want)
	}
}

func TestUnsupportedAndMissingFields(t *testing.T) {
	e := death()
	e.Type = "loot"
	if _, err := Convert(e); !errors.Is(err, ErrUnsupportedEvent) {
		t.Fatalf("unsupported type: %v", err)
	}
	e = death()
	delete(e.Data, "zone")
	if _, err := Convert(e); err == nil {
		t.Fatal("expected missing zone error")
	}
	e = death()
	e.HasTS = false
	if _, err := Convert(e); err == nil {
		t.Fatal("expected missing timestamp error")
	}
}

func TestPostToSymfony(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("unexpected request: %s %s", r.Method, r.Header.Get("Content-Type"))
		}
		var request Request
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if request.Type != "player_died" || request.Data["level"] != float64(20) {
			t.Errorf("bad payload: %+v", request)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	if err := (Client{Endpoint: srv.URL}).Send(context.Background(), death()); err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Fatalf("requests = %d", requests)
	}
}

func TestSymfonyRejectsEvent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"violations":[{"field":"level","message":"invalid"}]}`))
	}))
	defer srv.Close()
	if err := (Client{Endpoint: srv.URL}).Send(context.Background(), death()); err == nil {
		t.Fatal("expected HTTP 422 to be returned as error")
	}
}
