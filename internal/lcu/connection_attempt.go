package lcu

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type candidateHandler func(info connectionInfo, candidateLabel string) (shouldTerminate bool)

func (c *Client) forEachCandidate(ctx context.Context, attempt *connectionAttempt, handler candidateHandler) (success bool, ctxErr error) {
	seenEndpoints := map[string]struct{}{}
	for _, candidate := range c.connectionCandidates(ctx) {
		if ctxErr = ctx.Err(); ctxErr != nil {
			return false, ctxErr
		}
		candidateLabel := candidate.label()

		info, err := candidate.resolve()
		if err != nil {
			if !errors.Is(err, ErrLockfileNotFound) {
				attempt.markResolvableCandidate()
			}
			attempt.observe(candidateLabel, ErrLockfileNotFound, err)
			continue
		}
		endpointKey := candidateEndpointKey(info)
		if _, ok := seenEndpoints[endpointKey]; ok {
			continue
		}
		seenEndpoints[endpointKey] = struct{}{}
		attempt.markResolvableCandidate()

		if handler(info, candidateLabel) {
			return true, ctxErr
		}
	}

	return false, ctxErr
}

func candidateEndpointKey(info connectionInfo) string {
	return fmt.Sprintf("%s:%d:%s", strings.ToLower(strings.TrimSpace(info.Protocol)), info.Port, info.Password)
}

type connectionAttempt struct {
	seenResolvableCandidate bool
	byBase                  map[error]error
	lastErr                 error
}

func newConnectionAttempt() *connectionAttempt {
	return &connectionAttempt{byBase: make(map[error]error)}
}

func (ca *connectionAttempt) markResolvableCandidate() {
	ca.seenResolvableCandidate = true
}

func (ca *connectionAttempt) observe(label string, base, err error) {
	if err == nil {
		return
	}

	detail := fmt.Errorf("candidate %q: %w", label, err)
	ca.lastErr = detail

	if base != nil {
		ca.byBase[base] = detail
	}
}

func (ca *connectionAttempt) finish(defaultErr error, priority ...error) error {
	for _, base := range priority {
		if detail, ok := ca.byBase[base]; ok {
			if errors.Is(detail, base) {
				return detail
			}
			return fmt.Errorf("%w: %w", base, detail)
		}
	}

	if !ca.seenResolvableCandidate {
		return ErrLockfileNotFound
	}

	if ca.lastErr != nil {
		if errors.Is(ca.lastErr, defaultErr) {
			return ca.lastErr
		}
		return fmt.Errorf("%w: %w", defaultErr, ca.lastErr)
	}

	return defaultErr
}
