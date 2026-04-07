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
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/controlado/lol-autobuild/internal/ports"
)

var ErrNotConfigured = errors.New("lcu client is not configured")
var ErrLockfileNotFound = errors.New("lcu lockfile not found")
var ErrInvalidLockfile = errors.New("invalid lcu lockfile")
var ErrChampSelectUnavailable = errors.New("champ select session is unavailable")
var ErrChampionNotSelected = errors.New("local champion is not selected")

type Client struct {
	Enabled      bool
	LockfilePath string
	HTTPClient   *http.Client
}

func NewClient(enabled bool, lockfilePath string) *Client {
	return &Client{
		Enabled:      enabled,
		LockfilePath: strings.TrimSpace(lockfilePath),
	}
}

func (c *Client) DetectChampionID(ctx context.Context) (int, error) {
	if !c.Enabled {
		return 0, ErrNotConfigured
	}

	info, err := c.readLockfile()
	if err != nil {
		return 0, err
	}

	session, err := c.fetchChampSelectSession(ctx, info)
	if err != nil {
		return 0, err
	}

	for _, member := range session.MyTeam {
		if member.CellID != session.LocalPlayerCellID {
			continue
		}

		if member.ChampionID <= 0 {
			return 0, ErrChampionNotSelected
		}

		return member.ChampionID, nil
	}

	return 0, fmt.Errorf("%w: local player cell %d not found in myTeam", ErrChampSelectUnavailable, session.LocalPlayerCellID)
}

type lockfileInfo struct {
	Port     int
	Password string
	Protocol string
}

type champSelectSession struct {
	LocalPlayerCellID int `json:"localPlayerCellId"`
	MyTeam            []struct {
		CellID     int `json:"cellId"`
		ChampionID int `json:"championId"`
	} `json:"myTeam"`
}

func (c *Client) readLockfile() (lockfileInfo, error) {
	lockfilePath, err := c.resolveLockfilePath()
	if err != nil {
		return lockfileInfo{}, err
	}

	raw, err := os.ReadFile(lockfilePath)
	if err != nil {
		return lockfileInfo{}, fmt.Errorf("%w: %v", ErrLockfileNotFound, err)
	}

	return parseLockfile(raw)
}

func (c *Client) resolveLockfilePath() (string, error) {
	candidates := autoDiscoverLockfilePaths()
	if c.LockfilePath != "" {
		candidates = append(candidates, c.LockfilePath)
	}

	for _, path := range candidates {
		cleanPath := strings.TrimSpace(path)
		if cleanPath == "" {
			continue
		}

		stat, err := os.Stat(cleanPath)
		if err != nil || stat.IsDir() {
			continue
		}

		return cleanPath, nil
	}

	return "", ErrLockfileNotFound
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

func autoDiscoverLockfilePaths() []string {
	switch runtime.GOOS {
	case "windows":
		paths := []string{`C:\Riot Games\League of Legends\lockfile`}
		if v := strings.TrimSpace(os.Getenv("ProgramFiles")); v != "" {
			paths = append(paths, filepath.Join(v, "Riot Games", "League of Legends", "lockfile"))
		}
		if v := strings.TrimSpace(os.Getenv("ProgramFiles(x86)")); v != "" {
			paths = append(paths, filepath.Join(v, "Riot Games", "League of Legends", "lockfile"))
		}
		return paths
	case "darwin":
		paths := []string{"/Applications/League of Legends.app/Contents/LoL/lockfile"}
		if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
			paths = append(paths, filepath.Join(home, "Applications", "League of Legends.app", "Contents", "LoL", "lockfile"))
		}
		return paths
	default:
		return nil
	}
}

func (c *Client) fetchChampSelectSession(ctx context.Context, info lockfileInfo) (champSelectSession, error) {
	url := fmt.Sprintf("%s://127.0.0.1:%d/lol-champ-select/v1/session", info.Protocol, info.Port)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return champSelectSession{}, fmt.Errorf("%w: build request: %v", ErrChampSelectUnavailable, err)
	}

	token := base64.StdEncoding.EncodeToString([]byte("riot:" + info.Password))
	req.Header.Set("Authorization", "Basic "+token)
	req.Header.Set("Accept", "application/json")

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

func (c *Client) ApplyItemSet(ctx context.Context, req ports.ApplyItemSetRequest) error {
	_ = ctx
	if !c.Enabled {
		return ErrNotConfigured
	}

	if req.DryRun {
		return nil
	}

	return fmt.Errorf("apply item set: %w", ErrNotConfigured)
}

func (c *Client) ApplyRunePage(ctx context.Context, req ports.ApplyRunePageRequest) error {
	_ = ctx
	if !c.Enabled {
		return ErrNotConfigured
	}

	if req.DryRun {
		return nil
	}

	return fmt.Errorf("apply rune page: %w", ErrNotConfigured)
}

func (c *Client) ApplySummonerSpells(ctx context.Context, req ports.ApplySummonerSpellsRequest) error {
	_ = ctx
	if !c.Enabled {
		return ErrNotConfigured
	}

	if req.DryRun {
		return nil
	}

	return fmt.Errorf("apply summoner spells: %w", ErrNotConfigured)
}

type StubClient struct {
	DetectedChampionID int
	DetectChampionErr  error
	ItemSetCalls       []ports.ApplyItemSetRequest
	RunePageCalls      []ports.ApplyRunePageRequest
	SummonerSpellCalls []ports.ApplySummonerSpellsRequest
	ItemSetErr         error
	RunePageErr        error
	SummonerSpellsErr  error
}

func (c *StubClient) DetectChampionID(ctx context.Context) (int, error) {
	_ = ctx
	if c.DetectChampionErr != nil {
		return 0, c.DetectChampionErr
	}

	return c.DetectedChampionID, nil
}

func (c *StubClient) ApplyItemSet(ctx context.Context, req ports.ApplyItemSetRequest) error {
	_ = ctx
	c.ItemSetCalls = append(c.ItemSetCalls, req)
	return c.ItemSetErr
}

func (c *StubClient) ApplyRunePage(ctx context.Context, req ports.ApplyRunePageRequest) error {
	_ = ctx
	c.RunePageCalls = append(c.RunePageCalls, req)
	return c.RunePageErr
}

func (c *StubClient) ApplySummonerSpells(ctx context.Context, req ports.ApplySummonerSpellsRequest) error {
	_ = ctx
	c.SummonerSpellCalls = append(c.SummonerSpellCalls, req)
	return c.SummonerSpellsErr
}
