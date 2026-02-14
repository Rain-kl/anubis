package lib

import (
	"net/http"

	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/lib/policy"
)

// Check evaluates the list of rules and returns the result.
// This wraps the internal check method with a request-scoped logger.
func (s *Server) Check(r *http.Request) (policy.CheckResult, *policy.Bot, error) {
	lg := internal.GetRequestLogger(s.logger, r)
	return s.check(r, lg)
}
