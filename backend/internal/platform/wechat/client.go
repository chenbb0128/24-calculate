package wechat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/example/go-service/internal/config"
)

var (
	ErrNotConfigured = errors.New("wechat client is not configured")
	ErrInvalidCode   = errors.New("wechat login code is invalid")
)

type LoginResult struct {
	OpenID  string
	UnionID string
}

type LoginClient interface {
	ExchangeCode(ctx context.Context, code string) (LoginResult, error)
}

type Client struct {
	appID      string
	appSecret  string
	apiBaseURL string
	httpClient *http.Client
}

func NewClient(cfg config.WeChatConfig) *Client {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &Client{
		appID:      strings.TrimSpace(cfg.AppID),
		appSecret:  strings.TrimSpace(cfg.AppSecret),
		apiBaseURL: strings.TrimRight(strings.TrimSpace(cfg.APIBaseURL), "/"),
		httpClient: &http.Client{Timeout: timeout},
	}
}

type code2SessionResponse struct {
	OpenID     string `json:"openid"`
	UnionID    string `json:"unionid"`
	ErrCode    int    `json:"errcode"`
	ErrMessage string `json:"errmsg"`
}

func (c *Client) ExchangeCode(ctx context.Context, code string) (LoginResult, error) {
	if c == nil || c.httpClient == nil || c.appID == "" || c.appSecret == "" || c.apiBaseURL == "" {
		return LoginResult{}, ErrNotConfigured
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return LoginResult{}, ErrInvalidCode
	}

	endpoint, err := url.Parse(c.apiBaseURL + "/sns/jscode2session")
	if err != nil {
		return LoginResult{}, fmt.Errorf("parse wechat endpoint: %w", err)
	}
	query := endpoint.Query()
	query.Set("appid", c.appID)
	query.Set("secret", c.appSecret)
	query.Set("js_code", code)
	query.Set("grant_type", "authorization_code")
	endpoint.RawQuery = query.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return LoginResult{}, fmt.Errorf("create wechat request: %w", err)
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return LoginResult{}, fmt.Errorf("request wechat login: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return LoginResult{}, fmt.Errorf("wechat login returned status %d", response.StatusCode)
	}
	var payload code2SessionResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return LoginResult{}, fmt.Errorf("decode wechat login response: %w", err)
	}
	if payload.ErrCode != 0 || strings.TrimSpace(payload.OpenID) == "" {
		return LoginResult{}, fmt.Errorf("%w: code=%d message=%s", ErrInvalidCode, payload.ErrCode, payload.ErrMessage)
	}
	return LoginResult{OpenID: strings.TrimSpace(payload.OpenID), UnionID: strings.TrimSpace(payload.UnionID)}, nil
}
