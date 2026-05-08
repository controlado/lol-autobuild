package lcu

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func assertHTTPStatusError(t *testing.T, err error, wantStatusCode int, wantBody string) {
	t.Helper()

	var statusErr *httpStatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("expected httpStatusError, got %v", err)
	}
	if statusErr.StatusCode() != wantStatusCode {
		t.Fatalf("expected status code %d, got %d", wantStatusCode, statusErr.StatusCode())
	}
	if statusErr.Body() != wantBody {
		t.Fatalf("expected body %q, got %q", wantBody, statusErr.Body())
	}
}

func TestDoJSON(t *testing.T) {
	t.Parallel()

	type requestSnapshot struct {
		Method      string
		Path        string
		Host        string
		Auth        string
		Accept      string
		ContentType string
		Body        string
	}

	type sampleResponse struct {
		OK bool `json:"ok"`
	}

	type samplePayload struct {
		Champion int `json:"champion"`
	}

	tests := []struct {
		name           string
		method         string
		path           string
		body           any
		responseStatus int
		responseBody   string
		call           func(context.Context, *Client, connectionInfo, string, string, any) (any, error)
		assert         func(*testing.T, any, error, requestSnapshot, connectionInfo, int)
	}{
		{
			name:           "2xx decodes json and sends json payload with headers",
			method:         http.MethodPost,
			path:           "/ok",
			body:           samplePayload{Champion: 240},
			responseStatus: http.StatusOK,
			responseBody:   `{"ok":true}`,
			call: func(ctx context.Context, c *Client, info connectionInfo, method, path string, body any) (any, error) {
				return doJSON[sampleResponse](ctx, c, info, method, path, body)
			},
			assert: func(t *testing.T, result any, err error, gotReq requestSnapshot, info connectionInfo, port int) {
				t.Helper()

				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}

				decoded, ok := result.(sampleResponse)
				if !ok {
					t.Fatalf("expected sampleResponse type, got %T", result)
				}
				if !decoded.OK {
					t.Fatalf("expected decoded OK=true, got %#v", decoded)
				}

				if gotReq.Method != http.MethodPost {
					t.Fatalf("expected method POST, got %q", gotReq.Method)
				}
				if gotReq.Path != "/ok" {
					t.Fatalf("expected path /ok, got %q", gotReq.Path)
				}
				if gotReq.Host != "127.0.0.1:"+strconv.Itoa(port) {
					t.Fatalf("expected host 127.0.0.1:%d, got %q", port, gotReq.Host)
				}
				if gotReq.Auth != basicAuthHeader(info.Password) {
					t.Fatalf("unexpected authorization header: %q", gotReq.Auth)
				}
				if gotReq.Accept != "application/json" {
					t.Fatalf("expected Accept application/json, got %q", gotReq.Accept)
				}
				if gotReq.ContentType != "application/json" {
					t.Fatalf("expected Content-Type application/json, got %q", gotReq.ContentType)
				}
				if gotReq.Body != `{"champion":240}` {
					t.Fatalf("unexpected request body: %q", gotReq.Body)
				}
			},
		},
		{
			name:           "404 returns not found",
			method:         http.MethodGet,
			path:           "/missing",
			body:           nil,
			responseStatus: http.StatusNotFound,
			responseBody:   `missing`,
			call: func(ctx context.Context, c *Client, info connectionInfo, method, path string, body any) (any, error) {
				return doJSON[sampleResponse](ctx, c, info, method, path, body)
			},
			assert: func(t *testing.T, _ any, err error, gotReq requestSnapshot, info connectionInfo, port int) {
				t.Helper()

				if !errors.Is(err, errHTTPNotFound) {
					t.Fatalf("expected errHTTPNotFound, got %v", err)
				}
				if errors.Is(err, errHTTPStatus) {
					t.Fatalf("expected 404 not to match errHTTPStatus")
				}
				assertHTTPStatusError(t, err, http.StatusNotFound, "missing")
				if err.Error() != "http: not found: status: 404: missing" {
					t.Fatalf("unexpected error text: %v", err)
				}
				if gotReq.Path != "/missing" {
					t.Fatalf("expected path /missing, got %q", gotReq.Path)
				}
				if gotReq.Host != "127.0.0.1:"+strconv.Itoa(port) {
					t.Fatalf("expected host 127.0.0.1:%d, got %q", port, gotReq.Host)
				}
				if gotReq.Auth != basicAuthHeader(info.Password) {
					t.Fatalf("unexpected authorization header: %q", gotReq.Auth)
				}
				if gotReq.Accept != "application/json" {
					t.Fatalf("expected Accept application/json, got %q", gotReq.Accept)
				}
				if gotReq.ContentType != "" {
					t.Fatalf("expected empty Content-Type when body=nil, got %q", gotReq.ContentType)
				}
			},
		},
		{
			name:           "5xx with body returns status and trimmed body",
			method:         http.MethodGet,
			path:           "/boom",
			body:           nil,
			responseStatus: http.StatusInternalServerError,
			responseBody:   "  exploded \n",
			call: func(ctx context.Context, c *Client, info connectionInfo, method, path string, body any) (any, error) {
				return doJSON[sampleResponse](ctx, c, info, method, path, body)
			},
			assert: func(t *testing.T, _ any, err error, _ requestSnapshot, _ connectionInfo, _ int) {
				t.Helper()

				if !errors.Is(err, errHTTPStatus) {
					t.Fatalf("expected errHTTPStatus, got %v", err)
				}
				if errors.Is(err, errHTTPNotFound) {
					t.Fatalf("expected 5xx not to match errHTTPNotFound")
				}
				assertHTTPStatusError(t, err, http.StatusInternalServerError, "exploded")
				if err.Error() != "http: bad status: status: 500: exploded" {
					t.Fatalf("unexpected error text: %v", err)
				}
			},
		},
		{
			name:           "5xx without body returns status only",
			method:         http.MethodGet,
			path:           "/empty",
			body:           nil,
			responseStatus: http.StatusServiceUnavailable,
			responseBody:   "",
			call: func(ctx context.Context, c *Client, info connectionInfo, method, path string, body any) (any, error) {
				return doJSON[sampleResponse](ctx, c, info, method, path, body)
			},
			assert: func(t *testing.T, _ any, err error, _ requestSnapshot, _ connectionInfo, _ int) {
				t.Helper()

				if !errors.Is(err, errHTTPStatus) {
					t.Fatalf("expected errHTTPStatus, got %v", err)
				}
				assertHTTPStatusError(t, err, http.StatusServiceUnavailable, "")
				if err.Error() != "http: bad status: status: 503" {
					t.Fatalf("unexpected error text: %v", err)
				}
			},
		},
		{
			name:           "5xx limits response body",
			method:         http.MethodGet,
			path:           "/long-body",
			body:           nil,
			responseStatus: http.StatusInternalServerError,
			responseBody:   strings.Repeat("a", httpStatusBodyLimit+10),
			call: func(ctx context.Context, c *Client, info connectionInfo, method, path string, body any) (any, error) {
				return doJSON[sampleResponse](ctx, c, info, method, path, body)
			},
			assert: func(t *testing.T, _ any, err error, _ requestSnapshot, _ connectionInfo, _ int) {
				t.Helper()

				if !errors.Is(err, errHTTPStatus) {
					t.Fatalf("expected errHTTPStatus, got %v", err)
				}
				assertHTTPStatusError(t, err, http.StatusInternalServerError, strings.Repeat("a", httpStatusBodyLimit))
			},
		},
		{
			name:           "2xx with invalid json returns decode error",
			method:         http.MethodGet,
			path:           "/invalid-json",
			body:           nil,
			responseStatus: http.StatusOK,
			responseBody:   "{invalid",
			call: func(ctx context.Context, c *Client, info connectionInfo, method, path string, body any) (any, error) {
				return doJSON[sampleResponse](ctx, c, info, method, path, body)
			},
			assert: func(t *testing.T, _ any, err error, _ requestSnapshot, _ connectionInfo, _ int) {
				t.Helper()

				if err == nil {
					t.Fatalf("expected decode error, got nil")
				}
				if !strings.Contains(err.Error(), "decode response") {
					t.Fatalf("expected decode response error, got %v", err)
				}
			},
		},
		{
			name:           "2xx with struct type skips response decode",
			method:         http.MethodGet,
			path:           "/unit",
			body:           nil,
			responseStatus: http.StatusOK,
			responseBody:   "{invalid",
			call: func(ctx context.Context, c *Client, info connectionInfo, method, path string, body any) (any, error) {
				return doJSON[struct{}](ctx, c, info, method, path, body)
			},
			assert: func(t *testing.T, _ any, err error, _ requestSnapshot, _ connectionInfo, _ int) {
				t.Helper()

				if err != nil {
					t.Fatalf("expected nil error for struct{} decode skip, got %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var gotReq requestSnapshot
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				rawBody, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("read request body: %v", err)
				}

				gotReq = requestSnapshot{
					Method:      r.Method,
					Path:        r.URL.Path,
					Host:        r.Host,
					Auth:        r.Header.Get("Authorization"),
					Accept:      r.Header.Get("Accept"),
					ContentType: r.Header.Get("Content-Type"),
					Body:        string(rawBody),
				}

				w.WriteHeader(tt.responseStatus)
				if tt.responseBody != "" {
					_, _ = io.WriteString(w, tt.responseBody)
				}
			}))
			defer server.Close()

			var (
				port = mustServerPort(t, server.URL)
				info = connectionInfo{
					Port:     port,
					Password: "secret",
					Protocol: "http",
				}
			)
			result, err := tt.call(context.Background(), &Client{}, info, tt.method, tt.path, tt.body)
			tt.assert(t, result, err, gotReq, info, port)
		})
	}
}

func TestDoRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		status            int
		responseBody      string
		expectErrIs       error
		expectErrContains string
	}{
		{
			name:         "2xx returns nil",
			status:       http.StatusNoContent,
			responseBody: "",
		},
		{
			name:              "404 returns not found",
			status:            http.StatusNotFound,
			responseBody:      "missing",
			expectErrIs:       errHTTPNotFound,
			expectErrContains: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				if tt.responseBody != "" {
					_, _ = io.WriteString(w, tt.responseBody)
				}
			}))
			defer server.Close()

			var (
				info = connectionInfo{
					Port:     mustServerPort(t, server.URL),
					Password: "secret",
					Protocol: "http",
				}
				body = map[string]int{"spell1Id": 4}
			)
			err := doRequest(context.Background(), &Client{}, info, http.MethodPatch, "/request", body)

			if tt.expectErrIs == nil {
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
				return
			}

			if !errors.Is(err, tt.expectErrIs) {
				t.Fatalf("expected error %v, got %v", tt.expectErrIs, err)
			}
			if tt.expectErrContains != "" && !strings.Contains(err.Error(), tt.expectErrContains) {
				t.Fatalf("expected error containing %q, got %v", tt.expectErrContains, err)
			}
		})
	}
}

func TestDoJSON_EncodePayloadError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body any
	}{
		{
			name: "non serializable channel",
			body: make(chan int),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var (
				info = connectionInfo{
					Port:     8080,
					Password: "secret",
					Protocol: "http",
				}
			)
			_, err := doJSON[struct{}](context.Background(), &Client{}, info, http.MethodPost, "/ignored", tt.body)
			if err == nil {
				t.Fatalf("expected encode payload error, got nil")
			}
			if !strings.Contains(err.Error(), "encode payload") {
				t.Fatalf("expected encode payload error, got %v", err)
			}
		})
	}
}

func TestHTTPClient(t *testing.T) {
	t.Parallel()

	custom := &http.Client{Timeout: time.Second}

	if got := (&Client{HTTPClient: custom}).httpClient("https"); got != custom {
		t.Fatalf("expected configured custom client pointer")
	}

	httpClient := (&Client{}).httpClient("http")
	if got := (&Client{}).httpClient("http"); got != httpClient {
		t.Fatalf("expected shared default http client")
	}
	assertDefaultLCUHTTPClient(t, httpClient, false)

	httpsClient := (&Client{}).httpClient("https")
	if got := (&Client{}).httpClient("https"); got != httpsClient {
		t.Fatalf("expected shared default https client")
	}
	if httpsClient == httpClient {
		t.Fatalf("expected separate default clients for http and https")
	}
	assertDefaultLCUHTTPClient(t, httpsClient, true)
}

func TestHTTPClientConcurrentDefaultReuse(t *testing.T) {
	t.Parallel()

	const workers = 32
	var (
		wg      sync.WaitGroup
		results = make(chan *http.Client, workers)
	)

	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			results <- (&Client{}).httpClient("https")
		}()
	}
	wg.Wait()
	close(results)

	var first *http.Client
	for got := range results {
		if first == nil {
			first = got
			continue
		}
		if got != first {
			t.Fatalf("expected concurrent calls to share the same default client")
		}
	}
	assertDefaultLCUHTTPClient(t, first, true)
}

func assertDefaultLCUHTTPClient(t *testing.T, got *http.Client, wantTLS bool) {
	t.Helper()

	if got == nil {
		t.Fatalf("expected non-nil client")
	}
	if got.Timeout != 3*time.Second {
		t.Fatalf("expected timeout 3s, got %s", got.Timeout)
	}
	if got.Transport == nil {
		t.Fatalf("expected default transport")
	}
	if got.Transport == http.DefaultTransport {
		t.Fatalf("expected cloned default transport")
	}

	transport, ok := got.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport, got %T", got.Transport)
	}
	if !transport.DisableKeepAlives {
		t.Fatalf("expected DisableKeepAlives=true")
	}
	if wantTLS {
		if transport.TLSClientConfig == nil {
			t.Fatalf("expected TLSClientConfig to be set")
		}
		if !transport.TLSClientConfig.InsecureSkipVerify {
			t.Fatalf("expected InsecureSkipVerify=true for https protocol")
		}
		return
	}
	if transport.TLSClientConfig != nil && transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatalf("expected http protocol not to enable insecure TLS")
	}
}
