package cust

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/TecharoHQ/anubis/lib/policy"
	"github.com/TecharoHQ/anubis/lib/store/memory"
)

func TestHandleAuthSuccessBlocksNonPasswordEndpoint(t *testing.T) {
	h, err := NewHooks(Config{
		AuthMode:  AuthModePassword,
		ValidMode: ValidModeJWT,
		Password:  "correct horse battery staple",
	}, memory.New(context.Background()), nil)
	if err != nil {
		t.Fatalf("can't create hooks: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/.within.website/x/cmd/anubis/api/pass-challenge", nil)
	handled, err := h.HandleAuthSuccess(httptest.NewRecorder(), req, policy.CheckResult{}, nil)
	if err != nil {
		t.Fatalf("expected non-password endpoint to be blocked without error, got: %v", err)
	}
	if !handled {
		t.Fatalf("expected non-password endpoint to be fully handled")
	}
}
