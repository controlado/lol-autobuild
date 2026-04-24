package lcu

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shirou/gopsutil/v4/process"
)

func staticCandidate(source string, info connectionInfo) connectionCandidate {
	return connectionCandidate{
		source: source,
		resolve: func() (connectionInfo, error) {
			return info, nil
		},
	}
}

func lockfileCandidate(source string, lockfilePath string) connectionCandidate {
	return connectionCandidate{
		source: source,
		resolve: func() (connectionInfo, error) {
			return readLockfile(lockfilePath)
		},
	}
}

func processLockfileCandidate(source string, exePath string) (connectionCandidate, bool) {
	exePath = strings.TrimSpace(exePath)
	if exePath == "" {
		return connectionCandidate{}, false
	}

	dir := filepath.Dir(filepath.Clean(exePath))
	if dir == "" || dir == "." {
		return connectionCandidate{}, false
	}

	return lockfileCandidate(source+":lockfile", filepath.Join(dir, "lockfile")), true
}

func (c *Client) connectionCandidates(ctx context.Context) []connectionCandidate {
	raw := make([]connectionCandidate, 0, 4)
	if c.discoverProcessConnections != nil {
		raw = append(raw, c.discoverProcessConnections(ctx)...)
	}

	if c.LockfilePath != "" {
		lockfilePath := filepath.Clean(strings.TrimSpace(c.LockfilePath))
		if lockfilePath != "" && lockfilePath != "." {
			raw = append(raw, lockfileCandidate("config:lcu.lockfile_path", lockfilePath))
		}
	}

	seen := make(map[string]struct{}, len(raw))
	out := make([]connectionCandidate, 0, len(raw))
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

func discoverProcessConnections(ctx context.Context) []connectionCandidate {
	processes, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return nil
	}

	seen := make(map[string]struct{})
	out := make([]connectionCandidate, 0, len(processes)*2)
	for _, proc := range processes {
		if proc == nil {
			continue
		}

		args, err := proc.CmdlineSliceWithContext(ctx)
		if err != nil {
			continue
		}

		info, err := parseProcessArgs(args)
		if err != nil {
			continue
		}

		connectionKey := fmt.Sprintf("%s:%d:%s", info.Protocol, info.Port, info.Password)
		if _, ok := seen[connectionKey]; ok {
			continue
		}
		seen[connectionKey] = struct{}{}

		source := fmt.Sprintf("process:%d", proc.Pid)
		out = append(out, staticCandidate(source, info))

		exePath, err := proc.ExeWithContext(ctx)
		if err != nil {
			continue
		}
		if candidate, ok := processLockfileCandidate(source, exePath); ok {
			out = append(out, candidate)
		}
	}

	return out
}

func parseProcessArgs(args []string) (connectionInfo, error) {
	values := parseArgValues(args)

	rawPort := strings.TrimSpace(values["app-port"])
	if rawPort == "" {
		return connectionInfo{}, errors.New("missing --app-port")
	}

	info, err := parseProcessConnection(rawPort, values["remoting-auth-token"], values["app-protocol"])
	if err != nil {
		switch {
		case errors.Is(err, errInvalidPort):
			return connectionInfo{}, errors.New("invalid --app-port")
		case errors.Is(err, errMissingPassword):
			return connectionInfo{}, errors.New("missing --remoting-auth-token")
		default:
			return connectionInfo{}, err
		}
	}

	return info, nil
}

func parseArgValues(args []string) map[string]string {
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

func readLockfile(path string) (connectionInfo, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return connectionInfo{}, fmt.Errorf("%w: %v", ErrLockfileNotFound, err)
	}

	return parseLockfile(raw)
}
