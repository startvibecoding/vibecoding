// Package wechat implements the WeChat iLink Bot messaging platform adapter.
// Protocol implementation is based on the iLink Bot API specification.
// Zero external dependencies — uses only Go standard library.
package wechat

import (
	"encoding/json"
	"fmt"
	"time"
)

// --- Message types from iLink protocol ---

// MessageType indicates who sent the message.
type MessageType int

const (
	MessageTypeUser MessageType = 1
	MessageTypeBot  MessageType = 2
)

// MessageItemType indicates the content type.
type MessageItemType int

const (
	ItemText  MessageItemType = 1
	ItemImage MessageItemType = 2
	ItemVoice MessageItemType = 3
	ItemFile  MessageItemType = 4
	ItemVideo MessageItemType = 5
)

// --- Wire types (raw JSON from iLink API) ---

// WireMessage is the raw message from the iLink API.
type WireMessage struct {
	Seq          int64         `json:"seq,omitempty"`
	MessageID    int64         `json:"message_id,omitempty"`
	FromUserID   string        `json:"from_user_id"`
	ToUserID     string        `json:"to_user_id"`
	ClientID     string        `json:"client_id"`
	CreateTimeMs int64         `json:"create_time_ms"`
	MessageType  MessageType   `json:"message_type"`
	ContextToken string        `json:"context_token"`
	ItemList     []MessageItem `json:"item_list"`
}

// MessageItem is a single content item within a message.
type MessageItem struct {
	Type     MessageItemType `json:"type"`
	TextItem *TextItem       `json:"text_item,omitempty"`
}

// TextItem holds text content.
type TextItem struct {
	Text string `json:"text"`
}

// --- API response types ---

// QRCodeResponse from get_bot_qrcode.
type QRCodeResponse struct {
	QRCode       string `json:"qrcode"`
	QRCodeImgURL string `json:"qrcode_img_content"`
}

// QRStatusResponse from get_qrcode_status.
type QRStatusResponse struct {
	Status       string `json:"status"`
	BotToken     string `json:"bot_token,omitempty"`
	BotID        string `json:"ilink_bot_id,omitempty"`
	UserID       string `json:"ilink_user_id,omitempty"`
	BaseURL      string `json:"baseurl,omitempty"`
	RedirectHost string `json:"redirect_host,omitempty"`
}

// GetUpdatesResponse from getupdates.
type GetUpdatesResponse struct {
	Ret           int               `json:"ret"`
	Msgs          []json.RawMessage `json:"msgs"`
	GetUpdatesBuf string            `json:"get_updates_buf"`
	ErrCode       int               `json:"errcode,omitempty"`
	ErrMsg        string            `json:"errmsg,omitempty"`
}

// GetConfigResponse from getconfig.
type GetConfigResponse struct {
	TypingTicket string `json:"typing_ticket,omitempty"`
}

// Credentials holds login credentials.
type Credentials struct {
	Token     string `json:"token"`
	BaseURL   string `json:"baseUrl"`
	AccountID string `json:"accountId"`
	UserID    string `json:"userId"`
	SavedAt   string `json:"savedAt,omitempty"`
}

// IncomingMessage is a parsed incoming user message.
type IncomingMessage struct {
	UserID       string
	Text         string
	Timestamp    time.Time
	ContextToken string
}

// APIError is returned when the iLink API returns a non-zero ret or HTTP error.
type APIError struct {
	Message    string
	HTTPStatus int
	ErrCode    int
}

func (e *APIError) Error() string {
	return fmt.Sprintf("ilink api: %s (http=%d, errcode=%d)", e.Message, e.HTTPStatus, e.ErrCode)
}

// IsSessionExpired returns true if this error indicates session timeout.
func (e *APIError) IsSessionExpired() bool {
	return e.ErrCode == -14
}
