package lcu

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type protocolMode uint8

const (
	protocolFallback protocolMode = iota
	protocolStrict
)

const (
	defaultLCUAppProtocol = "https"
)

var (
	errInvalidPort         = errors.New("invalid port")
	errMissingPassword     = errors.New("missing password")
	errUnsupportedProtocol = errors.New("unsupported protocol")
	errInvalidProtocolMode = errors.New("invalid protocol mode")
)

func parsePositivePort(raw string) (int, error) {
	port, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || port <= 0 {
		return 0, errInvalidPort
	}

	return port, nil
}

func parseConnectionInfoFromProcessArgs(rawPort, rawPassword, rawProtocol string) (lockfileInfo, error) {
	return parseConnectionInfo(rawPort, rawPassword, rawProtocol, protocolFallback)
}

func parseConnectionInfoFromLockfile(rawPort, rawPassword, rawProtocol string) (lockfileInfo, error) {
	return parseConnectionInfo(rawPort, rawPassword, rawProtocol, protocolStrict)
}

func parseConnectionInfo(rawPort, rawPassword, rawProtocol string, mode protocolMode) (lockfileInfo, error) {
	port, err := parsePositivePort(rawPort)
	if err != nil {
		return lockfileInfo{}, err
	}

	password := strings.TrimSpace(rawPassword)
	if password == "" {
		return lockfileInfo{}, errMissingPassword
	}

	protocol := strings.ToLower(strings.TrimSpace(rawProtocol))
	switch mode {
	case protocolFallback:
		protocol = normalizeLCUProtocol(protocol)
	case protocolStrict:
		if protocol != "https" && protocol != "http" {
			return lockfileInfo{}, fmt.Errorf("%w %q", errUnsupportedProtocol, protocol)
		}
	default:
		return lockfileInfo{}, errInvalidProtocolMode
	}

	return lockfileInfo{
		Port:     port,
		Password: password,
		Protocol: protocol,
	}, nil
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

func parseLockfile(raw []byte) (lockfileInfo, error) {
	parts := strings.Split(strings.TrimSpace(string(raw)), ":")
	if len(parts) != 5 {
		return lockfileInfo{}, fmt.Errorf("%w: expected 5 fields", ErrInvalidLockfile)
	}

	info, err := parseConnectionInfoFromLockfile(parts[2], parts[3], parts[4])
	if err != nil {
		return lockfileInfo{}, fmt.Errorf("%w: %v", ErrInvalidLockfile, err)
	}

	return info, nil
}
