package config

import (
	"strings"
	"testing"
	"time"
)

func TestValidateRequiresJWTSecretForAPI(t *testing.T) {
	cfg := &Config{
		JWTSecret:         "",
		RateLimitRequests: 100,
		RateLimitWindow:   time.Minute,
	}

	err := cfg.Validate(true)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "JWT_SECRET is required") {
		t.Fatalf("expected JWT_SECRET error, got %v", err)
	}
}

func TestValidateRejectsProductionDefaultJWTSecret(t *testing.T) {
	cfg := &Config{
		JWTSecret:         "dev-secret",
		Env:               "production",
		RateLimitRequests: 100,
		RateLimitWindow:   time.Minute,
	}

	err := cfg.Validate(true)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "development defaults") {
		t.Fatalf("expected insecure JWT secret error, got %v", err)
	}
}

func TestValidateRequiresOTLPEndpointWhenTracingEnabled(t *testing.T) {
	cfg := &Config{
		JWTSecret:         "super-secret",
		OTELEnabled:       true,
		RateLimitRequests: 100,
		RateLimitWindow:   time.Minute,
	}

	err := cfg.Validate(true)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "OTEL_EXPORTER_OTLP_ENDPOINT") {
		t.Fatalf("expected OTEL endpoint error, got %v", err)
	}
}

func TestValidateAcceptsProductionSafeConfig(t *testing.T) {
	cfg := &Config{
		JWTSecret:                "prod-secret-value",
		Env:                      "production",
		OTELEnabled:              true,
		OTELExporterOTLPEndpoint: "http://otel-collector:4318",
		RateLimitRequests:        100,
		RateLimitWindow:          time.Minute,
	}

	if err := cfg.Validate(true); err != nil {
		t.Fatalf("expected config to be valid, got %v", err)
	}
}
