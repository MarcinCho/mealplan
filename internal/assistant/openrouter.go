// Package assistant asks a language model (through OpenRouter) to match recipe ingredients to
// products and to find duplicate products. Everything it returns is a validated proposal: the
// package never writes products, aliases or ingredients itself.
package assistant

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

// DefaultModel is used when the household has not chosen a model.
//
// Chosen on 2026-11-11 from https://openrouter.ai/api/v1/models: google/gemini-3.1-flash-lite is a
// released (non-preview) fast "lite" model that lists structured_outputs, response_format and
// temperature in supported_parameters, handles Polish well and costs $0.25 / $1.50 per million
// input / output tokens. A prompt with a few hundred products is ~20k tokens, i.e. well under a
// cent per batch. Cheaper entries were :free/:batch variants, ~latest aliases that change under
// the user, or small models with weak Polish.
const DefaultModel = "google/gemini-3.1-flash-lite"

const (
	endpoint    = "https://openrouter.ai/api/v1/chat/completions"
	callTimeout = 60 * time.Second
	maxResponse = 1 << 20
	maxDetail   = 200
	referer     = "https://github.com/MarcinCho/mealplan"
)

var (
	ErrNotConfigured = errors.New("assistant: no OpenRouter key is set")
	ErrUnavailable   = errors.New("assistant: OpenRouter is unavailable")
	ErrBadKey        = errors.New("assistant: OpenRouter rejected the key")
	ErrNoCredit      = errors.New("assistant: the OpenRouter account has no credit")
)

// Client sends one chat completion whose answer must follow the JSON schema and returns the
// answer text. Errors wrap ErrUnavailable, ErrBadKey or ErrNoCredit and never contain the key.
type Client interface {
	Complete(ctx context.Context, key, model string, system, user string, schema json.RawMessage) (string, error)
}

type openRouter struct {
	hc  *http.Client
	url string // fixed endpoint; tests point it at httptest
}

// NewOpenRouter returns a client for the fixed OpenRouter endpoint. A nil hc gets a client with a
// 60 s timeout.
func NewOpenRouter(hc *http.Client) Client {
	if hc == nil {
		hc = &http.Client{Timeout: callTimeout}
	}
	return &openRouter{hc: hc, url: endpoint}
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type request struct {
	Model          string    `json:"model"`
	Messages       []message `json:"messages"`
	Temperature    float64   `json:"temperature"`
	ResponseFormat struct {
		Type       string `json:"type"`
		JSONSchema struct {
			Name   string          `json:"name"`
			Strict bool            `json:"strict"`
			Schema json.RawMessage `json:"schema"`
		} `json:"json_schema"`
	} `json:"response_format"`
	// Provider routing: only send to providers that honour response_format.
	Provider struct {
		RequireParameters bool `json:"require_parameters"`
	} `json:"provider"`
}

type apiError struct {
	Code    any    `json:"code"`
	Message string `json:"message"`
}

type response struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *apiError `json:"error"`
}

func (c *openRouter) Complete(ctx context.Context, key, model, system, user string, schema json.RawMessage) (string, error) {
	var req request
	req.Model = model
	req.Messages = []message{{Role: "system", Content: system}, {Role: "user", Content: user}}
	req.ResponseFormat.Type = "json_schema"
	req.ResponseFormat.JSONSchema.Name = "result"
	req.ResponseFormat.JSONSchema.Strict = true
	req.ResponseFormat.JSONSchema.Schema = schema
	req.Provider.RequireParameters = true
	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("%w: encode request: %v", ErrUnavailable, err)
	}
	hr, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	hr.Header.Set("Authorization", "Bearer "+key)
	hr.Header.Set("Content-Type", "application/json")
	hr.Header.Set("HTTP-Referer", referer)
	hr.Header.Set("X-Title", "Mealplan")
	res, err := c.hc.Do(hr)
	if err != nil {
		// *url.Error names the method and URL only; the key travels in a header.
		return "", fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, maxResponse+1))
	if err != nil {
		return "", fmt.Errorf("%w: read response: %w", ErrUnavailable, err)
	}
	if len(raw) > maxResponse {
		return "", fmt.Errorf("%w: response larger than %d bytes", ErrUnavailable, maxResponse)
	}
	var out response
	jsonErr := json.Unmarshal(raw, &out)
	switch res.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized:
		return "", ErrBadKey
	case http.StatusPaymentRequired:
		return "", ErrNoCredit
	default:
		return "", fmt.Errorf("%w: status %d%s", ErrUnavailable, res.StatusCode, detail(out.Error, key))
	}
	if jsonErr != nil {
		return "", fmt.Errorf("%w: response is not JSON", ErrUnavailable)
	}
	if out.Error != nil {
		return "", fmt.Errorf("%w: provider error%s", ErrUnavailable, detail(out.Error, key))
	}
	if len(out.Choices) == 0 || strings.TrimSpace(out.Choices[0].Message.Content) == "" {
		return "", fmt.Errorf("%w: empty answer", ErrUnavailable)
	}
	return out.Choices[0].Message.Content, nil
}

// detail returns OpenRouter's error message (e.g. "x/y is not a valid model ID"), shortened and
// with the key removed should it ever be echoed back.
func detail(e *apiError, key string) string {
	if e == nil {
		return ""
	}
	msg := e.Message
	if key != "" {
		msg = strings.ReplaceAll(msg, key, "***")
	}
	msg = strings.Join(strings.Fields(msg), " ")
	if utf8.RuneCountInString(msg) > maxDetail {
		msg = string([]rune(msg)[:maxDetail]) + "…"
	}
	if msg == "" {
		return ""
	}
	return ": " + msg
}
