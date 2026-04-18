package lcu

import (
	"crypto/tls"
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
