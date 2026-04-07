package secrets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"

	"github.com/controlado/lol-autobuild/internal/ports"
)

const tokenUsername = "coachless_tokens"

type KeyringStore struct {
	Service string
}

func NewKeyringStore(service string) *KeyringStore {
	return &KeyringStore{Service: service}
}

func (s *KeyringStore) ReadTokens(ctx context.Context) (ports.TokenPair, error) {
	_ = ctx

	raw, err := keyring.Get(s.Service, tokenUsername)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return ports.TokenPair{}, fmt.Errorf("tokens not found: %w", err)
		}

		return ports.TokenPair{}, fmt.Errorf("keyring get: %w", err)
	}

	var pair ports.TokenPair
	if err := json.Unmarshal([]byte(raw), &pair); err != nil {
		return ports.TokenPair{}, fmt.Errorf("decode tokens: %w", err)
	}

	return pair, nil
}

func (s *KeyringStore) WriteTokens(ctx context.Context, pair ports.TokenPair) error {
	_ = ctx

	raw, err := json.Marshal(pair)
	if err != nil {
		return fmt.Errorf("encode tokens: %w", err)
	}

	if err := keyring.Set(s.Service, tokenUsername, string(raw)); err != nil {
		return fmt.Errorf("keyring set: %w", err)
	}

	return nil
}

func (s *KeyringStore) ClearTokens(ctx context.Context) error {
	_ = ctx

	if err := keyring.Delete(s.Service, tokenUsername); err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return nil
		}

		return fmt.Errorf("keyring delete: %w", err)
	}

	return nil
}
