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

func parseProcessConnection(rawPort, rawPassword, rawProtocol string) (connectionInfo, error) {
	return parseConnection(rawPort, rawPassword, rawProtocol, protocolFallback)
}

func parseLockfileConnection(rawPort, rawPassword, rawProtocol string) (connectionInfo, error) {
	return parseConnection(rawPort, rawPassword, rawProtocol, protocolStrict)
}

func parseConnection(rawPort, rawPassword, rawProtocol string, mode protocolMode) (connectionInfo, error) {
	port, err := parsePositivePort(rawPort)
	if err != nil {
		return connectionInfo{}, err
	}

	password := strings.TrimSpace(rawPassword)
	if password == "" {
		return connectionInfo{}, errMissingPassword
	}

	protocol := strings.ToLower(strings.TrimSpace(rawProtocol))
	switch mode {
	case protocolFallback:
		protocol = normalizeProtocol(protocol)
	case protocolStrict:
		if protocol != "https" && protocol != "http" {
			return connectionInfo{}, fmt.Errorf("%w %q", errUnsupportedProtocol, protocol)
		}
	default:
		return connectionInfo{}, errInvalidProtocolMode
	}

	return connectionInfo{
		Port:     port,
		Password: password,
		Protocol: protocol,
	}, nil
}

func normalizeProtocol(raw string) string {
	protocol := strings.ToLower(strings.TrimSpace(raw))
	switch protocol {
	case "http", "https":
		return protocol
	default:
		return defaultLCUAppProtocol
	}
}

func parseLockfile(raw []byte) (connectionInfo, error) {
	parts := strings.Split(strings.TrimSpace(string(raw)), ":")
	if len(parts) != 5 {
		return connectionInfo{}, fmt.Errorf("%w: expected 5 fields", ErrInvalidLockfile)
	}

	info, err := parseLockfileConnection(parts[2], parts[3], parts[4])
	if err != nil {
		return connectionInfo{}, fmt.Errorf("%w: %v", ErrInvalidLockfile, err)
	}

	return info, nil
}
