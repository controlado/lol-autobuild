package lcu

import (
	"crypto/tls"
	"encoding/base64"
	"net/http"
	"time"
)

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

func applyHeaders(req *http.Request, password string) {
	req.Header.Set("Authorization", basicAuthHeader(password))
	req.Header.Set("Accept", "application/json")
}

func basicAuthHeader(password string) string {
	token := base64.StdEncoding.EncodeToString([]byte("riot:" + password))
	return "Basic " + token
}
