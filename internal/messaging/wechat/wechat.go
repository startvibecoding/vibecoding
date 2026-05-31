package wechat

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/startvibecoding/vibecoding/internal/messaging"
)

// Bot implements messaging.Platform for WeChat via the iLink protocol.
type Bot struct {
	client        *Client
	creds         *Credentials
	credPath      string
	autoTyping    bool
	connected     bool
	stopped       bool
	mu            sync.Mutex
	cancelPoll    context.CancelFunc
	contextTokens sync.Map // map[userID]contextToken
	cursor        string
}

// BotOptions configures a WeChat Bot.
type BotOptions struct {
	CredPath   string
	AutoTyping bool
}

// NewBot creates a new WeChat bot.
func NewBot(opts BotOptions) *Bot {
	return &Bot{
		client:     NewClient(),
		credPath:   opts.CredPath,
		autoTyping: opts.AutoTyping,
	}
}

// --- messaging.Platform implementation ---

func (b *Bot) Name() string { return "wechat" }

func (b *Bot) IsConnected() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.connected
}

// Start begins long-poll message receiving. Blocks until ctx is cancelled.
func (b *Bot) Start(ctx context.Context, handler messaging.MessageHandler) error {
	// Load credentials
	creds, err := LoadCredentials(b.credPath)
	if err != nil || creds == nil {
		return fmt.Errorf("wechat: no credentials found at %s — run 'vibecoding hermes wechat login' first", b.credPath)
	}

	b.mu.Lock()
	b.creds = creds
	b.connected = true
	b.stopped = false
	pollCtx, cancel := context.WithCancel(ctx)
	b.cancelPoll = cancel
	b.mu.Unlock()

	log.Printf("[wechat] Long-poll loop started (user: %s)", creds.UserID)
	retryDelay := time.Second

	for {
		select {
		case <-pollCtx.Done():
			b.mu.Lock()
			b.connected = false
			b.mu.Unlock()
			log.Printf("[wechat] Long-poll loop stopped")
			return nil
		default:
		}

		b.mu.Lock()
		currentCreds := b.creds
		b.mu.Unlock()

		updates, err := b.client.GetUpdates(pollCtx, currentCreds.BaseURL, currentCreds.Token, b.cursor)
		if err != nil {
			if pollCtx.Err() != nil {
				return nil
			}

			apiErr, isAPI := err.(*APIError)
			if isAPI && apiErr.IsSessionExpired() {
				log.Printf("[wechat] Session expired — re-login required")
				ClearCredentials(b.credPath)
				b.contextTokens = sync.Map{}
				b.cursor = ""
				// Try re-login
				newCreds, loginErr := Login(pollCtx, b.client, LoginOptions{
					CredPath: b.credPath,
					Force:    true,
				})
				if loginErr != nil {
					log.Printf("[wechat] Re-login failed: %v", loginErr)
					time.Sleep(retryDelay)
					continue
				}
				b.mu.Lock()
				b.creds = newCreds
				b.mu.Unlock()
				retryDelay = time.Second
				continue
			}

			log.Printf("[wechat] Poll error: %v", err)
			time.Sleep(retryDelay)
			if retryDelay < 10*time.Second {
				retryDelay *= 2
			}
			continue
		}

		if updates.GetUpdatesBuf != "" {
			b.cursor = updates.GetUpdatesBuf
		}
		retryDelay = time.Second

		for _, rawMsg := range updates.Msgs {
			var wire WireMessage
			if err := json.Unmarshal(rawMsg, &wire); err != nil {
				continue
			}

			// Remember context tokens
			b.rememberContext(&wire)

			// Only process user messages
			if wire.MessageType != MessageTypeUser {
				continue
			}

			text := extractText(wire.ItemList)
			if text == "" {
				continue
			}

			msg := messaging.InboundMessage{
				Platform:  "wechat",
				ChatID:    wire.FromUserID,
				UserID:    wire.FromUserID,
				Text:      text,
				Timestamp: time.UnixMilli(wire.CreateTimeMs),
			}

			// Show typing indicator
			if b.autoTyping {
				go b.sendTyping(pollCtx, wire.FromUserID)
			}

			// Handle message
			go func(m messaging.InboundMessage, ct string) {
				// Create progress buffer: max 7 progress lines per batch, reserve 3 for summary
				progressBuf := messaging.NewProgressBuffer(7, func(text string) {
					if err := b.SendMessage(pollCtx, wire.FromUserID, text); err != nil {
						log.Printf("[wechat] Progress send error: %v", err)
					}
				})
				m.ProgressFunc = func(text string) {
					progressBuf.Add(text)
				}

				response, err := handler(pollCtx, m)

				// Flush remaining progress lines before final summary
				progressBuf.Flush()

				if err != nil {
					log.Printf("[wechat] Handler error for %s: %v", m.UserID, err)
					response = "⚠️ Error: " + err.Error()
				}
				if response != "" {
					if sendErr := b.sendText(pollCtx, m.UserID, response, ct); sendErr != nil {
						log.Printf("[wechat] Send error for %s: %v", m.UserID, sendErr)
					} else {
						log.Printf("[wechat] Message sent to %s successfully (len=%d)", m.UserID, len(response))
					}
				} else {
					log.Printf("[wechat] Empty response for %s, not sending", m.UserID)
				}
				// Stop typing
				if b.autoTyping {
					b.stopTyping(pollCtx, m.UserID)
				}
			}(msg, wire.ContextToken)
		}
	}
}

// Stop gracefully stops the bot.
func (b *Bot) Stop() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.stopped = true
	if b.cancelPoll != nil {
		b.cancelPoll()
	}
	return nil
}

// SendMessage sends a text message to a user.
func (b *Bot) SendMessage(ctx context.Context, chatID string, text string) error {
	ct, ok := b.contextTokens.Load(chatID)
	if !ok {
		return fmt.Errorf("no context_token for user %s", chatID)
	}
	return b.sendText(ctx, chatID, text, ct.(string))
}

// --- Internal ---

func (b *Bot) sendText(ctx context.Context, userID, text, contextToken string) error {
	b.mu.Lock()
	creds := b.creds
	b.mu.Unlock()

	if creds == nil {
		return fmt.Errorf("not logged in")
	}

	chunks := chunkText(text, 4000)
	for _, chunk := range chunks {
		msg := BuildTextMessage(creds.UserID, userID, contextToken, chunk)
		if err := b.client.SendMessage(ctx, creds.BaseURL, creds.Token, msg); err != nil {
			return err
		}
	}
	return nil
}

func (b *Bot) sendTyping(ctx context.Context, userID string) {
	ct, ok := b.contextTokens.Load(userID)
	if !ok {
		return
	}
	b.mu.Lock()
	creds := b.creds
	b.mu.Unlock()
	if creds == nil {
		return
	}
	config, err := b.client.GetConfig(ctx, creds.BaseURL, creds.Token, userID, ct.(string))
	if err != nil || config.TypingTicket == "" {
		return
	}
	b.client.SendTyping(ctx, creds.BaseURL, creds.Token, userID, config.TypingTicket, 1)
}

func (b *Bot) stopTyping(ctx context.Context, userID string) {
	ct, ok := b.contextTokens.Load(userID)
	if !ok {
		return
	}
	b.mu.Lock()
	creds := b.creds
	b.mu.Unlock()
	if creds == nil {
		return
	}
	config, err := b.client.GetConfig(ctx, creds.BaseURL, creds.Token, userID, ct.(string))
	if err != nil || config.TypingTicket == "" {
		return
	}
	b.client.SendTyping(ctx, creds.BaseURL, creds.Token, userID, config.TypingTicket, 2)
}

func (b *Bot) rememberContext(wire *WireMessage) {
	userID := wire.FromUserID
	if wire.MessageType == MessageTypeBot {
		userID = wire.ToUserID
	}
	if userID != "" && wire.ContextToken != "" {
		b.contextTokens.Store(userID, wire.ContextToken)
	}
}

func extractText(items []MessageItem) string {
	var parts []string
	for _, item := range items {
		if item.Type == ItemText && item.TextItem != nil {
			parts = append(parts, item.TextItem.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func chunkText(text string, limit int) []string {
	if len(text) <= limit {
		return []string{text}
	}
	var chunks []string
	for len(text) > 0 {
		if len(text) <= limit {
			chunks = append(chunks, text)
			break
		}
		cut := limit
		if idx := strings.LastIndex(text[:limit], "\n\n"); idx > limit*3/10 {
			cut = idx + 2
		} else if idx := strings.LastIndex(text[:limit], "\n"); idx > limit*3/10 {
			cut = idx + 1
		}
		chunks = append(chunks, text[:cut])
		text = text[cut:]
	}
	return chunks
}

// Ensure Bot implements messaging.Platform at compile time.
var _ messaging.Platform = (*Bot)(nil)
