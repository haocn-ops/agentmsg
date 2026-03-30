package agentmsg

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestClientConnectChecksHealthAndCallsCallback(t *testing.T) {
	var connected atomic.Bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"healthy"}`))
	}))
	defer server.Close()

	client, err := NewClient(&ClientConfig{
		APIKey:    "token",
		AgentID:   uuid.New(),
		TenantID:  uuid.New(),
		BaseURL:   server.URL,
		Timeout:   5 * time.Second,
		OnConnect: func() { connected.Store(true) },
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if err := client.Connect(context.Background()); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	if !connected.Load() {
		t.Fatal("expected OnConnect callback to be invoked")
	}
}

func TestSendMessageInjectsIdentityAndReturnsResult(t *testing.T) {
	agentID := uuid.New()
	tenantID := uuid.New()
	resultID := uuid.New()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/messages" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer token" {
			t.Fatalf("unexpected authorization header: %s", auth)
		}

		var msg Message
		if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
			t.Fatalf("failed to decode message: %v", err)
		}
		if msg.SenderID != agentID {
			t.Fatalf("expected sender id %s, got %s", agentID, msg.SenderID)
		}
		if msg.TenantID != tenantID {
			t.Fatalf("expected tenant id %s, got %s", tenantID, msg.TenantID)
		}
		if msg.ConversationID == uuid.Nil {
			t.Fatal("expected conversation id to be set")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(SendResult{
			MessageID: resultID,
			Status:    "pending",
		})
	}))
	defer server.Close()

	client, err := NewClient(&ClientConfig{
		APIKey:   "token",
		AgentID:  agentID,
		TenantID: tenantID,
		BaseURL:  server.URL,
		Timeout:  5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	result, err := client.SendMessage(context.Background(), &Message{
		MessageType:      MessageTypeGeneric,
		RecipientIDs:     []uuid.UUID{uuid.New()},
		Content:          []byte("hello"),
		ContentType:      "text/plain",
		DeliveryGuarantee: DeliveryAtLeastOnce,
	})
	if err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}
	if result.MessageID != resultID {
		t.Fatalf("expected message id %s, got %s", resultID, result.MessageID)
	}
	if result.Status != "pending" {
		t.Fatalf("expected status pending, got %s", result.Status)
	}
}

func TestUpdateAgentCapabilitiesMapsStructuredNotFoundError(t *testing.T) {
	agentID := uuid.New()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":"agent_not_found","message":"agent not found"}}`))
	}))
	defer server.Close()

	client, err := NewClient(&ClientConfig{
		APIKey:  "token",
		AgentID: agentID,
		BaseURL: server.URL,
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	_, err = client.UpdateAgentCapabilities(context.Background(), Capabilities{
		{Type: CapabilityTextGeneration, Description: "text"},
	})
	if err != ErrAgentNotFound {
		t.Fatalf("expected ErrAgentNotFound, got %v", err)
	}
}
