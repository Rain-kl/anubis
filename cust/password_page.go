package cust

import (
	"html"
	"io"
	"strings"

	"github.com/TecharoHQ/anubis/lib/localization"
)

func writePasswordPage(w io.Writer, localizer *localization.SimpleLocalizer, stylesheet, imageURL, action, redir, errMsg string) {
	title := localizer.T("authorization_required")
	lang := localizer.GetLang()

	escapedTitle := html.EscapeString(title)
	escapedLang := html.EscapeString(lang)
	escapedAction := html.EscapeString(action)
	escapedRedir := html.EscapeString(redir)
	escapedErr := html.EscapeString(errMsg)
	escapedCSS := html.EscapeString(stylesheet)
	escapedImage := html.EscapeString(imageURL)

	var b strings.Builder
	b.WriteString("<!doctype html>")
	b.WriteString("<html lang=\"")
	b.WriteString(escapedLang)
	b.WriteString("\">")
	b.WriteString("<head>")
	b.WriteString("<meta charset=\"utf-8\">")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">")
	b.WriteString("<meta name=\"robots\" content=\"noindex,nofollow\">")
	b.WriteString("<title>")
	b.WriteString(escapedTitle)
	b.WriteString("</title>")
	if stylesheet != "" {
		b.WriteString("<link rel=\"stylesheet\" href=\"")
		b.WriteString(escapedCSS)
		b.WriteString("\">")
	}
	b.WriteString("<style>")
	b.WriteString("html,body{height:100%;margin:0;display:flex;align-items:center;justify-content:center;}")
	b.WriteString("main{max-width:24rem;width:92%;text-align:center;}")
	b.WriteString("form{display:flex;flex-direction:column;gap:.75rem;}")
	b.WriteString("input[type=password]{padding:.6rem;border:1px solid #ccc;border-radius:.4rem;}")
	b.WriteString("button{padding:.6rem;border:0;border-radius:.4rem;background:#b16286;color:#fff;font-weight:600;cursor:pointer;}")
	b.WriteString(".error{color:#b00020;margin:0.5rem 0 0;}")
	b.WriteString("</style>")
	b.WriteString("</head>")
	b.WriteString("<body>")
	b.WriteString("<main>")
	b.WriteString("<h1>")
	b.WriteString(escapedTitle)
	b.WriteString("</h1>")

	if imageURL != "" {
		b.WriteString("<img src=\"")
		b.WriteString(escapedImage)
		b.WriteString("\" alt=\"avatar\" style=\"width:100%;max-width:256px;height:auto;margin:0 auto 1.25rem;display:block;\">")
	}

	b.WriteString("<form method=\"POST\" action=\"")
	b.WriteString(escapedAction)
	b.WriteString("\">")
	b.WriteString("<input type=\"password\" name=\"password\" autocomplete=\"current-password\" placeholder=\"Password\" required>")
	b.WriteString("<input type=\"hidden\" name=\"redir\" value=\"")
	b.WriteString(escapedRedir)
	b.WriteString("\">")
	b.WriteString("<button type=\"submit\">Continue</button>")
	b.WriteString("</form>")
	if errMsg != "" {
		b.WriteString("<p class=\"error\">")
		b.WriteString(escapedErr)
		b.WriteString("</p>")
	}
	b.WriteString("</main>")
	b.WriteString("</body>")
	b.WriteString("</html>")

	_, _ = io.WriteString(w, b.String())
}
