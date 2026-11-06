package runner

import (
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/ryan-flan/DOINK/companion/internal/savedvars"
)

func apiDeath(seq int) savedvars.Event {
    return savedvars.Event{
        Seq: seq, Type: "death", Char: "Fenrir", Realm: "Forever",
        Level: 20, HasTS: true, TS: 1790000000,
        Data: map[string]any{"zone": "Westfall", "killer": "Hogger"},
    }
}

func TestAPIOnlySendsDeathAndDoesNotCallDiscord(t *testing.T) {
    f := newFixture(t)
    sent := 0
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        sent++
        if r.Method != http.MethodPost || r.URL.Path != "/api/events" {
            t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
        }
        var body struct {
            Type string `json:"type"`
            Data struct {
                Character string `json:"character"`
                Level int `json:"level"`
                Location string `json:"location"`
            } `json:"data"`
        }
        if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
            t.Error(err)
        }
        if body.Type != "player_died" || body.Data.Character != "Fenrir" || body.Data.Level != 20 || body.Data.Location != "Westfall" {
            t.Errorf("unexpected body: %+v", body)
        }
        w.WriteHeader(http.StatusOK)
    }))
    defer server.Close()
    f.cfg.APIURL = server.URL + "/api/events"
    f.sender.fail = true // Discord must not be used when api_url is configured
    state := f.state()
    events := []savedvars.Event{apiDeath(1), {Seq: 2, Type: "loot", Char: "Fenrir", Realm: "Forever", Data: map[string]any{}}}
    if !PostEvents(f.deps(state), "Fenrir-Forever", events, nil, 0) {
        t.Fatal("posting failed")
    }
    if sent != 1 || len(f.sender.sent) != 0 || state.IsPending("Fenrir-Forever", 2) {
        t.Errorf("sent=%d discord=%d pending2=%v", sent, len(f.sender.sent), state.IsPending("Fenrir-Forever", 2))
    }
    if !PostEvents(f.deps(state), "Fenrir-Forever", nil, nil, 0) {
        t.Fatal("empty batch failed")
    }
}

func TestAPIErrorDoesNotAdvanceDeliveryStateAndCanRetry(t *testing.T) {
    f := newFixture(t)
    calls := 0
    failing := true
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        calls++
        if failing {
            w.WriteHeader(http.StatusServiceUnavailable)
        } else {
            w.WriteHeader(http.StatusOK)
        }
    }))
    defer server.Close()
    f.cfg.APIURL = server.URL
    state := f.state()
    events := []savedvars.Event{apiDeath(5)}
    if PostEvents(f.deps(state), "Fenrir-Forever", events, nil, 10) {
        t.Fatal("expected API error")
    }
    if !state.IsPending("Fenrir-Forever", 5) {
        t.Fatal("failed event must remain pending")
    }
    failing = false
    if !PostEvents(f.deps(state), "Fenrir-Forever", events, nil, 10) {
        t.Fatal("retry failed")
    }
    if state.IsPending("Fenrir-Forever", 5) || calls != 2 {
        t.Fatalf("pending=%v requests=%d", state.IsPending("Fenrir-Forever", 5), calls)
    }
}

func TestAPIMissingFieldsStaysPending(t *testing.T) {
    f := newFixture(t)
    f.cfg.APIURL = "http://127.0.0.1:1/api/events"
    state := f.state()
    invalid := apiDeath(4)
    delete(invalid.Data, "zone")
    if PostEvents(f.deps(state), "Fenrir-Forever", []savedvars.Event{invalid}, nil, 0) {
        t.Fatal("expected validation error")
    }
    if !state.IsPending("Fenrir-Forever", 4) {
        t.Fatal("invalid event should remain pending")
    }
}
