package secrets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"

	"github.com/controlado/lol-autobuild/internal/autobuild/domain"
)

const tokenUsername = "coachless_tokens"

type KeyringStore struct {
	Service string
}

func NewKeyringStore(service string) *KeyringStore {
	return &KeyringStore{Service: service}
}

func (s *KeyringStore) ReadTokens(_ context.Context) (domain.TokenPair, error) {
	raw, err := keyring.Get(s.Service, tokenUsername)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return domain.TokenPair{}, fmt.Errorf("%w: %v", domain.ErrTokensNotFound, err)
		}

		return domain.TokenPair{}, fmt.Errorf("keyring get: %w", err)
	}

	var pair domain.TokenPair
	if err := json.Unmarshal([]byte(raw), &pair); err != nil {
		return domain.TokenPair{}, fmt.Errorf("decode tokens: %w", err)
	}

	return pair, nil
}

func (s *KeyringStore) WriteTokens(_ context.Context, pair domain.TokenPair) error {
	raw, err := json.Marshal(pair)
	if err != nil {
		return fmt.Errorf("encode tokens: %w", err)
	}

	if err := keyring.Set(s.Service, tokenUsername, string(raw)); err != nil {
		return fmt.Errorf("keyring set: %w", err)
	}

	return nil
}

func (s *KeyringStore) ClearTokens(_ context.Context) error {
	if err := keyring.Delete(s.Service, tokenUsername); err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return nil
		}

		return fmt.Errorf("keyring delete: %w", err)
	}

	return nil
}
