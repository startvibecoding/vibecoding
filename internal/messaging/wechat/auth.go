package wechat

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	maxQRRefreshCount = 3
	fixedQRBaseURL    = "https://ilinkai.weixin.qq.com"
)

// LoadCredentials loads stored credentials from disk.
func LoadCredentials(path string) (*Credentials, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, err
	}
	return &creds, nil
}

// SaveCredentials persists credentials to disk.
func SaveCredentials(creds *Credentials, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(creds, "", "  ")
	return os.WriteFile(path, append(data, '\n'), 0600)
}

// ClearCredentials removes stored credentials.
func ClearCredentials(path string) error {
	return os.Remove(path)
}

// LoginOptions configures the login flow.
type LoginOptions struct {
	BaseURL   string
	CredPath  string
	Force     bool
	OnQRURL   func(url string)
	OnScanned func()
	OnExpired func()
}

// Login performs QR code login, returning credentials.
// If stored credentials exist and Force is false, returns them directly.
func Login(ctx context.Context, client *Client, opts LoginOptions) (*Credentials, error) {
	baseURL := opts.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	if !opts.Force {
		creds, err := LoadCredentials(opts.CredPath)
		if err == nil && creds != nil {
			return creds, nil
		}
	}

	qrRefreshCount := 0
	for {
		qrRefreshCount++
		if qrRefreshCount > maxQRRefreshCount {
			return nil, fmt.Errorf("QR code expired %d times — login aborted", maxQRRefreshCount)
		}

		qr, err := client.GetQRCode(ctx, fixedQRBaseURL)
		if err != nil {
			return nil, fmt.Errorf("get QR code: %w", err)
		}

		if opts.OnQRURL != nil {
			opts.OnQRURL(qr.QRCodeImgURL)
		} else {
			fmt.Fprintf(os.Stderr, "Scan this URL in WeChat: %s\n", qr.QRCodeImgURL)
		}

		lastStatus := ""
		currentPollBaseURL := fixedQRBaseURL
		for {
			status, err := client.PollQRStatus(ctx, currentPollBaseURL, qr.QRCode)
			if err != nil {
				return nil, fmt.Errorf("poll QR status: %w", err)
			}

			if status.Status != lastStatus {
				lastStatus = status.Status
				switch status.Status {
				case "scaned":
					if opts.OnScanned != nil {
						opts.OnScanned()
					} else {
						fmt.Fprintln(os.Stderr, "QR scanned — confirm in WeChat")
					}
				case "expired":
					if opts.OnExpired != nil {
						opts.OnExpired()
					} else {
						fmt.Fprintln(os.Stderr, "QR expired — requesting new one")
					}
				case "confirmed":
					fmt.Fprintln(os.Stderr, "Login confirmed")
				}
			}

			if status.Status == "confirmed" {
				if status.BotToken == "" || status.BotID == "" || status.UserID == "" {
					return nil, fmt.Errorf("login confirmed but missing credentials")
				}
				resolvedBase := baseURL
				if status.BaseURL != "" {
					resolvedBase = status.BaseURL
				}
				creds := &Credentials{
					Token:     status.BotToken,
					BaseURL:   resolvedBase,
					AccountID: status.BotID,
					UserID:    status.UserID,
					SavedAt:   time.Now().UTC().Format(time.RFC3339),
				}
				if err := SaveCredentials(creds, opts.CredPath); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: could not save credentials: %v\n", err)
				}
				return creds, nil
			}

			if status.Status == "scaned_but_redirect" {
				if status.RedirectHost != "" {
					currentPollBaseURL = "https://" + status.RedirectHost
					fmt.Fprintf(os.Stderr, "IDC redirect → %s\n", status.RedirectHost)
				}
				time.Sleep(2 * time.Second)
				continue
			}

			if status.Status == "expired" {
				break
			}

			time.Sleep(2 * time.Second)
		}
	}
}
