// Package feishu implements the Feishu (Lark) messaging platform adapter.
// Uses the official Feishu Go SDK with WebSocket long connection for receiving messages.
package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"

	"github.com/startvibecoding/vibecoding/internal/messaging"
)

// Bot implements messaging.Platform for Feishu via official SDK WebSocket.
type Bot struct {
	appID     string
	appSecret string
	client    *lark.Client
	wsClient  *larkws.Client
	handler   messaging.MessageHandler
	connected bool
	mu        sync.Mutex
	cancel    context.CancelFunc
}

// BotOptions configures a Feishu Bot.
type BotOptions struct {
	AppID     string
	AppSecret string
}

// NewBot creates a new Feishu bot.
func NewBot(opts BotOptions) *Bot {
	client := lark.NewClient(opts.AppID, opts.AppSecret)
	return &Bot{
		appID:     opts.AppID,
		appSecret: opts.AppSecret,
		client:    client,
	}
}

// --- messaging.Platform implementation ---

func (b *Bot) Name() string { return "feishu" }

func (b *Bot) IsConnected() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.connected
}

// Start begins receiving messages via WebSocket long connection.
func (b *Bot) Start(ctx context.Context, handler messaging.MessageHandler) error {
	b.mu.Lock()
	b.handler = handler
	ctx, cancel := context.WithCancel(ctx)
	b.cancel = cancel
	b.mu.Unlock()

	// Create event dispatcher
	eventDispatcher := dispatcher.NewEventDispatcher("", "").
		OnP2MessageReceiveV1(b.onMessage)

	// Create WebSocket client
	b.wsClient = larkws.NewClient(b.appID, b.appSecret,
		larkws.WithEventHandler(eventDispatcher),
		larkws.WithLogLevel(larkcore.LogLevelInfo),
	)

	b.mu.Lock()
	b.connected = true
	b.mu.Unlock()

	log.Printf("[feishu] WebSocket long connection started")

	// Start blocks until connection drops or context cancelled
	err := b.wsClient.Start(ctx)

	b.mu.Lock()
	b.connected = false
	b.mu.Unlock()

	if ctx.Err() != nil {
		return nil // normal shutdown
	}
	return err
}

// Stop gracefully shuts down the bot.
func (b *Bot) Stop() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.cancel != nil {
		b.cancel()
	}
	b.connected = false
	return nil
}

// SendMessage sends a text message to a chat.
func (b *Bot) SendMessage(ctx context.Context, chatID string, text string) error {
	content, _ := json.Marshal(map[string]string{"text": text})
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType("chat_id").
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(chatID).
			MsgType("text").
			Content(string(content)).
			Build()).
		Build()

	resp, err := b.client.Im.Message.Create(ctx, req)
	if err != nil {
		return fmt.Errorf("feishu send message: %w", err)
	}
	if !resp.Success() {
		return fmt.Errorf("feishu send message: code=%d msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

// --- Event handler ---

func (b *Bot) onMessage(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
	b.mu.Lock()
	handler := b.handler
	b.mu.Unlock()

	if handler == nil {
		return nil
	}

	msg := event.Event.Message
	sender := event.Event.Sender

	// Only handle text messages
	if msg == nil || sender == nil {
		return nil
	}

	msgType := ""
	if msg.MessageType != nil {
		msgType = *msg.MessageType
	}
	if msgType != "text" {
		log.Printf("[feishu] Ignoring non-text message type: %s", msgType)
		return nil
	}

	// Parse text content
	var textContent struct {
		Text string `json:"text"`
	}
	if msg.Content != nil {
		json.Unmarshal([]byte(*msg.Content), &textContent)
	}
	if textContent.Text == "" {
		return nil
	}

	// Extract user info
	userID := ""
	if sender.SenderId != nil && sender.SenderId.OpenId != nil {
		userID = *sender.SenderId.OpenId
	}

	chatID := ""
	if msg.ChatId != nil {
		chatID = *msg.ChatId
	}

	inbound := messaging.InboundMessage{
		Platform: "feishu",
		ChatID:   chatID,
		UserID:   userID,
		Text:     textContent.Text,
	}

	// Handle message asynchronously
	go func() {
		// Create progress buffer: max 7 progress lines per batch, reserve 3 for summary
		progressBuf := messaging.NewProgressBuffer(7, func(text string) {
			if err := b.SendMessage(context.Background(), chatID, text); err != nil {
				log.Printf("[feishu] Progress send error: %v", err)
			}
		})
		inbound.ProgressFunc = func(text string) {
			progressBuf.Add(text)
		}

		response, err := handler(context.Background(), inbound)

		// Flush remaining progress lines before final summary
		progressBuf.Flush()

		if err != nil {
			log.Printf("[feishu] Handler error for %s: %v", userID, err)
			response = "⚠️ Error: " + err.Error()
		}
		if response != "" {
			// Reply in the same chat
			replyID := ""
			if msg.MessageId != nil {
				replyID = *msg.MessageId
			}
			if replyErr := b.replyMessage(context.Background(), replyID, chatID, response); replyErr != nil {
				log.Printf("[feishu] Reply error: %v", replyErr)
			}
		}
	}()

	return nil
}

// replyMessage replies to a message or sends to chat.
func (b *Bot) replyMessage(ctx context.Context, messageID, chatID, text string) error {
	content, _ := json.Marshal(map[string]string{"text": text})

	if messageID != "" {
		// Reply to specific message
		req := larkim.NewReplyMessageReqBuilder().
			MessageId(messageID).
			Body(larkim.NewReplyMessageReqBodyBuilder().
				MsgType("text").
				Content(string(content)).
				Build()).
			Build()

		resp, err := b.client.Im.Message.Reply(ctx, req)
		if err != nil {
			return err
		}
		if !resp.Success() {
			return fmt.Errorf("code=%d msg=%s", resp.Code, resp.Msg)
		}
		return nil
	}

	// Send to chat directly
	return b.SendMessage(ctx, chatID, text)
}

// Ensure Bot implements messaging.Platform at compile time.
var _ messaging.Platform = (*Bot)(nil)
