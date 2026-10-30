package lib

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/TecharoHQ/anubis"
	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/TecharoHQ/anubis/lib/policy"
	"github.com/golang-jwt/jwt/v5"
)

// stubPasswordHooks drives the password-mode branch of maybeReverseProxy without
// depending on the cust package.
type stubPasswordHooks struct {
	validMode string
}

func (s stubPasswordHooks) AuthMode() string  { return "password" }
func (s stubPasswordHooks) ValidMode() string { return s.validMode }

func (s stubPasswordHooks) ValidateRequest(*http.Request, policy.CheckResult, *policy.Bot) (ValidationDecision, error) {
	return ValidationDeny, nil
}

func (s stubPasswordHooks) RenderAuthPage(w http.ResponseWriter, _ *http.Request, _ policy.CheckResult, _ *policy.Bot, _ bool) (bool, error) {
	w.WriteHeader(http.StatusUnauthorized)
	return true, nil
}

func (s stubPasswordHooks) HandleAuthSuccess(http.ResponseWriter, *http.Request, policy.CheckResult, *policy.Bot) (bool, error) {
	return true, nil
}

// TestPasswordModeAcceptsIssuedCookie guards against password mode never becoming
// authenticated. SetCookie appends a suffix derived from the cookie settings when
// DynamicCookieSuffix is on (the default), so the validation side has to read the
// cookie back through the same accessor instead of the bare name.
func TestPasswordModeAcceptsIssuedCookie(t *testing.T) {
	for _, dynamicSuffix := range []bool{true, false} {
		name := "dynamic-suffix-off"
		if dynamicSuffix {
			name = "dynamic-suffix-on"
		}
		t.Run(name, func(t *testing.T) {
			srv := spawnAnubis(t, Options{
				Next:                  http.NewServeMux(),
				Policy:                loadPolicies(t, "testdata/zero_difficulty.yaml", 0),
				AuthHooks:             stubPasswordHooks{validMode: "jwt"},
				DefaultFallbackAction: config.RuleAllow,
				DynamicCookieSuffix:   dynamicSuffix,
				CookieExpiration:      time.Hour,
			})

			token, err := srv.signJWT(jwt.MapClaims{
				"challenge":  "password",
				"method":     "password",
				"policyRule": "irrelevant-for-password-mode",
				"action":     string(config.RuleAllow),
			})
			if err != nil {
				t.Fatalf("can't sign JWT: %v", err)
			}

			rec := httptest.NewRecorder()
			srv.SetCookie(rec, CookieOpts{Host: "example.com", Path: "/", Value: token})

			cookies := rec.Result().Cookies()
			if len(cookies) == 0 {
				t.Fatal("no auth cookie was issued")
			}
			if dynamicSuffix && cookies[0].Name == anubis.CookieName {
				t.Fatalf("expected a suffixed cookie name, got bare %q", cookies[0].Name)
			}

			req := httptest.NewRequest(http.MethodGet, "https://example.com/", nil)
			req.Host = "example.com"
			req.Header.Set("X-Real-IP", "203.0.113.4")
			for _, c := range cookies {
				req.AddCookie(c)
			}

			ok, err := srv.isPasswordModeAuthenticated(req, policy.CheckResult{}, nil)
			if err != nil {
				t.Fatalf("auth check returned error: %v", err)
			}
			if !ok {
				t.Fatalf("valid password-mode cookie was rejected (cookie name %q)", cookies[0].Name)
			}
		})
	}
}
