package lcu

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var ErrNotConfigured = errors.New("lcu client is not configured")
var ErrLockfileNotFound = errors.New("lcu lockfile not found")
var ErrInvalidLockfile = errors.New("invalid lcu lockfile")
var ErrChampSelectUnavailable = errors.New("champ select session is unavailable")
var ErrChampionNotSelected = errors.New("local champion is not selected")
var ErrRoleDetectionUnsupportedQueue = errors.New("role detection is unsupported for this queue")
var ErrRoleNotAssigned = errors.New("local role is not assigned")
var ErrRoleUnknown = errors.New("local role is unknown")
var ErrInvalidSummonerSpellsRequest = errors.New("invalid summoner spells apply request")
var ErrChampionSelectionChanged = errors.New("champion selection changed during apply")
var ErrSummonerSpellsApplyFailed = errors.New("apply summoner spells failed")

type Client struct {
	Enabled             bool
	LockfilePath        string
	HTTPClient          *http.Client
	WatchReconnectDelay time.Duration

	discoverOpenClientConnections func(context.Context) []clientConnectionCandidate
}

func NewClient(enabled bool, lockfilePath string) *Client {
	return &Client{
		Enabled:                       enabled,
		LockfilePath:                  strings.TrimSpace(lockfilePath),
		WatchReconnectDelay:           time.Second,
		discoverOpenClientConnections: discoverOpenClientConnections,
	}
}

type lockfileInfo struct {
	Port     int
	Password string
	Protocol string
}

type champSelectSession struct {
	LocalPlayerCellID int                          `json:"localPlayerCellId"`
	QueueID           int                          `json:"queueId"`
	MyTeam            []champSelectPlayerSelection `json:"myTeam"`
}

type champSelectPlayerSelection struct {
	CellID           int    `json:"cellId"`
	ChampionID       int    `json:"championId"`
	AssignedPosition string `json:"assignedPosition"`
	IsAutofilled     bool   `json:"isAutofilled"`
	Spell1ID         int    `json:"spell1Id"`
	Spell2ID         int    `json:"spell2Id"`
}

type champSelectMySelectionPatch struct {
	Spell1ID int `json:"spell1Id"`
	Spell2ID int `json:"spell2Id"`
}

func (c *Client) readLockfile(lockfilePath string) (lockfileInfo, error) {
	raw, err := os.ReadFile(lockfilePath)
	if err != nil {
		return lockfileInfo{}, fmt.Errorf("%w: %v", ErrLockfileNotFound, err)
	}

	return parseLockfile(raw)
}

func withLastCandidateError(base error, last error) error {
	if last == nil {
		return base
	}

	return errors.Join(base, fmt.Errorf("last candidate error: %w", last))
}

func parseLockfile(raw []byte) (lockfileInfo, error) {
	parts := strings.Split(strings.TrimSpace(string(raw)), ":")
	if len(parts) != 5 {
		return lockfileInfo{}, fmt.Errorf("%w: expected 5 fields", ErrInvalidLockfile)
	}

	port, err := strconv.Atoi(parts[2])
	if err != nil || port <= 0 {
		return lockfileInfo{}, fmt.Errorf("%w: invalid port", ErrInvalidLockfile)
	}

	password := strings.TrimSpace(parts[3])
	if password == "" {
		return lockfileInfo{}, fmt.Errorf("%w: missing password", ErrInvalidLockfile)
	}

	protocol := strings.ToLower(strings.TrimSpace(parts[4]))
	if protocol != "https" && protocol != "http" {
		return lockfileInfo{}, fmt.Errorf("%w: unsupported protocol %q", ErrInvalidLockfile, protocol)
	}

	return lockfileInfo{
		Port:     port,
		Password: password,
		Protocol: protocol,
	}, nil
}

func (c *Client) fetchChampSelectSession(ctx context.Context, info lockfileInfo) (champSelectSession, error) {
	url := fmt.Sprintf("%s://127.0.0.1:%d/lol-champ-select/v1/session", info.Protocol, info.Port)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return champSelectSession{}, fmt.Errorf("%w: build request: %v", ErrChampSelectUnavailable, err)
	}

	applyLCUHeaders(req, info.Password)

	resp, err := c.httpClient(info.Protocol).Do(req)
	if err != nil {
		return champSelectSession{}, fmt.Errorf("%w: %v", ErrChampSelectUnavailable, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	switch resp.StatusCode {
	case http.StatusOK:
		var session champSelectSession
		if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
			return champSelectSession{}, fmt.Errorf("%w: decode response: %v", ErrChampSelectUnavailable, err)
		}
		return session, nil
	case http.StatusNotFound:
		return champSelectSession{}, ErrChampSelectUnavailable
	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		if len(body) == 0 {
			return champSelectSession{}, fmt.Errorf("%w: status %d", ErrChampSelectUnavailable, resp.StatusCode)
		}
		return champSelectSession{}, fmt.Errorf("%w: status %d: %s", ErrChampSelectUnavailable, resp.StatusCode, strings.TrimSpace(string(body)))
	}
}

func applyLCUHeaders(req *http.Request, password string) {
	req.Header.Set("Authorization", lcuBasicAuthHeader(password))
	req.Header.Set("Accept", "application/json")
}

func lcuBasicAuthHeader(password string) string {
	token := base64.StdEncoding.EncodeToString([]byte("riot:" + password))
	return "Basic " + token
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
