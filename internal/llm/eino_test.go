package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"steadwell/internal/store"
)

func TestEinoGenerate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization = %q", got)
		}
		var body struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if body.Model != "gpt-4o-mini" {
			t.Errorf("model = %q", body.Model)
		}
		if len(body.Messages) != 2 {
			t.Fatalf("messages = %d", len(body.Messages))
		}
		if body.Messages[0].Role != "system" || body.Messages[0].Content != "You are Steadwell" {
			t.Errorf("system = %+v", body.Messages[0])
		}
		if body.Messages[1].Role != "user" || body.Messages[1].Content != "hello" {
			t.Errorf("user = %+v", body.Messages[1])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-test",
			"choices": []map[string]any{
				{"index": 0, "message": map[string]string{"role": "assistant", "content": "Hi from LiteLLM"}},
			},
		})
	}))
	t.Cleanup(srv.Close)

	gen, err := NewEino(context.Background(), EinoConfig{
		BaseURL:    srv.URL + "/v1",
		APIKey:     "test-key",
		Model:      "gpt-4o-mini",
		Timeout:    5 * time.Second,
		HTTPClient: srv.Client(),
	})
	if err != nil {
		t.Fatalf("NewEino: %v", err)
	}
	out, err := gen.Generate(context.Background(), "You are Steadwell", nil, "hello")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if out != "Hi from LiteLLM" {
		t.Fatalf("got %q", out)
	}
}

func TestEinoGenerateWithHistory(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(body.Messages) != 4 {
			t.Fatalf("messages = %d want 4", len(body.Messages))
		}
		want := []string{"system", "user", "assistant", "user"}
		for i, role := range want {
			if body.Messages[i].Role != role {
				t.Errorf("msg[%d].role = %q want %q", i, body.Messages[i].Role, role)
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": "ok"}},
			},
		})
	}))
	t.Cleanup(srv.Close)

	gen, err := NewEino(context.Background(), EinoConfig{
		BaseURL:    srv.URL + "/v1",
		APIKey:     "k",
		Model:      "gpt-4o-mini",
		HTTPClient: srv.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	hist := []store.ChatMessage{
		{Role: store.ChatRoleUser, Content: "hi"},
		{Role: store.ChatRoleAssistant, Content: "hello"},
	}
	out, err := gen.Generate(context.Background(), "sys", hist, "again")
	if err != nil {
		t.Fatal(err)
	}
	if out != "ok" {
		t.Fatalf("got %q", out)
	}
}

func TestNewEinoRequiresBaseURLAndModel(t *testing.T) {
	_, err := NewEino(context.Background(), EinoConfig{Model: "m"})
	if err == nil || !strings.Contains(err.Error(), "BaseURL") {
		t.Fatalf("want BaseURL error, got %v", err)
	}
	_, err = NewEino(context.Background(), EinoConfig{BaseURL: "http://x"})
	if err == nil || !strings.Contains(err.Error(), "Model") {
		t.Fatalf("want Model error, got %v", err)
	}
}

func TestSelectGeneratorEchoWhenUnset(t *testing.T) {
	g, name, err := SelectGenerator(context.Background(), LiteLLMEnv{})
	if err != nil {
		t.Fatal(err)
	}
	if name != "echo" {
		t.Fatalf("name = %q", name)
	}
	out, err := g.Generate(context.Background(), "sys", nil, "ping")
	if err != nil {
		t.Fatal(err)
	}
	if out != "I received your message: ping" {
		t.Fatalf("got %q", out)
	}
}
