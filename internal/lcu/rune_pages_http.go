package lcu

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/controlado/lol-autobuild/internal/autobuild/domain"
)

func (c *Client) fetchCurrentRunePage(ctx context.Context, info connectionInfo) (runePage, bool, error) {
	page, err := doJSON[runePage](ctx, c, info, http.MethodGet, currentRunePagePath, nil)
	if err != nil {
		if errors.Is(err, errHTTPNotFound) {
			return runePage{}, false, nil
		}
		return runePage{}, false, fmt.Errorf("%w: fetch current rune page: %v", ErrRunePageApplyFailed, err)
	}
	return page, true, nil
}

func (c *Client) fetchRunePages(ctx context.Context, info connectionInfo) ([]runePage, error) {
	pages, err := doJSON[[]runePage](ctx, c, info, http.MethodGet, runePagesPath, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: fetch rune pages: %v", ErrRunePageApplyFailed, err)
	}
	return pages, nil
}

func (c *Client) deleteRunePage(ctx context.Context, info connectionInfo, pageID int) error {
	endpoint := fmt.Sprintf(runePagePathFormat, pageID)
	if err := doRequest(ctx, c, info, http.MethodDelete, endpoint, nil); err != nil {
		return fmt.Errorf("%w: delete rune page: %v", ErrRunePageApplyFailed, err)
	}
	return nil
}

func (c *Client) createRunePage(ctx context.Context, info connectionInfo, payload runePageCreateRequest) error {
	if err := doRequest(ctx, c, info, http.MethodPost, runePagesPath, payload); err != nil {
		if isRunePageLimitReached(err) {
			return fmt.Errorf("%w: create rune page failed LCU validation: %w: %v", ErrRunePageApplyFailed, domain.ErrRunePageLimitReached, err)
		}
		return fmt.Errorf("%w: create rune page failed LCU validation: %v", ErrRunePageApplyFailed, err)
	}
	return nil
}

func isRunePageLimitReached(err error) bool {
	var statusErr *httpStatusError
	return errors.As(err, &statusErr) &&
		statusErr.StatusCode() == http.StatusBadRequest &&
		strings.Contains(statusErr.Body(), "Max pages reached")
}
