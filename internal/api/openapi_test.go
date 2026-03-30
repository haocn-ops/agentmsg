package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"agentmsg/internal/middleware"
	"agentmsg/internal/service"
)

func TestOpenAPISpecServed(t *testing.T) {
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

	resp, err := http.Get(httpServer.URL + "/openapi.yaml")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "application/yaml", resp.Header.Get("Content-Type"))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "openapi: 3.1.0")
	require.Contains(t, string(body), "/api/v1/messages")
}
