package lcu

import (
	"context"
	"errors"
	"fmt"
)

type candidateHandler func(info connectionInfo, candidateLabel string) (shouldTerminate bool)

func (c *Client) ForEachCandidate(ctx context.Context, attempt *connectionAttempt, handler candidateHandler) (success bool, ctxErr error) {
	for _, candidate := range c.candidates(ctx) {
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
		attempt.markResolvableCandidate()

		if handler(info, candidateLabel) {
			return true, ctxErr
		}
	}

	return false, ctxErr
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
	ca.lastErr = err

	if base != nil {
		ca.byBase[base] = detail
	}
}

func (ca *connectionAttempt) finish(defaultErr error, priority ...error) error {
	for _, base := range priority {
		if detail, ok := ca.byBase[base]; ok {
			return fmt.Errorf("%w: %w", base, detail)
		}
	}

	if !ca.seenResolvableCandidate {
		return ErrLockfileNotFound
	}

	if ca.lastErr != nil {
		return fmt.Errorf("%w: %w", defaultErr, ca.lastErr)
	}

	return defaultErr
}
