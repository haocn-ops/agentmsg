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
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	apiKey := os.Getenv("AGENTMSG_API_KEY")
	if apiKey == "" {
		apiKey = "test-api-key"
	}

	agentID := uuid.New()
	tenantID := uuid.New()

	config := &agentmsg.ClientConfig{
		APIKey:    apiKey,
		AgentID:   agentID,
		TenantID:  tenantID,
		BaseURL:   "http://localhost:8080",
		Timeout:   30 * time.Second,
		OnConnect: func() { fmt.Println("Connected!") },
		OnError:   func(err error) { fmt.Printf("Error: %v\n", err) },
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

	agent := &agentmsg.Agent{
		ID:   agentID,
		DID:  "did:agent:" + agentID.String(),
		Name: "example-agent",
		Capabilities: agentmsg.Capabilities{
			{
				Type:        agentmsg.CapabilityTextGeneration,
				Description: "Text generation capability",
				Examples:    []string{"generate story", "write poem"},
			},
		},
	}

	if err := client.RegisterAgent(ctx, agent); err != nil {
		log.Printf("Agent registration may require a tenant-scoped JWT with create permissions: %v", err)
	}

	agents, err := client.ListAgents(ctx)
	if err != nil {
		return fmt.Errorf("failed to list agents: %w", err)
	}
	fmt.Printf("Found %d agents\n", len(agents))

	msg := &agentmsg.Message{
		ConversationID:    uuid.New(),
		MessageType:       agentmsg.MessageTypeGeneric,
		RecipientIDs:      []uuid.UUID{agentID},
		Content:           []byte("Hello, World!"),
		ContentType:       "text/plain",
		DeliveryGuarantee: agentmsg.DeliveryAtLeastOnce,
	}

	result, err := client.SendMessage(ctx, msg)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	fmt.Printf("Message sent: %s, status: %s\n", result.MessageID, result.Status)
	return nil
}
