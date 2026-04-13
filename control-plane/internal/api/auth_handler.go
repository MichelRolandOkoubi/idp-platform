package api

import (
	"net/http"
	"time"
)

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := h.decode(r, &body); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid body")
		return
	}

	token, user, err := h.Auth.Login(body.Username, body.Password)
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"token":      token,
		"expires_at": time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		"user": map[string]any{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
			"roles":    user.Roles,
		},
	})
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusCreated, map[string]string{
		"message": "registration not implemented in demo mode",
	})
}
