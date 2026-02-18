package lib

import (
	"net/http"

	"github.com/TecharoHQ/anubis/lib/policy"
)

// ValidationDecision tells Anubis how to handle access validation.
type ValidationDecision int

const (
	// ValidationSkip tells Anubis to use the built-in JWT validation flow.
	ValidationSkip ValidationDecision = iota
	// ValidationAllow tells Anubis to allow the request.
	ValidationAllow
	// ValidationDeny tells Anubis to deny the request and render auth.
	ValidationDeny
)

// AuthHooks provides optional overrides for authentication and validation flows.
// If nil, Anubis uses the built-in PoW + JWT flow.
type AuthHooks interface {
	// AuthMode returns the selected auth mode (e.g. "pow", "password").
	AuthMode() string

	// ValidMode returns the selected validation mode (e.g. "jwt", "whitelist").
	ValidMode() string

	// ValidateRequest is called before JWT validation for challenge rules.
	// Return ValidationSkip to use the built-in JWT flow.
	ValidateRequest(r *http.Request, cr policy.CheckResult, rule *policy.Bot) (ValidationDecision, error)

	// RenderAuthPage is called when auth is required (challenge rule, not validated).
	// Return handled=true if a response was written and default rendering should be skipped.
	RenderAuthPage(w http.ResponseWriter, r *http.Request, cr policy.CheckResult, rule *policy.Bot, returnHTTPStatusOnly bool) (handled bool, err error)

	// HandleAuthSuccess is called after a challenge/password passes, before JWT issuance.
	// Return handled=true if validation was issued and default JWT issuance should be skipped.
	HandleAuthSuccess(w http.ResponseWriter, r *http.Request, cr policy.CheckResult, rule *policy.Bot) (handled bool, err error)
}
