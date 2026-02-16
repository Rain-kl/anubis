package cust

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/TecharoHQ/anubis"
	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/internal/glob"
	"github.com/TecharoHQ/anubis/lib"
	"github.com/TecharoHQ/anubis/lib/localization"
	"github.com/TecharoHQ/anubis/lib/policy"
	"github.com/TecharoHQ/anubis/lib/store"
	"github.com/TecharoHQ/anubis/web"
	"github.com/TecharoHQ/anubis/xess"
	"github.com/a-h/templ"
)

const (
	whitelistPrefix = "cust:whitelist:"
	attemptsPrefix  = "cust:pw-attempts:"
)
const (
	passwordDelayStep = 200 * time.Millisecond
	passwordDelayMax  = 2 * time.Second
)

type WhitelistEntry struct {
	AddedAt  time.Time `json:"addedAt"`
	LastSeen time.Time `json:"lastSeen"`
}

type PasswordAttempt struct {
	Count      int       `json:"count"`
	LastFailed time.Time `json:"lastFailed"`
}

type Hooks struct {
	cfg       Config
	server    *lib.Server
	logger    *slog.Logger
	whitelist store.JSON[WhitelistEntry]
	attempts  store.JSON[PasswordAttempt]
}

func NewHooks(cfg Config, st store.Interface, logger *slog.Logger) (*Hooks, error) {
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if st == nil {
		return nil, fmt.Errorf("store is nil")
	}
	if logger == nil {
		logger = slog.With("subsystem", "cust")
	}
	return &Hooks{
		cfg:       cfg,
		logger:    logger,
		whitelist: store.JSON[WhitelistEntry]{Underlying: st, Prefix: whitelistPrefix},
		attempts:  store.JSON[PasswordAttempt]{Underlying: st, Prefix: attemptsPrefix},
	}, nil
}

func (h *Hooks) AttachServer(s *lib.Server) {
	h.server = s
}

func (h *Hooks) ValidateRequest(r *http.Request, _ policy.CheckResult, _ *policy.Bot) (lib.ValidationDecision, error) {
	if h == nil || h.cfg.ValidMode != ValidModeWhitelist {
		return lib.ValidationSkip, nil
	}

	ip := realIP(r)
	if ip == "" {
		return lib.ValidationDeny, fmt.Errorf("missing or invalid X-Real-Ip header")
	}

	entry, err := h.whitelist.Get(r.Context(), ip)
	if err == nil {
		if h.cfg.WhitelistSliding {
			entry.LastSeen = time.Now()
			if err := h.whitelist.Set(r.Context(), ip, entry, h.cfg.WhitelistTTL); err != nil {
				h.logger.Error("failed to refresh whitelist entry", "ip", ip, "err", err)
			}
		}
		return lib.ValidationAllow, nil
	}

	if errors.Is(err, store.ErrNotFound) {
		return lib.ValidationDeny, nil
	}

	return lib.ValidationDeny, err
}

func (h *Hooks) RenderAuthPage(w http.ResponseWriter, r *http.Request, _ policy.CheckResult, _ *policy.Bot, returnHTTPStatusOnly bool) (bool, error) {
	if h == nil || h.cfg.AuthMode != AuthModePassword {
		return false, nil
	}

	localizer := localization.GetLocalizer(r)
	if returnHTTPStatusOnly {
		if h.cfg.PublicUrl == "" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(localizer.T("authorization_required")))
			return true, nil
		}

		redirectURL, err := h.constructRedirectURL(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return true, nil
		}
		http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
		return true, nil
	}

	redir := r.FormValue("redir")
	if redir == "" {
		redir = r.URL.RequestURI()
	}

	if blocked, _ := h.isBlocked(r.Context(), realIP(r)); blocked {
		h.renderBlockedPage(w, r, localizer)
		return true, nil
	}

	h.renderPasswordPage(w, r, localizer, redir, false, "")
	return true, nil
}

func (h *Hooks) HandleAuthSuccess(w http.ResponseWriter, r *http.Request, cr policy.CheckResult, rule *policy.Bot) (bool, error) {
	if h == nil {
		return false, nil
	}
	// In password mode, only the dedicated password endpoint can issue auth.
	// This prevents challenge success paths from minting auth cookies/whitelist entries.
	if h.cfg.AuthMode == AuthModePassword && !h.isPasswordAuthIssuanceRequest(r) {
		h.logger.Warn("blocked auth issuance from non-password endpoint", "method", r.Method, "path", r.URL.Path)
		return true, nil
	}

	switch h.cfg.ValidMode {
	case ValidModeWhitelist:
		ip := realIP(r)
		if ip == "" {
			return true, fmt.Errorf("missing or invalid X-Real-Ip header")
		}
		now := time.Now()
		entry := WhitelistEntry{
			AddedAt:  now,
			LastSeen: now,
		}
		if err := h.whitelist.Set(r.Context(), ip, entry, h.cfg.WhitelistTTL); err != nil {
			return true, err
		}
		return true, nil
	case ValidModeJWT:
		if h.cfg.AuthMode != AuthModePassword {
			return false, nil
		}
		if err := h.issueJWT(w, r, cr, rule); err != nil {
			return true, err
		}
		return true, nil
	default:
		return false, nil
	}
}

func (h *Hooks) isPasswordAuthIssuanceRequest(r *http.Request) bool {
	if r == nil {
		return false
	}
	return r.Method == http.MethodPost && r.URL.Path == h.passwordEndpointPath()
}

func (h *Hooks) HandlePassword(w http.ResponseWriter, r *http.Request) {
	localizer := localization.GetLocalizer(r)
	if h == nil || h.cfg.AuthMode != AuthModePassword {
		http.NotFound(w, r)
		return
	}
	if h.server == nil {
		http.Error(w, localizer.T("internal_server_error"), http.StatusInternalServerError)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, localizer.T("invalid_invocation"), http.StatusBadRequest)
		return
	}

	if blocked, _ := h.isBlocked(r.Context(), realIP(r)); blocked {
		h.renderBlockedPage(w, r, localizer)
		return
	}

	redir := r.FormValue("redir")
	if redir == "" {
		redir = "/"
	}

	redirURL, err := h.validateRedirect(r, redir)
	if err != nil {
		h.renderPasswordPage(w, r, localizer, redir, false, localizer.T("invalid_redirect"))
		return
	}

	ip := realIP(r)
	password := r.FormValue("password")
	if !h.passwordMatches(password) {
		attempts, blocked := h.recordFailure(r.Context(), ip)
		h.applyDelay(r.Context(), attempts)
		if blocked {
			h.logger.Warn("password auth blocked", "ip", ip, "attempts", attempts)
			h.renderBlockedPage(w, r, localizer)
			return
		}
		h.logger.Warn("password auth failed", "ip", ip, "attempts", attempts)
		h.renderPasswordPage(w, r, localizer, redir, false, "Invalid password.")
		return
	}

	h.clearFailures(r.Context(), ip)

	reqCopy := r.Clone(r.Context())
	reqCopy.URL = redirURL

	cr, rule, err := h.server.Check(reqCopy)
	if err != nil {
		http.Error(w, localizer.T("internal_server_error"), http.StatusInternalServerError)
		return
	}

	if err := h.issueValidation(w, r, cr, rule); err != nil {
		h.logger.Error("failed to issue validation", "err", err)
		http.Error(w, localizer.T("internal_server_error"), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, redir, http.StatusFound)
}

func (h *Hooks) passwordEndpointPath() string {
	return strings.TrimSuffix(anubis.BasePrefix, "/") + anubis.APIPrefix + "password"
}

func (h *Hooks) issueValidation(w http.ResponseWriter, r *http.Request, cr policy.CheckResult, rule *policy.Bot) error {
	handled, err := h.HandleAuthSuccess(w, r, cr, rule)
	if err != nil {
		return err
	}
	if !handled {
		return fmt.Errorf("validation handler did not handle request")
	}
	return nil
}

func (h *Hooks) issueJWT(w http.ResponseWriter, r *http.Request, cr policy.CheckResult, rule *policy.Bot) error {
	if h.server == nil {
		return fmt.Errorf("server not attached")
	}
	if rule == nil {
		return fmt.Errorf("nil policy rule")
	}

	claims := jwt.MapClaims{
		"challenge":  "password",
		"method":     "password",
		"policyRule": rule.Hash(),
		"action":     string(cr.Rule),
	}

	if h.cfg.JWTRestrictionHeader != "" {
		if r.Header.Get(h.cfg.JWTRestrictionHeader) == "" {
			return fmt.Errorf("JWTRestrictionHeader is set but not present in request")
		}
		claims["restriction"] = internal.SHA256sum(r.Header.Get(h.cfg.JWTRestrictionHeader))
	}
	if h.cfg.DifficultyInJWT && rule.Challenge != nil {
		claims["difficulty"] = rule.Challenge.Difficulty
	}

	tokenString, err := h.signJWT(claims)
	if err != nil {
		return err
	}

	cookiePath := "/"
	if anubis.BasePrefix != "" {
		cookiePath = strings.TrimSuffix(anubis.BasePrefix, "/") + "/"
	}

	h.server.SetCookie(w, lib.CookieOpts{Path: cookiePath, Host: r.Host, Value: tokenString})
	return nil
}

func (h *Hooks) signJWT(claims jwt.MapClaims) (string, error) {
	claims["iat"] = time.Now().Unix()
	claims["nbf"] = time.Now().Add(-1 * time.Minute).Unix()
	claims["exp"] = time.Now().Add(h.cfg.CookieExpiration).Unix()

	if len(h.cfg.HS512Secret) == 0 {
		if h.cfg.ED25519PrivateKey == nil {
			return "", fmt.Errorf("no ed25519 private key available")
		}
		return jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims).SignedString(h.cfg.ED25519PrivateKey)
	}

	return jwt.NewWithClaims(jwt.SigningMethodHS512, claims).SignedString(h.cfg.HS512Secret)
}

func (h *Hooks) passwordMatches(password string) bool {
	if h.cfg.Password == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(password), []byte(h.cfg.Password)) == 1
}

func (h *Hooks) recordFailure(ctx context.Context, ip string) (int, bool) {
	if ip == "" {
		return 0, false
	}
	attempt, err := h.attempts.Get(ctx, ip)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		h.logger.Error("failed to load password attempts", "ip", ip, "err", err)
	}
	attempt.Count++
	attempt.LastFailed = time.Now()
	if err := h.attempts.Set(ctx, ip, attempt, h.cfg.PasswordBanDuration); err != nil {
		h.logger.Error("failed to store password attempts", "ip", ip, "err", err)
	}
	return attempt.Count, attempt.Count >= h.cfg.PasswordMaxFails
}

func (h *Hooks) clearFailures(ctx context.Context, ip string) {
	if ip == "" {
		return
	}
	if err := h.attempts.Delete(ctx, ip); err != nil && !errors.Is(err, store.ErrNotFound) {
		h.logger.Error("failed to clear password attempts", "ip", ip, "err", err)
	}
}

func (h *Hooks) applyDelay(ctx context.Context, attempts int) {
	delay := time.Duration(attempts) * passwordDelayStep
	if delay > passwordDelayMax {
		delay = passwordDelayMax
	}
	if delay <= 0 {
		return
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return
	case <-timer.C:
		return
	}
}

func (h *Hooks) isBlocked(ctx context.Context, ip string) (bool, *PasswordAttempt) {
	if ip == "" {
		return false, nil
	}
	attempt, err := h.attempts.Get(ctx, ip)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			h.logger.Error("failed to load password attempts", "ip", ip, "err", err)
		}
		return false, nil
	}
	if attempt.Count >= h.cfg.PasswordMaxFails {
		return true, &attempt
	}
	return false, &attempt
}

func (h *Hooks) validateRedirect(r *http.Request, redir string) (*url.URL, error) {
	redirURL, err := url.ParseRequestURI(redir)
	if err != nil {
		return nil, err
	}

	switch redirURL.Scheme {
	case "", "http", "https":
		// allowed
	default:
		return nil, fmt.Errorf("invalid redirect scheme: %s", redirURL.Scheme)
	}

	urlParsed, err := redirURL.Parse(redir)
	if err != nil {
		return nil, err
	}

	if len(urlParsed.Host) > 0 && len(h.cfg.RedirectDomains) != 0 && !matchRedirectDomain(h.cfg.RedirectDomains, urlParsed.Host) {
		return nil, fmt.Errorf("redirect domain not allowed: %s", urlParsed.Host)
	}
	if urlParsed.Host != redirURL.Host {
		return nil, fmt.Errorf("redirect domain mismatch: %s", urlParsed.Host)
	}

	return redirURL, nil
}

func (h *Hooks) constructRedirectURL(r *http.Request) (string, error) {
	proto := r.Header.Get("X-Forwarded-Proto")
	host := r.Header.Get("X-Forwarded-Host")
	uri := r.Header.Get("X-Forwarded-Uri")

	if proto == "" || host == "" || uri == "" {
		return "", fmt.Errorf("missing required forwarded headers")
	}

	switch proto {
	case "http", "https":
		// allowed
	default:
		return "", fmt.Errorf("invalid redirect scheme: %s", proto)
	}

	if len(h.cfg.RedirectDomains) > 0 && !matchRedirectDomain(h.cfg.RedirectDomains, host) {
		return "", fmt.Errorf("redirect domain not allowed: %s", host)
	}

	redir := proto + "://" + host + uri
	escapedURL := url.QueryEscape(redir)
	return fmt.Sprintf("%s/.within.website/?redir=%s", h.cfg.PublicUrl, escapedURL), nil
}

func matchRedirectDomain(allowed []string, host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	for _, pat := range allowed {
		p := strings.ToLower(strings.TrimSpace(pat))
		if strings.Contains(p, glob.GLOB) {
			if glob.Glob(p, h) {
				return true
			}
			continue
		}
		if p == h {
			return true
		}
	}
	return false
}

func realIP(r *http.Request) string {
	ip := strings.TrimSpace(r.Header.Get("X-Real-Ip"))
	if ip == "" {
		ip = strings.TrimSpace(r.Header.Get("X-Real-IP"))
	}
	if ip == "" {
		return ""
	}
	if parsed := net.ParseIP(ip); parsed != nil {
		return parsed.String()
	}
	if host, _, err := net.SplitHostPort(ip); err == nil {
		if parsed := net.ParseIP(host); parsed != nil {
			return parsed.String()
		}
	}
	return ""
}

func (h *Hooks) renderPasswordPage(w http.ResponseWriter, r *http.Request, localizer *localization.SimpleLocalizer, redir string, blocked bool, errMsg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)

	basePrefix := strings.TrimSuffix(anubis.BasePrefix, "/")
	passwordAction := basePrefix + anubis.APIPrefix + "password"
	stylesheet := basePrefix + xess.URL
	imageName := "pensive.webp"
	if blocked {
		imageName = "reject.webp"
	}
	imageURL := basePrefix + anubis.StaticPath + "static/img/" + imageName

	writePasswordPage(w, localizer, stylesheet, imageURL, passwordAction, redir, errMsg)
}

func (h *Hooks) renderBlockedPage(w http.ResponseWriter, r *http.Request, localizer *localization.SimpleLocalizer) {
	msg := "Too many attempts, please try again later"
	templ.Handler(
		web.Base(localizer.T("oh_noes"), web.ErrorPage(msg, "", "", localizer), nil, localizer),
		templ.WithStatus(http.StatusTooManyRequests),
	).ServeHTTP(w, r)
}
