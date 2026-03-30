package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"agentmsg/internal/middleware"
	"agentmsg/internal/service"
)

func TestGetMessageRejectsInvalidUUID(t *testing.T) {
	authService := service.NewAuthService("test-secret")
	server := NewServer(&ServerConfig{
		Addr:         "127.0.0.1:0",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  10 * time.Second,
	}, &Dependencies{
		AuthService: authService,
		Middleware:  middleware.NewMiddleware(nil, nil, authService, 100, time.Minute),
	})

	httpServer := httptest.NewServer(server.httpServer.Handler)
	defer httpServer.Close()

	token, err := authService.GenerateToken(mustUUID(t, "11111111-1111-1111-1111-111111111111"), mustUUID(t, "22222222-2222-2222-2222-222222222222"))
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodGet, httpServer.URL+"/api/v1/messages/not-a-uuid", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var payload apiErrorEnvelope
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
	require.Equal(t, "invalid_uuid", payload.Error.Code)
}

func mustUUID(t *testing.T, value string) uuid.UUID {
	t.Helper()
	parsed, err := uuid.Parse(value)
	require.NoError(t, err)
	return parsed
}
