package user

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultWeChatAvatarFetchTimeout = 5 * time.Second

// HTTPWeChatAvatarFetcher downloads only the short-lived avatar URL returned
// by WeChat. The caller still has to decode and moderate the bytes before they
// can become a public avatar.
type HTTPWeChatAvatarFetcher struct {
	client  *http.Client
	timeout time.Duration
}

// NewWeChatAvatarFetcher creates a fetcher with a bounded timeout. A caller
// may provide a custom transport, but redirects are always replaced with the
// allowlisted-host check below.
func NewWeChatAvatarFetcher(client *http.Client, timeout time.Duration) *HTTPWeChatAvatarFetcher {
	if timeout <= 0 {
		timeout = defaultWeChatAvatarFetchTimeout
	}
	if client == nil {
		client = &http.Client{}
	}
	copyClient := *client
	if copyClient.Timeout <= 0 || copyClient.Timeout > timeout {
		copyClient.Timeout = timeout
	}
	copyClient.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if err := validateWeChatAvatarURL(request.URL); err != nil {
			return fmt.Errorf("reject WeChat avatar redirect: %w", err)
		}
		if len(via) >= 5 {
			return fmt.Errorf("too many WeChat avatar redirects")
		}
		return nil
	}
	return &HTTPWeChatAvatarFetcher{client: &copyClient, timeout: timeout}
}

func (f *HTTPWeChatAvatarFetcher) Fetch(ctx context.Context, rawURL string, maxBytes int64) ([]byte, error) {
	if f == nil || f.client == nil {
		return nil, fmt.Errorf("WeChat avatar fetcher is not configured")
	}
	if maxBytes <= 0 {
		return nil, fmt.Errorf("WeChat avatar maximum size is invalid")
	}
	parsed, err := url.ParseRequestURI(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, fmt.Errorf("parse WeChat avatar URL: %w", err)
	}
	if err := validateWeChatAvatarURL(parsed); err != nil {
		return nil, err
	}

	requestContext := ctx
	if f.timeout > 0 {
		var cancel context.CancelFunc
		requestContext, cancel = context.WithTimeout(ctx, f.timeout)
		defer cancel()
	}
	request, err := http.NewRequestWithContext(requestContext, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create WeChat avatar request: %w", err)
	}
	request.Header.Set("Accept", "image/jpeg,image/png,image/webp")
	response, err := f.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch WeChat avatar: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("fetch WeChat avatar returned status %d", response.StatusCode)
	}
	if response.ContentLength > maxBytes {
		return nil, fmt.Errorf("WeChat avatar exceeds %d bytes", maxBytes)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read WeChat avatar: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("WeChat avatar response is empty")
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("WeChat avatar exceeds %d bytes", maxBytes)
	}
	return data, nil
}

func validateWeChatAvatarURL(value *url.URL) error {
	if value == nil || value.Scheme != "https" || value.Host == "" || value.User != nil || value.Fragment != "" || value.Port() != "" {
		return fmt.Errorf("WeChat avatar URL must use HTTPS without credentials or a port")
	}
	if !isWeChatAvatarHost(value.Hostname()) {
		return fmt.Errorf("WeChat avatar URL host is not allowlisted")
	}
	if strings.TrimSpace(value.Path) == "" || value.Path == "/" {
		return fmt.Errorf("WeChat avatar URL path is invalid")
	}
	return nil
}
