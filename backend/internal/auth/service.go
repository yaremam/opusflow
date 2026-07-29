package auth

import (
	"context"
	"errors"
)

// ErrLastToken is returned by DeleteToken when id is the only token left —
// deleting it would lock every device, including whoever's making this
// request, out of the entire app with no way back in short of a database
// edit (issue #59).
var ErrLastToken = errors.New("cannot delete the last remaining pairing token")

// Service is the seam httpserver's token endpoints and cmd/server's
// bootstrap step both sit behind — the one place that generates a
// plaintext token and hands it back exactly once, coordinating GenerateToken/
// HashToken with Store the same way library.Service coordinates its own
// Store.
type Service struct {
	store *Store
}

func NewService(store *Store) *Service {
	return &Service{store: store}
}

// CreateToken generates a new token, stores only its hash, and returns
// the plaintext once — the caller (an HTTP handler, a QR code, Bootstrap's
// file) is responsible for surfacing it, since nothing here can ever
// reproduce it again.
//
// The first token any install ever creates — through this method, not
// just through Bootstrap's file-writing path — is what flips the
// bootstrap marker (see Middleware): if a request to POST /api/tokens
// itself raced Bootstrap and created the very first token, that has to
// count too, or a later "delete my only token" would incorrectly reopen
// the API instead of failing closed.
func (s *Service) CreateToken(ctx context.Context, name string) (plaintext string, tok Token, err error) {
	plaintext, err = GenerateToken()
	if err != nil {
		return "", Token{}, err
	}
	tok, err = s.store.Create(ctx, name, HashToken(plaintext))
	if err != nil {
		return "", Token{}, err
	}

	bootstrapped, err := s.store.HasBootstrapped(ctx)
	if err != nil {
		return "", Token{}, err
	}
	if !bootstrapped {
		if err := s.store.MarkBootstrapped(ctx); err != nil {
			return "", Token{}, err
		}
	}

	return plaintext, tok, nil
}

func (s *Service) ListTokens(ctx context.Context) ([]Token, error) {
	return s.store.List(ctx)
}

func (s *Service) DeleteToken(ctx context.Context, id int64) error {
	count, err := s.store.Count(ctx)
	if err != nil {
		return err
	}
	if count <= 1 {
		return ErrLastToken
	}
	return s.store.Delete(ctx, id)
}
