package api

import (
	_ "embed"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed openapi.yaml
var openAPISpec []byte

func (s *Server) openAPISpec(c *gin.Context) {
	c.Data(http.StatusOK, "application/yaml", openAPISpec)
}
