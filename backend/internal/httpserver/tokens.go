package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/yaremam/opusflow/backend/internal/auth"
)

// tokenResponse is one row of the "Paired Devices" list — never the
// token hash or (except createTokenResponse below) the plaintext.
type tokenResponse struct {
	ID         int64   `json:"id"`
	Name       string  `json:"name"`
	CreatedAt  string  `json:"createdAt"`
	LastUsedAt *string `json:"lastUsedAt"`
}

func toTokenResponse(t auth.Token) tokenResponse {
	resp := tokenResponse{ID: t.ID, Name: t.Name, CreatedAt: t.CreatedAt.Format(timeFormat)}
	if t.LastUsedAt != nil {
		s := t.LastUsedAt.Format(timeFormat)
		resp.LastUsedAt = &s
	}
	return resp
}

const timeFormat = "2006-01-02T15:04:05Z07:00"

// createTokenResponse is the one and only response that ever carries a
// token's plaintext — AC-5's "display it once" (text + QR code on the
// frontend); nothing after this response can ever retrieve it again.
type createTokenResponse struct {
	Token string `json:"token"`
	tokenResponse
}

// handleCreateToken is Settings' "Generate new pairing token" (AC-5).
func handleCreateToken(svc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
		if req.Name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}

		plaintext, tok, err := svc.CreateToken(r.Context(), req.Name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusCreated, createTokenResponse{Token: plaintext, tokenResponse: toTokenResponse(tok)})
	}
}

// handleListTokens backs the "Paired Devices" list (AC-5).
func handleListTokens(svc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tokens, err := svc.ListTokens(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		resp := make([]tokenResponse, len(tokens))
		for i, t := range tokens {
			resp[i] = toTokenResponse(t)
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// handleDeleteToken is "Revoke" — a request bearing this token starts
// failing immediately afterward (AC-5), including a request bearing it
// concurrently with this one, since ValidateAndTouch and this DELETE both
// go straight to the same row.
func handleDeleteToken(svc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid token id", http.StatusBadRequest)
			return
		}
		if err := svc.DeleteToken(r.Context(), id); err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, auth.ErrTokenNotFound) {
				status = http.StatusNotFound
			}
			http.Error(w, err.Error(), status)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
