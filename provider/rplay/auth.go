package rplay

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"streamwatcher/config"
)

const (
	loginURL = "https://api.rplay.live/rplay/account/login"
)

// Auth handles RPLAY self-signed JWT authentication.
type Auth struct {
	Email  string
	Hash   string // hex-encoded SHA256 of the password
	UserID string
	client *http.Client
}

// NewAuth creates an Auth from the current configuration. If user_id is empty, it
// attempts to resolve it via the login endpoint.
func NewAuth() (*Auth, error) {
	cfg := config.Get().RPLAY
	if cfg.Email == "" || cfg.Password == "" {
		return nil, fmt.Errorf("rplay email and password are required")
	}

	hash := sha256Hex(cfg.Password)
	auth := &Auth{
		Email:  cfg.Email,
		Hash:   hash,
		UserID: cfg.UserID,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}

	if auth.UserID == "" {
		if err := auth.resolveUserID(); err != nil {
			slog.Warn("rplay: failed to resolve user_id", "error", err)
		}
	}

	return auth, nil
}

// Headers returns a fresh set of authenticated HTTP headers.
func (a *Auth) Headers() map[string]string {
	return map[string]string{
		"accept":             "application/json, text/plain, */*",
		"accept-language":    "en;q=0.8,en-US;q=0.7",
		"authorization":      a.generateToken(),
		"origin":             "https://rplay.live",
		"referer":            "https://rplay.live/",
		"sec-ch-ua":          `"Not.A/Brand";v="8", "Chromium";v="114", "Google Chrome";v="114"`,
		"sec-ch-ua-mobile":   "?0",
		"sec-ch-ua-platform": `"Windows"`,
		"sec-fetch-dest":     "empty",
		"sec-fetch-mode":     "cors",
		"sec-fetch-site":     "same-site",
		"user-agent":         "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36",
	}
}

func (a *Auth) generateToken() string {
	header := base64RawURLEncode([]byte(`{"alg":"HS256","typ":"JWT"}`))

	now := time.Now()
	dat := now.UTC().Format("2006-01-02T15:04:05.000") + "Z"
	payloadMap := map[string]any{
		"eml": a.Email,
		"dat": dat,
		"iat": now.Unix(),
	}
	payloadBytes, _ := json.Marshal(payloadMap)
	payload := base64RawURLEncode(payloadBytes)

	signingInput := header + "." + payload
	signature := hmacSHA256Base64RawURL(signingInput, a.Hash)

	return signingInput + "." + signature
}

func (a *Auth) resolveUserID() error {
	body, err := json.Marshal(map[string]any{
		"accountType":  "plax",
		"checkAdmin":   nil,
		"email":        a.Email,
		"lang":         "en",
		"loginType":    nil,
		"password":     config.Get().RPLAY.Password,
		"platformType": "rplay",
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, loginURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Referer", "https://rplay.live/")

	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("login endpoint returned %d", resp.StatusCode)
	}

	var result struct {
		User struct {
			ID string `json:"_id"`
		} `json:"user"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if result.User.ID == "" {
		return fmt.Errorf("no user_id in login response")
	}

	a.UserID = result.User.ID
	slog.Info("rplay: resolved user_id", "user_id", a.UserID)
	return nil
}

func sha256Hex(s string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(s)))
}

func base64RawURLEncode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

func hmacSHA256Base64RawURL(message, hexKey string) string {
	key := []byte(hexKey)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(message))
	return base64RawURLEncode(mac.Sum(nil))
}
