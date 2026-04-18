package lcu

import (
	"encoding/base64"
	"net/http"
)

func applyHeaders(req *http.Request, password string) {
	req.Header.Set("Authorization", basicAuthHeader(password))
	req.Header.Set("Accept", "application/json")
}

func basicAuthHeader(password string) string {
	token := base64.StdEncoding.EncodeToString([]byte("riot:" + password))
	return "Basic " + token
}
