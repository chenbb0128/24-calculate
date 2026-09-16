package wechat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/example/go-service/internal/modules/moderation"
)

const (
	contentSafetyTextPath  = "/wxa/msg_sec_check"
	contentSafetyImagePath = "/wxa/img_sec_check"
	serverTokenPath        = "/cgi-bin/token"
)

type serverTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
	ErrCode     int    `json:"errcode"`
	ErrMessage  string `json:"errmsg"`
}

type contentSafetyResponse struct {
	ErrCode    int    `json:"errcode"`
	ErrMessage string `json:"errmsg"`
	TraceID    string `json:"trace_id"`
	RequestID  string `json:"request_id"`
}

var errContentSafetyTokenInvalid = fmt.Errorf("wechat content safety access token is invalid")

func (c *Client) CheckText(ctx context.Context, request moderation.TextCheckRequest) (moderation.ProviderResult, error) {
	requestContext, cancel := c.contentSafetyContext(ctx)
	defer cancel()
	return c.withContentSafetyToken(requestContext, func(token string) (moderation.ProviderResult, error) {
		return c.checkTextOnce(requestContext, token, request)
	})
}

func (c *Client) CheckImage(ctx context.Context, request moderation.ImageCheckRequest) (moderation.ProviderResult, error) {
	requestContext, cancel := c.contentSafetyContext(ctx)
	defer cancel()
	return c.withContentSafetyToken(requestContext, func(token string) (moderation.ProviderResult, error) {
		return c.checkImageOnce(requestContext, token, request)
	})
}

func (c *Client) withContentSafetyToken(ctx context.Context, check func(string) (moderation.ProviderResult, error)) (moderation.ProviderResult, error) {
	token, err := c.serverAccessToken(ctx)
	if err != nil {
		return moderation.ProviderResult{}, err
	}
	for attempt := 0; ; attempt++ {
		result, err := check(token)
		if err != errContentSafetyTokenInvalid || attempt >= c.contentSafetyMaxRetries {
			return result, err
		}
		c.invalidateServerAccessToken(token)
		token, err = c.serverAccessToken(ctx)
		if err != nil {
			return moderation.ProviderResult{}, err
		}
	}
}

func (c *Client) contentSafetyContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if c == nil {
		return context.WithCancel(ctx)
	}
	timeout := c.contentSafetyTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return context.WithTimeout(ctx, timeout)
}

func (c *Client) checkTextOnce(ctx context.Context, token string, request moderation.TextCheckRequest) (moderation.ProviderResult, error) {
	body, err := json.Marshal(struct {
		Content string `json:"content"`
		Version int    `json:"version"`
		Scene   int    `json:"scene"`
		OpenID  string `json:"openid,omitempty"`
	}{
		Content: request.Content,
		Version: 2,
		Scene:   2,
		OpenID:  strings.TrimSpace(request.Subject),
	})
	if err != nil {
		return moderation.ProviderResult{}, fmt.Errorf("marshal content safety text request")
	}
	return c.doContentSafetyJSON(ctx, contentSafetyTextPath, token, body)
}

func (c *Client) checkImageOnce(ctx context.Context, token string, request moderation.ImageCheckRequest) (moderation.ProviderResult, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("media", "avatar.webp")
	if err != nil {
		return moderation.ProviderResult{}, fmt.Errorf("create content safety image request")
	}
	if _, err := part.Write(request.Content); err != nil {
		return moderation.ProviderResult{}, fmt.Errorf("write content safety image request")
	}
	if err := writer.Close(); err != nil {
		return moderation.ProviderResult{}, fmt.Errorf("close content safety image request")
	}
	return c.doContentSafetyRequest(ctx, contentSafetyImagePath, token, &body, writer.FormDataContentType())
}

func (c *Client) doContentSafetyJSON(ctx context.Context, path, token string, body []byte) (moderation.ProviderResult, error) {
	return c.doContentSafetyRequest(ctx, path, token, bytes.NewReader(body), "application/json")
}

func (c *Client) doContentSafetyRequest(ctx context.Context, path, token string, body io.Reader, contentType string) (moderation.ProviderResult, error) {
	endpoint, err := c.endpoint(path)
	if err != nil {
		return moderation.ProviderResult{}, err
	}
	query := endpoint.Query()
	query.Set("access_token", token)
	endpoint.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), body)
	if err != nil {
		return moderation.ProviderResult{}, fmt.Errorf("create content safety request")
	}
	request.Header.Set("Content-Type", contentType)
	response, err := c.httpClient.Do(request)
	if err != nil {
		return moderation.ProviderResult{}, fmt.Errorf("content safety request failed")
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return moderation.ProviderResult{}, fmt.Errorf("content safety returned status %d", response.StatusCode)
	}
	var payload contentSafetyResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil {
		return moderation.ProviderResult{}, fmt.Errorf("decode content safety response")
	}
	requestID := strings.TrimSpace(payload.TraceID)
	if requestID == "" {
		requestID = strings.TrimSpace(payload.RequestID)
	}
	switch payload.ErrCode {
	case 0:
		return moderation.ProviderResult{Status: moderation.StatusApproved, ProviderRequestID: requestID}, nil
	case 87014:
		return moderation.ProviderResult{Status: moderation.StatusRejected, ReasonCode: strconv.Itoa(payload.ErrCode), ProviderRequestID: requestID}, nil
	case 40001, 40014, 42001:
		return moderation.ProviderResult{}, errContentSafetyTokenInvalid
	default:
		return moderation.ProviderResult{}, fmt.Errorf("content safety returned code %d", payload.ErrCode)
	}
}

func (c *Client) serverAccessToken(ctx context.Context) (string, error) {
	if c == nil || c.httpClient == nil || strings.TrimSpace(c.appID) == "" || strings.TrimSpace(c.appSecret) == "" || strings.TrimSpace(c.apiBaseURL) == "" {
		return "", ErrNotConfigured
	}
	now := time.Now()
	c.contentTokenMu.Lock()
	if c.contentToken != "" && now.Before(c.contentTokenExpiresAt) {
		token := c.contentToken
		c.contentTokenMu.Unlock()
		return token, nil
	}
	c.contentTokenMu.Unlock()

	endpoint, err := c.endpoint(serverTokenPath)
	if err != nil {
		return "", err
	}
	query := endpoint.Query()
	query.Set("grant_type", "client_credential")
	query.Set("appid", c.appID)
	query.Set("secret", c.appSecret)
	endpoint.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return "", fmt.Errorf("create wechat server token request")
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("wechat server token request failed")
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("wechat server token returned status %d", response.StatusCode)
	}
	var payload serverTokenResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode wechat server token response")
	}
	if payload.ErrCode != 0 || strings.TrimSpace(payload.AccessToken) == "" || payload.ExpiresIn <= 0 {
		return "", fmt.Errorf("wechat server token returned code %d", payload.ErrCode)
	}
	ttl := time.Duration(payload.ExpiresIn)*time.Second - time.Minute
	if ttl <= 0 {
		ttl = time.Second
	}
	expiresAt := time.Now().Add(ttl)
	c.contentTokenMu.Lock()
	c.contentToken = strings.TrimSpace(payload.AccessToken)
	c.contentTokenExpiresAt = expiresAt
	token := c.contentToken
	c.contentTokenMu.Unlock()
	return token, nil
}

func (c *Client) invalidateServerAccessToken(token string) {
	c.contentTokenMu.Lock()
	if c.contentToken == token {
		c.contentToken = ""
		c.contentTokenExpiresAt = time.Time{}
	}
	c.contentTokenMu.Unlock()
}

func (c *Client) endpoint(path string) (*url.URL, error) {
	base, err := url.Parse(strings.TrimRight(strings.TrimSpace(c.apiBaseURL), "/"))
	if err != nil || base.Scheme == "" || base.Host == "" {
		return nil, fmt.Errorf("wechat api base URL is invalid")
	}
	base.Path = strings.TrimRight(base.Path, "/") + path
	base.RawQuery = ""
	base.Fragment = ""
	return base, nil
}
