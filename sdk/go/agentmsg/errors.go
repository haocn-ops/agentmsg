package agentmsg

import "errors"

var (
	ErrAgentNotFound    = errors.New("agent not found")
	ErrMessageNotFound  = errors.New("message not found")
	ErrConnectionFailed = errors.New("connection failed")
	ErrTimeout          = errors.New("operation timeout")
	ErrInvalidConfig    = errors.New("invalid configuration")
)