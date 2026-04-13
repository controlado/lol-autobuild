package lcu

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/shirou/gopsutil/v4/process"
)

type clientConnectionCandidate struct {
	source  string
	resolve func() (lockfileInfo, error)
}

func (candidate clientConnectionCandidate) label() string {
	source := strings.TrimSpace(candidate.source)

	if source == "" {
		return "unknown"
	}

	return source
}

func staticConnectionCandidate(source string, info lockfileInfo) clientConnectionCandidate {
	return clientConnectionCandidate{
		source: source,
		resolve: func() (lockfileInfo, error) {
			return info, nil
		},
	}
}

func lockfileConnectionCandidate(source string, lockfilePath string, readLockfile func(string) (lockfileInfo, error)) clientConnectionCandidate {
	return clientConnectionCandidate{
		source: source,
		resolve: func() (lockfileInfo, error) {
			return readLockfile(lockfilePath)
		},
	}
}

func (c *Client) connectionCandidates(ctx context.Context) []clientConnectionCandidate {
	raw := make([]clientConnectionCandidate, 0, 4)
	if c.discoverOpenClientConnections != nil {
		raw = append(raw, c.discoverOpenClientConnections(ctx)...)
	}

	if c.LockfilePath != "" {
		lockfilePath := filepath.Clean(strings.TrimSpace(c.LockfilePath))
		if lockfilePath != "" && lockfilePath != "." {
			raw = append(raw, lockfileConnectionCandidate("config:lcu.lockfile_path", lockfilePath, c.readLockfile))
		}
	}

	seen := make(map[string]struct{}, len(raw))
	out := make([]clientConnectionCandidate, 0, len(raw))
	for _, candidate := range raw {
		if candidate.resolve == nil {
			continue
		}
		key := candidate.label()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, candidate)
	}

	return out
}

func discoverOpenClientConnections(ctx context.Context) []clientConnectionCandidate {
	processes, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return nil
	}

	seen := make(map[string]struct{})
	out := make([]clientConnectionCandidate, 0, len(processes))
	for _, proc := range processes {
		if proc == nil {
			continue
		}

		args, err := proc.CmdlineSliceWithContext(ctx)
		if err != nil {
			continue
		}

		info, err := parseLCUProcessArgs(args)
		if err != nil {
			continue
		}

		connectionKey := fmt.Sprintf("%s:%d:%s", info.Protocol, info.Port, info.Password)
		if _, ok := seen[connectionKey]; ok {
			continue
		}
		seen[connectionKey] = struct{}{}

		out = append(out, staticConnectionCandidate(fmt.Sprintf("process:%d", proc.Pid), info))
	}

	return out
}

func parseLCUProcessArgs(args []string) (lockfileInfo, error) {
	values := processArgValues(args)

	rawPort := strings.TrimSpace(values["app-port"])
	if rawPort == "" {
		return lockfileInfo{}, errors.New("missing --app-port")
	}

	info, err := parseConnectionInfoFromProcessArgs(rawPort, values["remoting-auth-token"], values["app-protocol"])
	if err != nil {
		switch {
		case errors.Is(err, errInvalidPort):
			return lockfileInfo{}, errors.New("invalid --app-port")
		case errors.Is(err, errMissingPassword):
			return lockfileInfo{}, errors.New("missing --remoting-auth-token")
		default:
			return lockfileInfo{}, err
		}
	}

	return info, nil
}

func processArgValues(args []string) map[string]string {
	values := make(map[string]string, len(args))

	for i := 0; i < len(args); i++ {
		raw := strings.TrimSpace(args[i])
		if !strings.HasPrefix(raw, "--") {
			continue
		}

		raw = strings.TrimPrefix(raw, "--")
		if raw == "" {
			continue
		}

		key, value, hasSeparator := strings.Cut(raw, "=")
		if !hasSeparator && i+1 < len(args) {
			nextArg := strings.TrimSpace(args[i+1])
			if nextArg != "" && !strings.HasPrefix(nextArg, "--") {
				value = nextArg
				i++
			}
		}

		key = strings.ToLower(strings.TrimSpace(key))
		if key == "" {
			continue
		}
		values[key] = strings.TrimSpace(strings.Trim(value, `"'`))
	}

	return values
}
