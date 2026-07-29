package auth

import "context"

// Service is the seam httpserver's token endpoints sit behind — the one
// place that generates a plaintext token and hands it back exactly once,
// coordinating GenerateToken/HashToken with Store the same way
// library.Service coordinates its own Store.
type Service struct {
	store *Store
}

func NewService(store *Store) *Service {
	return &Service{store: store}
}

// CreateToken generates a new token, stores only its hash, and returns
// the plaintext once — the caller (an HTTP handler, a QR code) is
// responsible for surfacing it, since nothing here can ever reproduce it
// again.
func (s *Service) CreateToken(ctx context.Context, name string) (plaintext string, tok Token, err error) {
	plaintext, err = GenerateToken()
	if err != nil {
		return "", Token{}, err
	}
	tok, err = s.store.Create(ctx, name, HashToken(plaintext))
	if err != nil {
		return "", Token{}, err
	}
	return plaintext, tok, nil
}

func (s *Service) ListTokens(ctx context.Context) ([]Token, error) {
	return s.store.List(ctx)
}

// DeleteToken revokes a token — TDR 024: nothing enforces tokens
// anymore, so deleting even the last one can't lock anyone out of
// anything (see issue #59's now-removed guard).
func (s *Service) DeleteToken(ctx context.Context, id int64) error {
	return s.store.Delete(ctx, id)
}
