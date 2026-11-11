package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testKey = "sk-or-v1-secretsecretsecret-abcd"

func newTestClient(srvURL string, timeout time.Duration) *openRouter {
	c := NewOpenRouter(&http.Client{Timeout: timeout}).(*openRouter)
	c.url = srvURL
	return c
}

func TestOpenRouterRequestShape(t *testing.T) {
	var got struct {
		Model          string `json:"model"`
		Temperature    *float64
		Messages       []struct{ Role, Content string }
		ResponseFormat struct {
			Type       string `json:"type"`
			JSONSchema struct {
				Name   string          `json:"name"`
				Strict bool            `json:"strict"`
				Schema json.RawMessage `json:"schema"`
			} `json:"json_schema"`
		} `json:"response_format"`
	}
	var auth, title, referer, method, ctype string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, auth, title, referer, ctype = r.Method, r.Header.Get("Authorization"), r.Header.Get("X-Title"), r.Header.Get("HTTP-Referer"), r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &got); err != nil {
			t.Errorf("request body: %v", err)
		}
		_, _ = io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"{\"ok\":true}"}}]}`)
	}))
	defer srv.Close()

	schema := json.RawMessage(`{"type":"object","properties":{"ok":{"type":"boolean"}},"required":["ok"],"additionalProperties":false}`)
	out, err := newTestClient(srv.URL, 5*time.Second).Complete(context.Background(), testKey, "vendor/model-1", "system text", "user text", schema)
	if err != nil {
		t.Fatal(err)
	}
	if out != `{"ok":true}` {
		t.Fatalf("content = %q", out)
	}
	if method != http.MethodPost || auth != "Bearer "+testKey || title != "Mealplan" || referer == "" || !strings.HasPrefix(ctype, "application/json") {
		t.Fatalf("method %q auth %q title %q referer %q content-type %q", method, auth, title, referer, ctype)
	}
	if got.Model != "vendor/model-1" || got.Temperature == nil || *got.Temperature != 0 {
		t.Fatalf("model %q temperature %v", got.Model, got.Temperature)
	}
	if got.ResponseFormat.Type != "json_schema" || !got.ResponseFormat.JSONSchema.Strict || got.ResponseFormat.JSONSchema.Name == "" {
		t.Fatalf("response_format = %+v", got.ResponseFormat)
	}
	if string(got.ResponseFormat.JSONSchema.Schema) != string(schema) {
		t.Fatalf("schema = %s", got.ResponseFormat.JSONSchema.Schema)
	}
	if len(got.Messages) != 2 || got.Messages[0].Role != "system" || got.Messages[0].Content != "system text" ||
		got.Messages[1].Role != "user" || got.Messages[1].Content != "user text" {
		t.Fatalf("messages = %+v", got.Messages)
	}
}

func TestOpenRouterFixedURL(t *testing.T) {
	if c := NewOpenRouter(nil).(*openRouter); c.url != "https://openrouter.ai/api/v1/chat/completions" || c.hc.Timeout != 60*time.Second {
		t.Fatalf("url %q timeout %v", c.url, c.hc.Timeout)
	}
}

func TestOpenRouterErrors(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		body    string
		delay   time.Duration
		want    error
		timeout time.Duration
	}{
		{name: "bad key", status: 401, body: `{"error":{"code":401,"message":"No auth credentials found ` + testKey + `"}}`, want: ErrBadKey},
		{name: "no credit", status: 402, body: `{"error":{"code":402,"message":"Insufficient credits"}}`, want: ErrNoCredit},
		{name: "server error", status: 500, body: `{"error":{"code":500,"message":"boom ` + testKey + `"}}`, want: ErrUnavailable},
		{name: "unknown model", status: 400, body: `{"error":{"code":400,"message":"x/y is not a valid model ID ` + testKey + `"}}`, want: ErrBadModel},
		{name: "model not found", status: 404, body: `{"error":{"code":404,"message":"No endpoints found for x/y ` + testKey + `"}}`, want: ErrBadModel},
		{name: "error in 200", status: 200, body: `{"error":{"code":502,"message":"provider failed"}}`, want: ErrUnavailable},
		{name: "no choices", status: 200, body: `{"choices":[]}`, want: ErrUnavailable},
		{name: "not json", status: 200, body: `<html>`, want: ErrUnavailable},
		{name: "too large", status: 200, body: `{"choices":[{"message":{"content":"` + strings.Repeat("a", 1<<20) + `"}}]}`, want: ErrUnavailable},
		{name: "timeout", status: 200, body: `{"choices":[{"message":{"content":"{}"}}]}`, delay: 300 * time.Millisecond, timeout: 50 * time.Millisecond, want: ErrUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.delay > 0 {
					select {
					case <-time.After(tc.delay):
					case <-r.Context().Done():
						return
					}
				}
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer srv.Close()
			timeout := tc.timeout
			if timeout == 0 {
				timeout = 5 * time.Second
			}
			_, err := newTestClient(srv.URL, timeout).Complete(context.Background(), testKey, "vendor/model", "s", "u", json.RawMessage(`{}`))
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if strings.Contains(err.Error(), testKey) || strings.Contains(err.Error(), "secretsecret") {
				t.Fatalf("error leaks the key: %v", err)
			}
		})
	}
}
