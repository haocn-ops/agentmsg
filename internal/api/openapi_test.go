package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

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

func TestOpenAPISpecMatchesRegisteredRoutes(t *testing.T) {
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

	engine, ok := server.httpServer.Handler.(*gin.Engine)
	require.True(t, ok, "server handler must be a gin engine")

	var spec struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	require.NoError(t, yaml.Unmarshal(openAPISpec, &spec))

	documentedRoutes := make(map[string]struct{})
	for path, operations := range spec.Paths {
		for method := range operations {
			upperMethod := strings.ToUpper(method)
			if upperMethod == "PARAMETERS" {
				continue
			}
			documentedRoutes[routeKey(upperMethod, path)] = struct{}{}
		}
	}

	registeredRoutes := make(map[string]struct{})
	for _, route := range engine.Routes() {
		if route.Method == http.MethodHead || route.Method == http.MethodOptions {
			continue
		}
		registeredRoutes[routeKey(route.Method, normalizeRoutePath(route.Path))] = struct{}{}
	}

	var undocumented []string
	for route := range registeredRoutes {
		if _, ok := documentedRoutes[route]; !ok {
			undocumented = append(undocumented, route)
		}
	}

	var stale []string
	for route := range documentedRoutes {
		if _, ok := registeredRoutes[route]; !ok {
			stale = append(stale, route)
		}
	}

	sort.Strings(undocumented)
	sort.Strings(stale)

	require.Empty(t, undocumented, "registered routes missing from OpenAPI spec: %v", undocumented)
	require.Empty(t, stale, "OpenAPI routes missing from registered handlers: %v", stale)
}

func normalizeRoutePath(path string) string {
	return strings.ReplaceAll(path, ":id", "{id}")
}

func routeKey(method, path string) string {
	return method + " " + path
}
