package adminorg

import (
	"encoding/json"
	"strings"
)

type ChannelInput struct {
	Enabled            bool   `json:"enabled"`
	BotUsername        string `json:"bot_username"`
	BotToken           string `json:"bot_token"`
	WebhookSecret      string `json:"webhook_secret"`
	PhoneNumber        string `json:"phone_number"`
	APIToken           string `json:"api_token"`
	LiffURL            string `json:"liff_url"`
	ChannelSecret      string `json:"channel_secret"`
	ChannelAccessToken string `json:"channel_access_token"`
}

type credsError struct {
	Errors []string
}

func (e credsError) Error() string {
	if len(e.Errors) == 0 {
		return "channel credentials invalid"
	}
	return strings.Join(e.Errors, "; ")
}

func validateChannelCreds(slug string, in ChannelInput, hasExisting bool) []string {
	if !in.Enabled {
		return nil
	}
	var errs []string
	switch strings.ToLower(strings.TrimSpace(slug)) {
	case "telegram":
		if strings.TrimSpace(in.BotUsername) == "" {
			errs = append(errs, "telegram: bot_username is required")
		}
		if !hasExisting && strings.TrimSpace(in.BotToken) == "" {
			errs = append(errs, "telegram: bot_token is required")
		}
	case "whatsapp":
		if strings.TrimSpace(in.PhoneNumber) == "" {
			errs = append(errs, "whatsapp: phone_number is required")
		}
		if !hasExisting && strings.TrimSpace(in.APIToken) == "" {
			errs = append(errs, "whatsapp: api_token is required")
		}
	case "line":
		if strings.TrimSpace(in.LiffURL) == "" {
			errs = append(errs, "line: liff_url is required")
		}
		if !hasExisting {
			if strings.TrimSpace(in.ChannelSecret) == "" {
				errs = append(errs, "line: channel_secret is required")
			}
			if strings.TrimSpace(in.ChannelAccessToken) == "" {
				errs = append(errs, "line: channel_access_token is required")
			}
		}
	}
	return errs
}

func publicConfigJSON(slug string, in ChannelInput) ([]byte, error) {
	m := map[string]string{}
	switch strings.ToLower(slug) {
	case "telegram":
		if v := strings.TrimSpace(in.BotUsername); v != "" {
			m["bot_username"] = strings.TrimPrefix(v, "@")
		}
	case "whatsapp":
		if v := strings.TrimSpace(in.PhoneNumber); v != "" {
			m["phone_number"] = v
		}
	case "line":
		if v := strings.TrimSpace(in.LiffURL); v != "" {
			m["liff_url"] = v
		}
	}
	if len(m) == 0 {
		return []byte("{}"), nil
	}
	return json.Marshal(m)
}

func secretPayloadJSON(slug string, in ChannelInput) ([]byte, bool, error) {
	m := map[string]string{}
	switch strings.ToLower(slug) {
	case "telegram":
		if v := strings.TrimSpace(in.BotToken); v != "" {
			m["bot_token"] = v
		}
		if v := strings.TrimSpace(in.WebhookSecret); v != "" {
			m["webhook_secret"] = v
		}
	case "whatsapp":
		if v := strings.TrimSpace(in.APIToken); v != "" {
			m["api_token"] = v
		}
	case "line":
		if v := strings.TrimSpace(in.ChannelSecret); v != "" {
			m["channel_secret"] = v
		}
		if v := strings.TrimSpace(in.ChannelAccessToken); v != "" {
			m["channel_access_token"] = v
		}
	}
	if len(m) == 0 {
		return nil, false, nil
	}
	b, err := json.Marshal(m)
	return b, true, err
}

func publicFromJSON(raw []byte) map[string]string {
	out := map[string]string{}
	if len(raw) == 0 {
		return out
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return out
	}
	for k, v := range m {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			out[k] = strings.TrimSpace(s)
		}
	}
	return out
}

func mergePublic(existing map[string]string, slug string, in ChannelInput) map[string]string {
	next := map[string]string{}
	for k, v := range existing {
		next[k] = v
	}
	switch strings.ToLower(slug) {
	case "telegram":
		if v := strings.TrimSpace(in.BotUsername); v != "" {
			next["bot_username"] = strings.TrimPrefix(v, "@")
		}
	case "whatsapp":
		if v := strings.TrimSpace(in.PhoneNumber); v != "" {
			next["phone_number"] = v
		}
	case "line":
		if v := strings.TrimSpace(in.LiffURL); v != "" {
			next["liff_url"] = v
		}
	}
	return next
}

func fmtFieldErrors(errs []string) error {
	if len(errs) == 0 {
		return nil
	}
	return credsError{Errors: errs}
}

func asCredsError(err error, dest *credsError) bool {
	if err == nil || dest == nil {
		return false
	}
	e, ok := err.(credsError)
	if !ok {
		return false
	}
	*dest = e
	return true
}
