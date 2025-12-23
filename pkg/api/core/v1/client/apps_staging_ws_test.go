package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/cli/settings"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gorilla/websocket"
)

func TestStagingCompleteStreamSuccess(t *testing.T) {
	upgrader := websocket.Upgrader{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade failed: %v", err)
		}
		defer conn.Close()

		event := models.StageCompleteEvent{
			StageID:   "id",
			Namespace: "ns",
			Status:    models.StageStatusSucceeded,
			Completed: true,
		}
		payload, _ := json.Marshal(event)
		if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
			t.Fatalf("write failed: %v", err)
		}
		_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second))
	}))
	defer server.Close()

	wsURL := strings.Replace(server.URL, "http", "ws", 1)
	c := &Client{
		log:           tracelog.NewLogger(),
		Settings:      &settings.Settings{WSS: wsURL, API: server.URL},
		HttpClient:    server.Client(),
		customHeaders: http.Header{},
	}

	seen := false
	err := c.StagingCompleteStream(context.Background(), "ns", "id", func(event models.StageCompleteEvent) error {
		if !event.Completed || event.Status != models.StageStatusSucceeded {
			t.Fatalf("unexpected event: %+v", event)
		}
		seen = true
		return nil
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !seen {
		t.Fatalf("expected to see completion event")
	}
}

func TestStagingCompleteStreamPropagatesCallbackError(t *testing.T) {
	upgrader := websocket.Upgrader{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade failed: %v", err)
		}
		defer conn.Close()

		event := models.StageCompleteEvent{
			StageID:   "id",
			Namespace: "ns",
			Status:    models.StageStatusFailed,
			Message:   "boom",
			Completed: true,
		}
		payload, _ := json.Marshal(event)
		if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
			t.Fatalf("write failed: %v", err)
		}
		_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second))
	}))
	defer server.Close()

	wsURL := strings.Replace(server.URL, "http", "ws", 1)
	c := &Client{
		log:           tracelog.NewLogger(),
		Settings:      &settings.Settings{WSS: wsURL, API: server.URL},
		HttpClient:    server.Client(),
		customHeaders: http.Header{},
	}

	err := c.StagingCompleteStream(context.Background(), "ns", "id", func(event models.StageCompleteEvent) error {
		return errors.New(event.Message)
	})

	if err == nil {
		t.Fatalf("expected error from callback, got nil")
	}
}
