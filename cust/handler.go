package cust

import "net/http"

type Handler struct {
	next  http.Handler
	hooks *Hooks
}

func NewHandler(next http.Handler, hooks *Hooks) http.Handler {
	if hooks == nil {
		return next
	}
	return &Handler{next: next, hooks: hooks}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.hooks != nil && h.hooks.cfg.AuthMode == AuthModePassword {
		if r.Method == http.MethodPost && r.URL.Path == h.hooks.passwordEndpointPath() {
			h.hooks.HandlePassword(w, r)
			return
		}
	}
	h.next.ServeHTTP(w, r)
}
