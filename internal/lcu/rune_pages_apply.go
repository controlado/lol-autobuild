package lcu

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/controlado/lol-autobuild/internal/autobuild/domain"
	"github.com/controlado/lol-autobuild/internal/autobuild/runes"
)

const (
	runePagePerkCount      = 9
	runePageRestoreTimeout = 3 * time.Second
)

func (c *Client) ApplyRunePage(ctx context.Context, req domain.ApplyRunePageRequest) error {
	if !c.Enabled {
		return ErrNotConfigured
	}

	if req.DryRun {
		return nil
	}

	payload, err := validateRunePageApplyRequest(req)
	if err != nil {
		return err
	}

	var (
		attempt          = newConnectionAttempt()
		candidateHandler = func(info connectionInfo, candidateLabel string) (success bool) {
			selection := c.validatedLocalPlayerSelection(ctx, info, req.ChampionID)
			if selection.err != nil {
				attempt.observe(candidateLabel, selection.baseErr, selection.err)
				return false
			}

			currentPage, hasCurrentPage, err := c.fetchCurrentRunePage(ctx, info)
			if err != nil {
				attempt.observe(candidateLabel, ErrRunePageApplyFailed, err)
				return false
			}

			if hasCurrentPage && isManagedRunePage(currentPage) {
				if err := c.replaceManagedRunePage(ctx, info, currentPage, payload); err != nil {
					attempt.observe(candidateLabel, ErrRunePageApplyFailed, err)
					return false
				}
				return true
			}

			pages, err := c.fetchRunePages(ctx, info)
			if err != nil {
				attempt.observe(candidateLabel, ErrRunePageApplyFailed, err)
				return false
			}

			var currentPageID int
			if hasCurrentPage {
				currentPageID = currentPage.ID
			}

			if managedPage, ok := reusableManagedRunePage(pages, currentPageID); ok {
				if err := c.replaceManagedRunePage(ctx, info, managedPage, payload); err != nil {
					attempt.observe(candidateLabel, ErrRunePageApplyFailed, err)
					return false
				}
				return true
			}

			if err := c.createRunePage(ctx, info, payload); err != nil {
				attempt.observe(candidateLabel, ErrRunePageApplyFailed, err)
				return false
			}

			return true
		}
	)

	if success, err := c.forEachCandidate(ctx, attempt, candidateHandler); err != nil {
		return err
	} else if success {
		return nil
	}

	return attempt.finish(
		ErrRunePageApplyFailed,
		ErrChampionSelectionChanged,
		ErrChampionNotSelected,
		ErrChampSelectUnavailable,
	)
}

func (c *Client) replaceManagedRunePage(ctx context.Context, info connectionInfo, managedPage runePage, payload runePageCreateRequest) error {
	if managedPage.ID <= 0 {
		return fmt.Errorf("%w: managed rune page id must be > 0", ErrRunePageApplyFailed)
	}
	if !managedPage.IsDeletable {
		return fmt.Errorf("%w: managed rune page %d is not deletable", ErrRunePageApplyFailed, managedPage.ID)
	}

	if err := c.deleteRunePage(ctx, info, managedPage.ID); err != nil {
		return err
	}

	if err := c.createRunePage(ctx, info, payload); err != nil {
		restorePayload := runePageCreateRequestFromPage(managedPage)

		restoreCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), runePageRestoreTimeout)
		defer cancel()

		restoreErr := c.createRunePage(restoreCtx, info, restorePayload)
		if restoreErr != nil {
			return fmt.Errorf("%w; restore previous rune page failed: %v", err, restoreErr)
		}
		return fmt.Errorf("%w; previous rune page restored", err)
	}

	return nil
}

func reusableManagedRunePage(pages []runePage, currentPageID int) (runePage, bool) {
	for _, page := range pages {
		if page.ID <= 0 || page.ID == currentPageID || !page.IsDeletable || !isManagedRunePage(page) {
			continue
		}
		return page, true
	}
	return runePage{}, false
}

func validateRunePageApplyRequest(req domain.ApplyRunePageRequest) (runePageCreateRequest, error) {
	if req.ChampionID <= 0 {
		return runePageCreateRequest{}, fmt.Errorf("%w: championID must be > 0", ErrInvalidRunePageRequest)
	}
	if !req.Position.IsValid() {
		return runePageCreateRequest{}, fmt.Errorf("%w: invalid position %q", ErrInvalidRunePageRequest, req.Position)
	}
	if !runes.IsStyle(req.Page.PrimaryStyleID) {
		return runePageCreateRequest{}, fmt.Errorf("%w: invalid primaryStyleID %d", ErrInvalidRunePageRequest, req.Page.PrimaryStyleID)
	}
	if !runes.IsStyle(req.Page.SubStyleID) {
		return runePageCreateRequest{}, fmt.Errorf("%w: invalid subStyleID %d", ErrInvalidRunePageRequest, req.Page.SubStyleID)
	}
	if req.Page.PrimaryStyleID == req.Page.SubStyleID {
		return runePageCreateRequest{}, fmt.Errorf("%w: primaryStyleID and subStyleID must be distinct", ErrInvalidRunePageRequest)
	}
	if len(req.Page.SelectedPerkIDs) != runePagePerkCount {
		return runePageCreateRequest{}, fmt.Errorf("%w: exactly %d selected perk IDs are required", ErrInvalidRunePageRequest, runePagePerkCount)
	}

	selectedPerkIDs := make([]int, 0, len(req.Page.SelectedPerkIDs))
	for _, perkID := range req.Page.SelectedPerkIDs {
		if perkID <= 0 {
			return runePageCreateRequest{}, fmt.Errorf("%w: selected perk IDs must be > 0", ErrInvalidRunePageRequest)
		}
		selectedPerkIDs = append(selectedPerkIDs, perkID)
	}

	return runePageCreateRequest{
		Name:            managedRunePageTitle(req),
		PrimaryStyleID:  req.Page.PrimaryStyleID,
		SubStyleID:      req.Page.SubStyleID,
		SelectedPerkIDs: selectedPerkIDs,
		Current:         true,
	}, nil
}

func runePageCreateRequestFromPage(page runePage) runePageCreateRequest {
	return runePageCreateRequest{
		Name:            page.Name,
		PrimaryStyleID:  page.PrimaryStyleID,
		SubStyleID:      page.SubStyleID,
		SelectedPerkIDs: append([]int(nil), page.SelectedPerkIDs...),
		Current:         page.Current,
	}
}

func managedRunePageTitle(req domain.ApplyRunePageRequest) string {
	return managedResourceTitle(req.Position, req.ChampionID, req.ChampionName)
}

func isManagedRunePage(page runePage) bool {
	name := strings.TrimSpace(page.Name)
	return strings.HasPrefix(name, managedNamePrefix+" ") || strings.HasPrefix(name, legacyAutoBuildPrefix)
}
