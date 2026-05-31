package wechat

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	DefaultBaseURL = "https://ilinkai.weixin.qq.com"
	CDNBaseURL     = "https://novac2c.cdn.weixin.qq.com/c2c"
	ChannelVersion = "0.1.0"
	iLinkAppID     = "bot"
	iLinkClientVer = "256"
)

// Client wraps HTTP calls to the iLink API.
type Client struct {
	HTTP *http.Client
}

// NewClient creates a protocol client.
func NewClient() *Client {
	return &Client{
		HTTP: &http.Client{Timeout: 45 * time.Second},
	}
}

// CommonHeaders returns headers for iLink API requests.
func CommonHeaders() http.Header {
	h := http.Header{}
	h.Set("iLink-App-Id", iLinkAppID)
	h.Set("iLink-App-ClientVersion", iLinkClientVer)
	return h
}

// AuthHeaders returns the standard iLink POST headers.
func AuthHeaders(token string) http.Header {
	h := CommonHeaders()
	h.Set("Content-Type", "application/json")
	h.Set("AuthorizationType", "ilink_bot_token")
	h.Set("Authorization", "Bearer "+token)
	h.Set("X-WECHAT-UIN", randomWechatUIN())
	return h
}

func randomWechatUIN() string {
	var buf [4]byte
	rand.Read(buf[:])
	val := binary.BigEndian.Uint32(buf[:])
	return base64.StdEncoding.EncodeToString([]byte(strconv.FormatUint(uint64(val), 10)))
}

func baseInfo() map[string]string {
	return map[string]string{"channel_version": ChannelVersion}
}

// GetQRCode requests a new QR code for login.
func (c *Client) GetQRCode(ctx context.Context, baseURL string) (*QRCodeResponse, error) {
	u := baseURL + "/ilink/bot/get_bot_qrcode?bot_type=3"
	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
	for k, v := range CommonHeaders() {
		req.Header[k] = v
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get_bot_qrcode: %w", err)
	}
	defer resp.Body.Close()
	var result QRCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("get_bot_qrcode decode: %w", err)
	}
	return &result, nil
}

// PollQRStatus polls the QR code scan status.
func (c *Client) PollQRStatus(ctx context.Context, baseURL, qrcode string) (*QRStatusResponse, error) {
	u := baseURL + "/ilink/bot/get_qrcode_status?qrcode=" + url.QueryEscape(qrcode)
	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
	for k, v := range CommonHeaders() {
		req.Header[k] = v
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result QRStatusResponse
	json.NewDecoder(resp.Body).Decode(&result)
	return &result, nil
}

// apiPost sends a POST to the iLink API and parses the response.
func (c *Client) apiPost(ctx context.Context, baseURL, endpoint, token string, body interface{}, timeout time.Duration) (json.RawMessage, error) {
	data, _ := json.Marshal(body)
	u := baseURL + endpoint
	httpCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, _ := http.NewRequestWithContext(httpCtx, "POST", u, bytes.NewReader(data))
	for k, v := range AuthHeaders(token) {
		req.Header[k] = v
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, &APIError{Message: string(raw), HTTPStatus: resp.StatusCode}
	}

	var check struct {
		Ret     int    `json:"ret"`
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	json.Unmarshal(raw, &check)
	if check.Ret != 0 || check.ErrCode != 0 {
		code := check.ErrCode
		if code == 0 {
			code = check.Ret
		}
		msg := check.ErrMsg
		if msg == "" {
			msg = fmt.Sprintf("ret=%d", check.Ret)
		}
		return nil, &APIError{Message: msg, HTTPStatus: resp.StatusCode, ErrCode: code}
	}

	return json.RawMessage(raw), nil
}

// GetUpdates performs a long-poll for new messages.
func (c *Client) GetUpdates(ctx context.Context, baseURL, token, cursor string) (*GetUpdatesResponse, error) {
	body := map[string]interface{}{
		"get_updates_buf": cursor,
		"base_info":       baseInfo(),
	}
	raw, err := c.apiPost(ctx, baseURL, "/ilink/bot/getupdates", token, body, 45*time.Second)
	if err != nil {
		return nil, err
	}
	var result GetUpdatesResponse
	json.Unmarshal(raw, &result)
	return &result, nil
}

// SendMessage sends a message through the iLink API.
func (c *Client) SendMessage(ctx context.Context, baseURL, token string, msg interface{}) error {
	body := map[string]interface{}{
		"msg":       msg,
		"base_info": baseInfo(),
	}
	_, err := c.apiPost(ctx, baseURL, "/ilink/bot/sendmessage", token, body, 15*time.Second)
	return err
}

// GetConfig gets the typing ticket for a user.
func (c *Client) GetConfig(ctx context.Context, baseURL, token, userID, contextToken string) (*GetConfigResponse, error) {
	body := map[string]interface{}{
		"ilink_user_id": userID,
		"context_token": contextToken,
		"base_info":     baseInfo(),
	}
	raw, err := c.apiPost(ctx, baseURL, "/ilink/bot/getconfig", token, body, 15*time.Second)
	if err != nil {
		return nil, err
	}
	var result GetConfigResponse
	json.Unmarshal(raw, &result)
	return &result, nil
}

// SendTyping sends or cancels the typing indicator.
func (c *Client) SendTyping(ctx context.Context, baseURL, token, userID, ticket string, status int) error {
	body := map[string]interface{}{
		"ilink_user_id": userID,
		"typing_ticket": ticket,
		"status":        status,
		"base_info":     baseInfo(),
	}
	_, err := c.apiPost(ctx, baseURL, "/ilink/bot/sendtyping", token, body, 15*time.Second)
	return err
}

// BuildTextMessage creates a text message payload.
func BuildTextMessage(fromUserID, toUserID, contextToken, text string) map[string]interface{} {
	return map[string]interface{}{
		"from_user_id":  fromUserID,
		"to_user_id":    toUserID,
		"client_id":     newUUID(),
		"message_type":  2,
		"message_state": 2,
		"context_token": contextToken,
		"item_list": []map[string]interface{}{
			{"type": 1, "text_item": map[string]string{"text": text}},
		},
	}
}

func newUUID() string {
	var buf [16]byte
	rand.Read(buf[:])
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		buf[0:4], buf[4:6], buf[6:8], buf[8:10], buf[10:16])
}
