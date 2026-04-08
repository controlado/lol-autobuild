package lcu

import (
	"bytes"
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
var ErrRoleDetectionUnsupportedQueue = errors.New("role detection is unsupported for this queue")
var ErrRoleNotAssigned = errors.New("local role is not assigned")
var ErrRoleUnknown = errors.New("local role is unknown")
var ErrInvalidSummonerSpellsRequest = errors.New("invalid summoner spells apply request")
var ErrChampionSelectionChanged = errors.New("champion selection changed during apply")
var ErrSummonerSpellsApplyFailed = errors.New("apply summoner spells failed")

type queueID int

const (
	queueDraftPick   queueID = 400
	queueSoloDuo     queueID = 420
	queueFlex        queueID = 440
	queueRankedClash queueID = 3110
)

type Client struct {
	Enabled      bool
	LockfilePath string
	HTTPClient   *http.Client

	discoverLockfilePaths func() []string
}

func NewClient(enabled bool, lockfilePath string) *Client {
	return &Client{
		Enabled:      enabled,
		LockfilePath: strings.TrimSpace(lockfilePath),
		discoverLockfilePaths: func() []string {
			return autoDiscoverLockfilePaths()
		},
	}
}

func (c *Client) DetectSelection(ctx context.Context) (ports.DetectedSelection, error) {
	if !c.Enabled {
		return ports.DetectedSelection{}, ErrNotConfigured
	}

	var lastErr error
	seenExisting := false
	seenChampionNotSelected := false
	seenRoleNotAssigned := false
	seenRoleUnknown := false
	seenUnsupportedQueue := false
	seenSessionUnavailable := false

	for _, lockfilePath := range c.lockfileCandidates() {
		stat, err := os.Stat(lockfilePath)
		if err != nil || stat.IsDir() {
			continue
		}
		seenExisting = true

		info, err := c.readLockfile(lockfilePath)
		if err != nil {
			lastErr = fmt.Errorf("lockfile %q: %w", lockfilePath, err)
			continue
		}

		session, err := c.fetchChampSelectSession(ctx, info)
		if err != nil {
			if errors.Is(err, ErrChampSelectUnavailable) {
				seenSessionUnavailable = true
			}
			lastErr = fmt.Errorf("lockfile %q: %w", lockfilePath, err)
			continue
		}

		selection, err := selectionFromSession(session)
		if err != nil {
			if errors.Is(err, ErrChampionNotSelected) {
				seenChampionNotSelected = true
			}
			if errors.Is(err, ErrRoleNotAssigned) {
				seenRoleNotAssigned = true
			}
			if errors.Is(err, ErrRoleUnknown) {
				seenRoleUnknown = true
			}
			if errors.Is(err, ErrRoleDetectionUnsupportedQueue) {
				seenUnsupportedQueue = true
			}
			if errors.Is(err, ErrChampSelectUnavailable) {
				seenSessionUnavailable = true
			}
			lastErr = fmt.Errorf("lockfile %q: %w", lockfilePath, err)
			continue
		}

		return selection, nil
	}

	if seenChampionNotSelected {
		return ports.DetectedSelection{}, withLastCandidateError(ErrChampionNotSelected, lastErr)
	}

	if seenRoleNotAssigned {
		return ports.DetectedSelection{}, withLastCandidateError(ErrRoleNotAssigned, lastErr)
	}

	if seenRoleUnknown {
		return ports.DetectedSelection{}, withLastCandidateError(ErrRoleUnknown, lastErr)
	}

	if seenUnsupportedQueue {
		return ports.DetectedSelection{}, withLastCandidateError(ErrRoleDetectionUnsupportedQueue, lastErr)
	}

	if seenSessionUnavailable {
		return ports.DetectedSelection{}, withLastCandidateError(ErrChampSelectUnavailable, lastErr)
	}

	if !seenExisting {
		return ports.DetectedSelection{}, ErrLockfileNotFound
	}

	return ports.DetectedSelection{}, withLastCandidateError(ErrLockfileNotFound, lastErr)
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

func (c *Client) lockfileCandidates() []string {
	raw := make([]string, 0, 4)
	if c.discoverLockfilePaths != nil {
		raw = append(raw, c.discoverLockfilePaths()...)
	}
	if c.LockfilePath != "" {
		raw = append(raw, c.LockfilePath)
	}

	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, candidate := range raw {
		cleanPath := filepath.Clean(strings.TrimSpace(candidate))
		if cleanPath == "" || cleanPath == "." {
			continue
		}

		key := cleanPath
		if runtime.GOOS == "windows" {
			key = strings.ToLower(cleanPath)
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, cleanPath)
	}

	return out
}

func (c *Client) readLockfile(lockfilePath string) (lockfileInfo, error) {
	raw, err := os.ReadFile(lockfilePath)
	if err != nil {
		return lockfileInfo{}, fmt.Errorf("%w: %v", ErrLockfileNotFound, err)
	}

	return parseLockfile(raw)
}

func selectionFromSession(session champSelectSession) (ports.DetectedSelection, error) {
	if !isRoleDetectionQueueSupported(session.QueueID) {
		return ports.DetectedSelection{}, fmt.Errorf("%w: queueId %d", ErrRoleDetectionUnsupportedQueue, session.QueueID)
	}

	member, err := localPlayerFromSession(session)
	if err != nil {
		return ports.DetectedSelection{}, err
	}

	if member.ChampionID <= 0 {
		return ports.DetectedSelection{}, ErrChampionNotSelected
	}

	role, err := canonicalRoleFromAssignedPosition(member.AssignedPosition)
	if err != nil {
		return ports.DetectedSelection{}, err
	}

	return ports.DetectedSelection{
		ChampionID:   member.ChampionID,
		Role:         role,
		QueueID:      session.QueueID,
		IsAutofilled: member.IsAutofilled,
	}, nil
}

func localPlayerFromSession(session champSelectSession) (champSelectPlayerSelection, error) {
	for _, member := range session.MyTeam {
		if member.CellID == session.LocalPlayerCellID {
			return member, nil
		}
	}

	return champSelectPlayerSelection{}, fmt.Errorf("%w: local player cell %d not found in myTeam", ErrChampSelectUnavailable, session.LocalPlayerCellID)
}

func isRoleDetectionQueueSupported(queueIDValue int) bool {
	switch queueID(queueIDValue) {
	case queueDraftPick, queueSoloDuo, queueFlex, queueRankedClash:
		return true
	default:
		return false
	}
}

func canonicalRoleFromAssignedPosition(assignedPosition string) (string, error) {
	switch strings.ToUpper(strings.TrimSpace(assignedPosition)) {
	case "TOP":
		return "top", nil
	case "JUNGLE":
		return "jungle", nil
	case "MIDDLE":
		return "mid", nil
	case "BOTTOM":
		return "adc", nil
	case "UTILITY":
		return "support", nil
	case "", "FILL", "UNSELECTED":
		return "", fmt.Errorf("%w: assignedPosition %q", ErrRoleNotAssigned, assignedPosition)
	default:
		return "", fmt.Errorf("%w: assignedPosition %q", ErrRoleUnknown, assignedPosition)
	}
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
	if !c.Enabled {
		return ErrNotConfigured
	}

	if req.DryRun {
		return nil
	}

	if err := validateApplySummonerSpellsRequest(req); err != nil {
		return err
	}

	var lastErr error
	seenExisting := false
	seenChampionSelectionChanged := false
	seenChampionNotSelected := false
	seenSessionUnavailable := false

	for _, lockfilePath := range c.lockfileCandidates() {
		stat, err := os.Stat(lockfilePath)
		if err != nil || stat.IsDir() {
			continue
		}
		seenExisting = true

		info, err := c.readLockfile(lockfilePath)
		if err != nil {
			lastErr = fmt.Errorf("lockfile %q: %w", lockfilePath, err)
			continue
		}

		session, err := c.fetchChampSelectSession(ctx, info)
		if err != nil {
			if errors.Is(err, ErrChampSelectUnavailable) {
				seenSessionUnavailable = true
			}
			lastErr = fmt.Errorf("lockfile %q: %w", lockfilePath, err)
			continue
		}

		member, err := localPlayerFromSession(session)
		if err != nil {
			if errors.Is(err, ErrChampSelectUnavailable) {
				seenSessionUnavailable = true
			}
			lastErr = fmt.Errorf("lockfile %q: %w", lockfilePath, err)
			continue
		}

		if member.ChampionID <= 0 {
			seenChampionNotSelected = true
			lastErr = fmt.Errorf("lockfile %q: %w", lockfilePath, ErrChampionNotSelected)
			continue
		}

		if member.ChampionID != req.ChampionID {
			seenChampionSelectionChanged = true
			lastErr = fmt.Errorf("lockfile %q: %w: expected championId %d, got %d", lockfilePath, ErrChampionSelectionChanged, req.ChampionID, member.ChampionID)
			continue
		}

		spell1ID, spell2ID := preserveFlashSlot(req.SpellIDs, member.Spell1ID, member.Spell2ID)
		if err := c.patchMySelectionSpells(ctx, info, spell1ID, spell2ID); err != nil {
			if errors.Is(err, ErrChampSelectUnavailable) {
				seenSessionUnavailable = true
			}
			lastErr = fmt.Errorf("lockfile %q: %w", lockfilePath, err)
			continue
		}

		return nil
	}

	if seenChampionSelectionChanged {
		return withLastCandidateError(ErrChampionSelectionChanged, lastErr)
	}
	if seenChampionNotSelected {
		return withLastCandidateError(ErrChampionNotSelected, lastErr)
	}
	if seenSessionUnavailable {
		return withLastCandidateError(ErrChampSelectUnavailable, lastErr)
	}
	if !seenExisting {
		return ErrLockfileNotFound
	}
	return withLastCandidateError(ErrSummonerSpellsApplyFailed, lastErr)
}

func validateApplySummonerSpellsRequest(req ports.ApplySummonerSpellsRequest) error {
	if req.ChampionID <= 0 {
		return fmt.Errorf("%w: championID must be > 0", ErrInvalidSummonerSpellsRequest)
	}
	if len(req.SpellIDs) != 2 {
		return fmt.Errorf("%w: exactly 2 spell IDs are required", ErrInvalidSummonerSpellsRequest)
	}
	if req.SpellIDs[0] <= 0 || req.SpellIDs[1] <= 0 {
		return fmt.Errorf("%w: spell IDs must be > 0", ErrInvalidSummonerSpellsRequest)
	}
	if req.SpellIDs[0] == req.SpellIDs[1] {
		return fmt.Errorf("%w: spell IDs must be distinct", ErrInvalidSummonerSpellsRequest)
	}
	return nil
}

func preserveFlashSlot(spellIDs []int, currentSpell1ID int, currentSpell2ID int) (int, int) {
	spell1ID := spellIDs[0]
	spell2ID := spellIDs[1]

	containsFlash := spell1ID == 4 || spell2ID == 4
	if !containsFlash {
		return spell1ID, spell2ID
	}

	otherSpellID := spell1ID
	if spell1ID == 4 {
		otherSpellID = spell2ID
	}

	switch {
	case currentSpell1ID == 4:
		return 4, otherSpellID
	case currentSpell2ID == 4:
		return otherSpellID, 4
	default:
		return spell1ID, spell2ID
	}
}

func (c *Client) patchMySelectionSpells(ctx context.Context, info lockfileInfo, spell1ID int, spell2ID int) error {
	payload, err := json.Marshal(champSelectMySelectionPatch{
		Spell1ID: spell1ID,
		Spell2ID: spell2ID,
	})
	if err != nil {
		return fmt.Errorf("%w: encode payload: %v", ErrSummonerSpellsApplyFailed, err)
	}

	url := fmt.Sprintf("%s://127.0.0.1:%d/lol-champ-select/v1/session/my-selection", info.Protocol, info.Port)
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("%w: build request: %v", ErrSummonerSpellsApplyFailed, err)
	}

	applyLCUHeaders(req, info.Password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient(info.Protocol).Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrSummonerSpellsApplyFailed, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusNoContent:
		return nil
	case http.StatusNotFound:
		return ErrChampSelectUnavailable
	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		if len(body) == 0 {
			return fmt.Errorf("%w: status %d", ErrSummonerSpellsApplyFailed, resp.StatusCode)
		}
		return fmt.Errorf("%w: status %d: %s", ErrSummonerSpellsApplyFailed, resp.StatusCode, strings.TrimSpace(string(body)))
	}
}

type StubClient struct {
	DetectedSelection  ports.DetectedSelection
	DetectErr          error
	ItemSetCalls       []ports.ApplyItemSetRequest
	RunePageCalls      []ports.ApplyRunePageRequest
	SummonerSpellCalls []ports.ApplySummonerSpellsRequest
	ItemSetErr         error
	RunePageErr        error
	SummonerSpellsErr  error
}

func (c *StubClient) DetectSelection(ctx context.Context) (ports.DetectedSelection, error) {
	_ = ctx
	if c.DetectErr != nil {
		return ports.DetectedSelection{}, c.DetectErr
	}

	return c.DetectedSelection, nil
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
