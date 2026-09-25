package replyfmt

import (
	"encoding/json"
	"html"
	"regexp"
	"strings"
	"unicode"

	"steadwell/internal/channel"
)

const (
	CallbackAccept = "sw:accept"
	CallbackReject = "sw:reject"
)

// LLMPayload is the locked-flow JSON the system prompt requires.
type LLMPayload struct {
	MessageBody string  `json:"message_body"`
	ButtonBody  *string `json:"button_body"`
	ConsentType *string `json:"consent_type"`
	AcceptLabel *string `json:"accept_label"`
	RejectLabel *string `json:"reject_label"`
}

// Parse turns model output into a Telegram-ready Reply and a plain history string.
func Parse(raw string) (channel.Reply, string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return channel.Reply{}, ""
	}
	payload, ok := decodePayload(raw)
	if !ok {
		htmlBody := ToHTML(raw)
		return channel.Reply{MessageBody: htmlBody, ParseMode: "HTML"}, stripToPlain(raw)
	}
	msg := strings.TrimSpace(payload.MessageBody)
	if msg == "" {
		htmlBody := ToHTML(raw)
		return channel.Reply{MessageBody: htmlBody, ParseMode: "HTML"}, stripToPlain(raw)
	}
	reply := channel.Reply{
		MessageBody: ToHTML(msg),
		ParseMode:   "HTML",
	}
	plain := msg
	btn := deref(payload.ButtonBody)
	consent := strings.TrimSpace(deref(payload.ConsentType))
	accept := limitRunes(coalesce(deref(payload.AcceptLabel), "I Agree"), 20)
	reject := limitRunes(coalesce(deref(payload.RejectLabel), "Disagree"), 20)
	if consent != "" && (accept != "" || reject != "") {
		if btn != "" {
			reply.ButtonBody = ToHTML(btn)
			plain = msg + "\n\n" + btn
		}
		row := make([]channel.InlineButton, 0, 2)
		if accept != "" {
			row = append(row, channel.InlineButton{Text: accept, CallbackData: CallbackAccept})
		}
		if reject != "" {
			row = append(row, channel.InlineButton{Text: reject, CallbackData: CallbackReject})
		}
		if len(row) > 0 {
			reply.Buttons = [][]channel.InlineButton{row}
		}
	}
	return reply, plain
}

// CallbackUserText maps inline callback_data to the label the model expects.
func CallbackUserText(data string) string {
	switch strings.TrimSpace(data) {
	case CallbackAccept:
		return "I Agree"
	case CallbackReject:
		return "Disagree"
	default:
		return strings.TrimSpace(data)
	}
}

var (
	fenceRE      = regexp.MustCompile("(?is)^```(?:json)?\\s*|\\s*```$")
	jsonObjectRE = regexp.MustCompile(`(?s)\{.*\}`)
	mdBoldRE     = regexp.MustCompile(`\*\*([^*]+)\*\*`)
)

func decodePayload(raw string) (LLMPayload, bool) {
	cleaned := strings.TrimSpace(fenceRE.ReplaceAllString(raw, ""))
	var p LLMPayload
	if err := json.Unmarshal([]byte(cleaned), &p); err == nil && strings.TrimSpace(p.MessageBody) != "" {
		return p, true
	}
	if m := jsonObjectRE.FindString(cleaned); m != "" {
		if err := json.Unmarshal([]byte(m), &p); err == nil && strings.TrimSpace(p.MessageBody) != "" {
			return p, true
		}
	}
	return LLMPayload{}, false
}

// ToHTML escapes text and applies bold + bullet formatting for Telegram HTML.
func ToHTML(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	// Markdown bold → markers we protect through escaping.
	text = mdBoldRE.ReplaceAllString(text, "\x00b\x00$1\x00/b\x00")
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimRightFunc(line, unicode.IsSpace)
		if trimmed == "" {
			out = append(out, "")
			continue
		}
		bullet := ""
		body := trimmed
		switch {
		case strings.HasPrefix(body, "• "):
			bullet = "• "
			body = strings.TrimSpace(body[len("• "):])
		case strings.HasPrefix(body, "- "):
			bullet = "• "
			body = strings.TrimSpace(body[2:])
		case strings.HasPrefix(body, "* "):
			bullet = "• "
			body = strings.TrimSpace(body[2:])
		}
		escaped := html.EscapeString(body)
		escaped = strings.ReplaceAll(escaped, "\x00b\x00", "<b>")
		escaped = strings.ReplaceAll(escaped, "\x00/b\x00", "</b>")
		if bullet == "" && shouldBoldLine(body) {
			escaped = "<b>" + escaped + "</b>"
		}
		out = append(out, bullet+escaped)
	}
	return strings.Join(out, "\n")
}

func shouldBoldLine(line string) bool {
	t := strings.TrimSpace(line)
	if t == "" {
		return false
	}
	// Short intro / section headers (e.g. "Steadwell is intended for:")
	if strings.HasSuffix(t, ":") && len([]rune(t)) <= 80 {
		return true
	}
	// Greeting first-line style without bullets
	lower := strings.ToLower(t)
	if strings.Contains(lower, "i'm steadwell") || strings.Contains(lower, "i am steadwell") {
		return true
	}
	return false
}

func stripToPlain(s string) string {
	s = mdBoldRE.ReplaceAllString(s, "$1")
	return strings.TrimSpace(s)
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(*p)
}

func coalesce(v, fallback string) string {
	if strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return fallback
}

func limitRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
