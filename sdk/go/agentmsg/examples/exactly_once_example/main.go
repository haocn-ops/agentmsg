package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"

	agentmsg "github.com/haocn-ops/agentmsg/sdk/go/agentmsg"
)

func main() {
	if err := runExactlyOnceExample(); err != nil {
		log.Fatal(err)
	}
}

func runExactlyOnceExample() error {
	apiKey := os.Getenv("AGENTMSG_API_KEY")
	if apiKey == "" {
		apiKey = "test-api-key"
	}

	agentID := uuid.New()
	tenantID := uuid.New()

	config := &agentmsg.ClientConfig{
		APIKey:   apiKey,
		AgentID:  agentID,
		TenantID: tenantID,
		BaseURL:  "http://localhost:8080",
		Timeout:  30 * time.Second,
	}

	client, err := agentmsg.NewClient(config)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	ctx := context.Background()
	if err := client.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Disconnect()

	log.Println("Exactly-once delivery currently uses the REST send path; realtime receive is not exposed by the server build yet.")

	sendMessageWithIdempotency(ctx, client, "This message will be delivered exactly once")
	sendMessageWithIdempotency(ctx, client, "This message will be delivered exactly once")
	sendMessageWithIdempotency(ctx, client, "This message will be delivered exactly once")

	time.Sleep(2 * time.Second)
	return nil
}

func sendMessageWithIdempotency(ctx context.Context, client *agentmsg.Client, content string) {
	msg := &agentmsg.Message{
		ConversationID:    uuid.New(),
		MessageType:       agentmsg.MessageTypeGeneric,
		RecipientIDs:      []uuid.UUID{uuid.New()},
		Content:           []byte(content),
		ContentType:       "text/plain",
		DeliveryGuarantee: agentmsg.DeliveryExactlyOnce,
	}

	result, err := client.SendMessage(ctx, msg)
	if err != nil {
		log.Printf("Failed to send message: %v", err)
		return
	}
	log.Printf("Message sent with exactly-once guarantee: %s", result.MessageID)
}
