package telegramapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"steadwell/internal/channel"
)

func TestBotSend(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode: %v", err)
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)

	b := NewBot("tok")
	b.httpClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			req.URL.Scheme = "http"
			req.URL.Host = srv.Listener.Addr().String()
			return http.DefaultTransport.RoundTrip(req)
		}),
	}
	if err := b.Send(context.Background(), "99", channel.Reply{MessageBody: "hello"}); err != nil {
		t.Fatal(err)
	}
	if got["chat_id"] != "99" || got["text"] != "hello" {
		t.Fatalf("got %#v", got)
	}
}

func TestBotSendHTMLWithButtons(t *testing.T) {
	var calls []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode: %v", err)
		}
		calls = append(calls, body)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)

	b := NewBot("tok")
	b.httpClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			req.URL.Scheme = "http"
			req.URL.Host = srv.Listener.Addr().String()
			return http.DefaultTransport.RoundTrip(req)
		}),
	}
	err := b.Send(context.Background(), "7", channel.Reply{
		MessageBody: "<b>Hi</b>\n• one",
		ParseMode:   "HTML",
		ButtonBody:  "Confirm?",
		Buttons: [][]channel.InlineButton{
			{{Text: "I Agree", CallbackData: "sw:accept"}, {Text: "Disagree", CallbackData: "sw:reject"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 {
		t.Fatalf("calls = %d", len(calls))
	}
	if calls[0]["parse_mode"] != "HTML" || calls[0]["text"] != "<b>Hi</b>\n• one" {
		t.Fatalf("msg1 = %#v", calls[0])
	}
	if calls[0]["reply_markup"] != nil {
		t.Fatal("first bubble should not carry keyboard when button_body is set")
	}
	if calls[1]["text"] != "Confirm?" {
		t.Fatalf("msg2 = %#v", calls[1])
	}
	if calls[1]["reply_markup"] == nil {
		t.Fatal("expected keyboard on second bubble")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
