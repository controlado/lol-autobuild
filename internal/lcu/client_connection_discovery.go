package lcu

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v4/process"
)

const defaultLCUAppProtocol = "https"

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

	port, err := strconv.Atoi(rawPort)
	if err != nil || port <= 0 {
		return lockfileInfo{}, errors.New("invalid --app-port")
	}

	password := strings.TrimSpace(values["remoting-auth-token"])
	if password == "" {
		return lockfileInfo{}, errors.New("missing --remoting-auth-token")
	}

	return lockfileInfo{
		Port:     port,
		Password: password,
		Protocol: normalizeLCUProtocol(values["app-protocol"]),
	}, nil
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

		key := raw
		value := ""

		if idx := strings.Index(raw, "="); idx >= 0 {
			key = raw[:idx]
			value = raw[idx+1:]
		} else if i+1 < len(args) {
			next := strings.TrimSpace(args[i+1])
			if next != "" && !strings.HasPrefix(next, "--") {
				value = next
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

func normalizeLCUProtocol(raw string) string {
	protocol := strings.ToLower(strings.TrimSpace(raw))
	switch protocol {
	case "http", "https":
		return protocol
	default:
		return defaultLCUAppProtocol
	}
}
