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
	if err := runCapabilityDiscoveryExample(); err != nil {
		log.Fatal(err)
	}
}

func runCapabilityDiscoveryExample() error {
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

	capabilities := []string{"text-generation", "code-generation"}

	agents, err := client.QueryCapabilities(ctx, capabilities)
	if err != nil {
		return fmt.Errorf("failed to query capabilities: %w", err)
	}

	fmt.Printf("Found %d agents with requested capabilities:\n", len(agents))
	for _, agent := range agents {
		fmt.Printf("  - %s (%s)\n", agent.Name, agent.DID)
		for _, cap := range agent.Capabilities {
			fmt.Printf("    Capability: %s - %s\n", cap.Type, cap.Description)
		}
	}

	return nil
}
