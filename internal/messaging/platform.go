// Package messaging defines the messaging platform abstraction for Hermes mode.
// Each platform (WeChat, Feishu, etc.) implements the Platform interface.
package messaging

import (
	"context"
	"time"
)

// Platform defines the interface that all messaging platform adapters must implement.
type Platform interface {
	// Name returns the platform identifier (e.g. "wechat", "feishu").
	Name() string
	// Start begins receiving messages. Blocks until ctx is cancelled or Stop is called.
	Start(ctx context.Context, handler MessageHandler) error
	// Stop gracefully shuts down the platform connection.
	Stop() error
	// SendMessage sends a text message to a specific chat.
	SendMessage(ctx context.Context, chatID string, text string) error
	// IsConnected reports whether the platform is currently connected.
	IsConnected() bool
}

// MessageHandler is called for each incoming message.
// It returns the response text to send back to the user.
type MessageHandler func(ctx context.Context, msg InboundMessage) (string, error)

// InboundMessage represents a message received from a messaging platform.
type InboundMessage struct {
	Platform  string    // "wechat", "feishu", etc.
	ChatID    string    // Conversation/chat identifier
	UserID    string    // Sender user ID
	UserName  string    // Sender display name
	Text      string    // Message text content
	Timestamp time.Time // When the message was sent

	// ProgressFunc is called to send intermediate progress updates during agent execution.
	// If nil, no progress updates are sent.
	ProgressFunc func(text string)
}
