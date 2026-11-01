package cust

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/TecharoHQ/anubis/lib"
	"github.com/TecharoHQ/anubis/lib/policy"
	"github.com/TecharoHQ/anubis/lib/store/memory"
)

func TestPasswordHint(t *testing.T) {
	for _, tt := range []struct {
		name string
		hint *string
		want string
	}{
		{name: "unset"},
		{name: "empty", hint: new("")},
		{name: "question", hint: new("What is two plus two?"), want: "What is two plus two?"},
		{name: "multiline", hint: new("First line\nSecond line"), want: "First line\nSecond line"},
		{name: "html escaped", hint: new(`<script>alert("hint")</script> & question`), want: "&lt;script&gt;alert(&#34;hint&#34;)&lt;/script&gt; &amp; question"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h, err := NewHooks(Config{
				AuthMode:     AuthModePassword,
				Password:     "secret-answer",
				PasswordHint: tt.hint,
			}, memory.New(t.Context()), nil)
			if err != nil {
				t.Fatalf("can't create hooks: %v", err)
			}
			h.AttachServer(&lib.Server{})

			for _, failed := range []bool{false, true} {
				name := "initial visit"
				if failed {
					name = "failed attempt"
				}
				t.Run(name, func(t *testing.T) {
					rec := httptest.NewRecorder()
					if failed {
						form := url.Values{"password": {"wrong"}, "redir": {"/"}}
						req := httptest.NewRequest(http.MethodPost, h.passwordEndpointPath(), strings.NewReader(form.Encode()))
						req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
						h.HandlePassword(rec, req)
					} else {
						req := httptest.NewRequest(http.MethodGet, "/", nil)
						handled, err := h.RenderAuthPage(rec, req, policy.CheckResult{}, nil, false)
						if err != nil || !handled {
							t.Fatalf("RenderAuthPage = %v, %v", handled, err)
						}
					}
					if rec.Code != http.StatusOK {
						t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
					}
					body := rec.Body.String()
					if tt.want == "" {
						if strings.Contains(body, `<label class="password-hint"`) {
							t.Error("empty hint should not render a label")
						}
					} else {
						label := `<label class="password-hint" for="password">` + tt.want + `</label>`
						if !strings.Contains(body, label+`<input id="password" type="password"`) {
							t.Error("expected escaped hint label immediately before its password input")
						}
					}
					if strings.Contains(body, "secret-answer") {
						t.Error("password must not be included in the page")
					}
					if strings.Contains(body, `<p class="error">`) != failed {
						t.Errorf("error message presence does not match failed attempt: %v", failed)
					}
				})
			}
		})
	}
}

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
