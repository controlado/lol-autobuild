package lcu

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var (
	errHTTPNotFound = errors.New("http: not found")
	errHTTPStatus   = errors.New("http: bad status")
)

const httpStatusBodyLimit = 256

type httpStatusError struct {
	sentinel   error
	statusCode int
	body       string
}

func newHTTPStatusError(statusCode int, body string) *httpStatusError {
	sentinel := errHTTPStatus
	if statusCode == http.StatusNotFound {
		sentinel = errHTTPNotFound
	}

	return &httpStatusError{
		sentinel:   sentinel,
		statusCode: statusCode,
		body:       body,
	}
}

func (e *httpStatusError) Error() string {
	if e.body == "" {
		return fmt.Sprintf("%v: status: %d", e.sentinel, e.statusCode)
	}
	return fmt.Sprintf("%v: status: %d: %s", e.sentinel, e.statusCode, e.body)
}

func (e *httpStatusError) Unwrap() error {
	return e.sentinel
}

func (e *httpStatusError) StatusCode() int {
	return e.statusCode
}

func (e *httpStatusError) Body() string {
	return e.body
}

func doRequest(ctx context.Context, c *Client, info connectionInfo, method, path string, body any) error {
	_, err := doJSON[struct{}](ctx, c, info, method, path, body)
	return err
}

func doJSON[T any](ctx context.Context, c *Client, info connectionInfo, method, path string, body any) (T, error) {
	var (
		zeroValueResult T
		reader          io.Reader
	)

	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return zeroValueResult, fmt.Errorf("encode payload: %w", err)
		}
		reader = bytes.NewReader(raw)
	}

	url := fmt.Sprintf("%s://127.0.0.1:%d%s", info.Protocol, info.Port, path)
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return zeroValueResult, fmt.Errorf("build request: %w", err)
	}

	applyHeaders(req, info.Password)

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient(info.Protocol).Do(req)
	if err != nil {
		return zeroValueResult, fmt.Errorf("transport: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var valueResult T

		if _, isUnit := any(valueResult).(struct{}); isUnit {
			return zeroValueResult, nil
		}

		if err := json.NewDecoder(resp.Body).Decode(&valueResult); err != nil {
			return zeroValueResult, fmt.Errorf("decode response: %w", err)
		}

		return valueResult, nil
	}

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, httpStatusBodyLimit))
	return zeroValueResult, newHTTPStatusError(resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
}

func (c *Client) httpClient(protocol string) *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}

	client := &http.Client{Timeout: 3 * time.Second}
	if protocol == "https" {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	}

	return client
}
