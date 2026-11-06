// Package bridge converts DOINK events into the WoW AI Companion HTTP contract.
package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ryan-flan/DOINK/companion/internal/savedvars"
)

var ErrUnsupportedEvent = errors.New("unsupported DOINK event")

type Request struct {
	Type      string         `json:"type"`
	Timestamp string         `json:"timestamp"`
	Data      map[string]any `json:"data"`
}

// Convert maps only event types supported by the current Symfony API.
// Additional DOINK event types should be added when matching Symfony DTOs exist.
func Convert(e savedvars.Event) (Request, error) {
	if e.Type != "death" {
		return Request{}, fmt.Errorf("%w: %s", ErrUnsupportedEvent, e.Type)
	}
	character := strings.TrimSpace(e.FullName())
	zone, _ := e.Data["zone"].(string)
	zone = strings.TrimSpace(zone)
	if character == "" || e.Level < 1 || zone == "" {
		return Request{}, errors.New("death event needs character, positive level and zone")
	}
	if !e.HasTS {
		return Request{}, errors.New("death event has no timestamp")
	}
	return Request{
		Type:      "player_died",
		Timestamp: time.Unix(e.TS, 0).UTC().Format(time.RFC3339),
		Data: map[string]any{
			"character": character,
			"level":     e.Level,
			"location":  zone,
		},
	}, nil
}

type Client struct {
	Endpoint string
	HTTP     *http.Client
}

// Send posts one mapped event. A non-2xx response is an error, so callers
// must not mark the original event as delivered on failure.
func (c Client) Send(ctx context.Context, e savedvars.Event) error {
	payload, err := Convert(e)
	if err != nil {
		return err
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode event: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("build HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	response, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("post event: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 512))
		return fmt.Errorf("Symfony returned HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}
