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
	"strings"
	"time"
)

var (
	ErrNotConfigured                 = errors.New("lcu client is not configured")
	ErrLockfileNotFound              = errors.New("lcu lockfile not found")
	ErrInvalidLockfile               = errors.New("invalid lcu lockfile")
	ErrChampSelectUnavailable        = errors.New("champ select session is unavailable")
	ErrChampionNotSelected           = errors.New("local champion is not selected")
	ErrRoleDetectionUnsupportedQueue = errors.New("role detection is unsupported for this queue")
	ErrRoleNotAssigned               = errors.New("local role is not assigned")
	ErrRoleUnknown                   = errors.New("local role is unknown")
	ErrInvalidSummonerSpellsRequest  = errors.New("invalid summoner spells apply request")
	ErrChampionSelectionChanged      = errors.New("champion selection changed during apply")
	ErrSummonerSpellsApplyFailed     = errors.New("apply summoner spells failed")
)

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

func (c *Client) readLockfile(lockfilePath string) (lockfileInfo, error) {
	raw, err := os.ReadFile(lockfilePath)
	if err != nil {
		return lockfileInfo{}, fmt.Errorf("%w: %v", ErrLockfileNotFound, err)
	}

	return parseLockfile(raw)
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

func applyLCUHeaders(req *http.Request, password string) {
	req.Header.Set("Authorization", lcuBasicAuthHeader(password))
	req.Header.Set("Accept", "application/json")
}

func lcuBasicAuthHeader(password string) string {
	token := base64.StdEncoding.EncodeToString([]byte("riot:" + password))
	return "Basic " + token
}

func withLastCandidateError(base error, last error) error {
	if last == nil {
		return base
	}

	return errors.Join(base, fmt.Errorf("last candidate error: %w", last))
}
