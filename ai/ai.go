// Package ai asks a language model, over OpenCode Go's API (or any
// that speaks OpenAI's chat completions or Anthropic's messages):
// text, and pictures of what's written, in; text out.
package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	_ "golang.org/x/crypto/x509roots/fallback" // CA roots, in case the Kobo has none Go can find
)

// Base is OpenCode Go's API.
const Base = "https://opencode.ai/inference/go"

// DefaultModel reads handwriting (it takes pictures) and answers fast.
const DefaultModel = "claude-haiku-5-5"

// Config is how to reach the model: from a file of "name = value"
// lines (key, model, base), or one with only the key in it.
type Config struct {
	Key     string
	Model   string
	Base    string
	Agent   string // the User-Agent: the app's, as OpenCode asks
	Session string // a stable id for the session header
	MaxOut  int    // tokens an answer may have
}

// Load reads path's config; os.ErrNotExist if there's none.
func Load(path string) (Config, error) {
	c := Config{Model: DefaultModel, Base: Base, MaxOut: 1500}
	data, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		l := strings.TrimSpace(sc.Text())
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		name, val, ok := strings.Cut(l, "=")
		if !ok { // a bare key
			c.Key = l
			continue
		}
		name, val = strings.ToLower(strings.TrimSpace(name)), strings.TrimSpace(val)
		switch name {
		case "key", "api_key", "apikey":
			c.Key = val
		case "model":
			c.Model = val
		case "base", "url":
			c.Base = strings.TrimRight(val, "/")
		case "max_tokens":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				c.MaxOut = n
			}
		}
	}
	if c.Key == "" {
		return c, errors.New("no key in it")
	}
	return c, sc.Err()
}

// Part is a piece of a message: text, or a PNG.
type Part struct {
	Text string
	PNG  []byte
}

// Msg is a turn: the user's ("user") or the model's ("assistant").
type Msg struct {
	Role  string
	Parts []Part
}

// anthropic says whether model is reached by Anthropic's messages, not
// OpenAI's chat completions (OpenCode Go's table).
func anthropic(model string) bool {
	return strings.HasPrefix(model, "claude-") || strings.HasPrefix(model, "qwen3.") || model == "minimax-m3"
}

// Ask asks the model: system's instructions, then msgs; its answer.
func (c Config) Ask(ctx context.Context, system string, msgs []Msg) (string, error) {
	var url string
	var body any
	hdr := http.Header{}
	if anthropic(c.Model) {
		url, body = c.Base+"/anthropic/v1/messages", anthropicBody(c, system, msgs)
		hdr.Set("x-api-key", c.Key)
		hdr.Set("anthropic-version", "2023-06-01")
	} else {
		url, body = c.Base+"/openai/v1/chat/completions", openaiBody(c, system, msgs)
	}
	hdr.Set("Authorization", "Bearer "+c.Key)
	data, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header = hdr
	req.Header.Set("Content-Type", "application/json")
	if c.Agent != "" {
		req.Header.Set("User-Agent", c.Agent)
	}
	if c.Session != "" {
		req.Header.Set("x-opencode-session", c.Session)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: %s", resp.Status, errorText(out))
	}
	if anthropic(c.Model) {
		return anthropicText(out)
	}
	return openaiText(out)
}

var client = &http.Client{Timeout: 3 * time.Minute}

// errorText is what an error's body says, short.
func errorText(body []byte) string {
	var e struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &e) == nil {
		if e.Error.Message != "" {
			return e.Error.Message
		}
		if e.Message != "" {
			return e.Message
		}
	}
	s := strings.TrimSpace(string(body))
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	return s
}

func dataURL(png []byte) string {
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
}

func openaiBody(c Config, system string, msgs []Msg) map[string]any {
	ms := []map[string]any{{"role": "system", "content": system}}
	for _, m := range msgs {
		var parts []map[string]any
		for _, p := range m.Parts {
			if p.PNG != nil {
				parts = append(parts, map[string]any{"type": "image_url", "image_url": map[string]any{"url": dataURL(p.PNG)}})
			} else {
				parts = append(parts, map[string]any{"type": "text", "text": p.Text})
			}
		}
		ms = append(ms, map[string]any{"role": m.Role, "content": parts})
	}
	return map[string]any{"model": c.Model, "messages": ms, "max_tokens": c.MaxOut}
}

func openaiText(body []byte) (string, error) {
	var r struct {
		Choices []struct {
			Message struct {
				Content any `json:"content"` // text, or parts
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return "", fmt.Errorf("its answer: %v", err)
	}
	if len(r.Choices) == 0 {
		return "", errors.New("an answer of nothing")
	}
	switch c := r.Choices[0].Message.Content.(type) {
	case string:
		return c, nil
	case []any:
		var b strings.Builder
		for _, p := range c {
			if m, ok := p.(map[string]any); ok {
				if t, ok := m["text"].(string); ok {
					b.WriteString(t)
				}
			}
		}
		return b.String(), nil
	}
	return "", errors.New("an answer of nothing")
}

func anthropicBody(c Config, system string, msgs []Msg) map[string]any {
	var ms []map[string]any
	for _, m := range msgs {
		var parts []map[string]any
		for _, p := range m.Parts {
			if p.PNG != nil {
				parts = append(parts, map[string]any{"type": "image", "source": map[string]any{
					"type": "base64", "media_type": "image/png", "data": base64.StdEncoding.EncodeToString(p.PNG)}})
			} else {
				parts = append(parts, map[string]any{"type": "text", "text": p.Text})
			}
		}
		ms = append(ms, map[string]any{"role": m.Role, "content": parts})
	}
	return map[string]any{"model": c.Model, "system": system, "messages": ms, "max_tokens": c.MaxOut}
}

func anthropicText(body []byte) (string, error) {
	var r struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return "", fmt.Errorf("its answer: %v", err)
	}
	var b strings.Builder
	for _, p := range r.Content {
		if p.Type == "text" {
			b.WriteString(p.Text)
		}
	}
	if b.Len() == 0 {
		return "", errors.New("an answer of nothing")
	}
	return b.String(), nil
}

// JSON finds the JSON object in a model's answer (it may be fenced, or
// said around) and reads it into v.
func JSON(answer string, v any) error {
	i, j := strings.Index(answer, "{"), strings.LastIndex(answer, "}")
	if i < 0 || j < i {
		return fmt.Errorf("no JSON in %q", short(answer))
	}
	if err := json.Unmarshal([]byte(answer[i:j+1]), v); err != nil {
		return fmt.Errorf("%v in %q", err, short(answer))
	}
	return nil
}

func short(s string) string {
	if r := []rune(s); len(r) > 120 {
		return string(r[:120]) + "…"
	}
	return s
}
