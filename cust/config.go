package cust

import (
	"crypto/ed25519"
	"fmt"
	"strings"
	"time"
)

const (
	AuthModePow      = "pow"
	AuthModePassword = "password"

	ValidModeJWT       = "jwt"
	ValidModeWhitelist = "whitelist"
)

type Config struct {
	AuthMode             string
	ValidMode            string
	Password             string
	WhitelistTTL         time.Duration
	WhitelistSliding     bool
	PasswordMaxFails     int
	PasswordBanDuration  time.Duration
	PublicUrl            string
	RedirectDomains      []string
	CookieExpiration     time.Duration
	JWTRestrictionHeader string
	DifficultyInJWT      bool
	HS512Secret          []byte
	ED25519PrivateKey    ed25519.PrivateKey
}

func (c *Config) Normalize() {
	c.AuthMode = strings.ToLower(strings.TrimSpace(c.AuthMode))
	c.ValidMode = strings.ToLower(strings.TrimSpace(c.ValidMode))

	if c.AuthMode == "" {
		c.AuthMode = AuthModePow
	}
	if c.ValidMode == "" {
		c.ValidMode = ValidModeJWT
	}
	if c.WhitelistTTL <= 0 {
		c.WhitelistTTL = time.Hour
	}
	if c.PasswordMaxFails <= 0 {
		c.PasswordMaxFails = 10
	}
	if c.PasswordBanDuration <= 0 {
		c.PasswordBanDuration = 15 * time.Minute
	}
	// Sliding expiration is always enabled by default.
	c.WhitelistSliding = true
}

func (c Config) Validate() error {
	switch c.AuthMode {
	case AuthModePow, AuthModePassword:
	default:
		return fmt.Errorf("unknown auth mode: %q", c.AuthMode)
	}

	switch c.ValidMode {
	case ValidModeJWT, ValidModeWhitelist:
	default:
		return fmt.Errorf("unknown valid mode: %q", c.ValidMode)
	}

	if c.AuthMode == AuthModePassword && c.Password == "" {
		return fmt.Errorf("password auth mode requires a non-empty password")
	}
	if c.PasswordMaxFails < 1 {
		return fmt.Errorf("password max fails must be >= 1")
	}
	if c.PasswordBanDuration < 0 {
		return fmt.Errorf("password ban duration must be >= 0")
	}

	return nil
}
