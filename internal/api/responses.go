package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"agentmsg/internal/service"
)

type apiErrorEnvelope struct {
	Error     apiError `json:"error"`
	RequestID string   `json:"requestId,omitempty"`
	TraceID   string   `json:"traceId,omitempty"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func respondError(c *gin.Context, status int, code, message string) {
	c.JSON(status, apiErrorEnvelope{
		Error: apiError{
			Code:    code,
			Message: message,
		},
		RequestID: c.GetString("request_id"),
		TraceID:   c.GetString("trace_id"),
	})
}

func respondServiceError(c *gin.Context, err error, notFoundCode string) {
	switch {
	case errors.Is(err, service.ErrAgentNotFound), errors.Is(err, service.ErrMessageNotFound):
		respondError(c, http.StatusNotFound, notFoundCode, err.Error())
	case errors.Is(err, service.ErrAgentAlreadyExists):
		respondError(c, http.StatusConflict, "agent_conflict", err.Error())
	case errors.Is(err, service.ErrInvalidMessage), errors.Is(err, service.ErrInvalidToken):
		respondError(c, http.StatusBadRequest, "invalid_request", err.Error())
	case errors.Is(err, service.ErrUnauthorized):
		respondError(c, http.StatusForbidden, "forbidden", err.Error())
	case errors.Is(err, service.ErrServiceUnavailable):
		respondError(c, http.StatusServiceUnavailable, "service_unavailable", err.Error())
	default:
		respondError(c, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func parseUUIDParam(c *gin.Context, paramName string) (uuid.UUID, bool) {
	value := c.Param(paramName)
	parsed, err := uuid.Parse(value)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid_uuid", "invalid "+paramName)
		return uuid.Nil, false
	}
	return parsed, true
}

func parseUUIDValue(c *gin.Context, value, field string) (uuid.UUID, bool) {
	parsed, err := uuid.Parse(value)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid_uuid", "invalid "+field)
		return uuid.Nil, false
	}
	return parsed, true
}

func contextUUID(c *gin.Context, key string) (uuid.UUID, bool) {
	value := c.GetString(key)
	if value == "" {
		respondError(c, http.StatusUnauthorized, "missing_identity", "missing "+key)
		return uuid.Nil, false
	}
	return parseUUIDValue(c, value, key)
}
