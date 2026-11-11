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
	Proto   string // "anthropic" or "openai": how the model's reached; "" as OpenCode Go's table has it
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
		case "protocol":
			c.Proto = val
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

// anthropic says whether the model is reached by Anthropic's messages,
// not OpenAI's chat completions (OpenCode Go's table, unless c says).
func (c Config) anthropic() bool {
	if c.Proto != "" {
		return c.Proto == "anthropic"
	}
	m := c.Model
	return strings.HasPrefix(m, "claude-") || strings.HasPrefix(m, "qwen3.") || m == "minimax-m3"
}

// Answer is what a model said, and what it cost.
type Answer struct {
	Text    string
	In, Out int     // tokens
	Cost    float64 // dollars counted against OpenCode Go's limits (its allowance), else the list price
}

// usage is an answer's usage, as both APIs (and OpenCode, in it) say it.
type usage struct {
	Usage struct {
		InputTokens      int `json:"input_tokens"`
		OutputTokens     int `json:"output_tokens"`
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		OpenCode         struct {
			ListCost  float64 `json:"list_cost"`
			Allowance float64 `json:"allowance"`
		} `json:"opencode"`
	} `json:"usage"`
}

func usageOf(body []byte, a *Answer) {
	var u usage
	if json.Unmarshal(body, &u) != nil {
		return
	}
	a.In, a.Out = u.Usage.InputTokens+u.Usage.PromptTokens, u.Usage.OutputTokens+u.Usage.CompletionTokens
	a.Cost = u.Usage.OpenCode.Allowance
	if a.Cost == 0 {
		a.Cost = u.Usage.OpenCode.ListCost
	}
}

// Ask asks the model: system's instructions, then msgs; its answer.
func (c Config) Ask(ctx context.Context, system string, msgs []Msg) (Answer, error) {
	var url string
	var body any
	hdr := http.Header{}
	if c.anthropic() {
		url, body = c.Base+"/anthropic/v1/messages", anthropicBody(c, system, msgs)
		hdr.Set("x-api-key", c.Key)
		hdr.Set("anthropic-version", "2023-06-01")
	} else {
		url, body = c.Base+"/openai/v1/chat/completions", openaiBody(c, system, msgs)
	}
	hdr.Set("Authorization", "Bearer "+c.Key)
	data, err := json.Marshal(body)
	if err != nil {
		return Answer{}, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return Answer{}, err
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
		return Answer{}, err
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return Answer{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return Answer{}, fmt.Errorf("%s: %s", resp.Status, errorText(out))
	}
	var a Answer
	if c.anthropic() {
		a.Text, err = anthropicText(out)
	} else {
		a.Text, err = openaiText(out)
	}
	usageOf(out, &a)
	return a, err
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
	if err := json.Unmarshal([]byte(texEscapes(answer[i:j+1])), v); err != nil {
		return fmt.Errorf("%v in %q", err, short(answer))
	}
	return nil
}

// texEscapes mends the TeX a model writes in a JSON string as it would
// anywhere else: \alpha (no JSON escape: an error), \frac (a form feed
// and "rac"), \theta (a tab). A backslash before what JSON
// doesn't escape, or before a TeX command that begins with one of
// JSON's escapes, is doubled: TeX's.
func texEscapes(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\\' {
			b.WriteByte(c)
			continue
		}
		if i+1 >= len(s) {
			b.WriteString(`\\`)
			continue
		}
		n := s[i+1]
		switch {
		case n == '\\' || n == '"' || n == '/':
			b.WriteByte(c)
			b.WriteByte(n)
			i++
		case n == 'u' && i+6 <= len(s) && isHex(s[i+2:i+6]):
			b.WriteString(s[i : i+6])
			i += 5
		case strings.IndexByte("bfnrt", n) >= 0 && !texCommands[word(s[i+1:])]:
			b.WriteByte(c)
			b.WriteByte(n)
			i++
		default:
			b.WriteString(`\\`) // and what's after it, as it is
		}
	}
	return b.String()
}

// texCommands are TeX's commands that begin with one of JSON's escapes
// (b f n r t).
var texCommands = func() map[string]bool {
	m := map[string]bool{}
	for _, c := range strings.Fields(`frac forall flat frown fbox beta bar bf big bigg bigl bigr biggl biggr bigm binom
		begin boldsymbol bullet bot bowtie bmod boxed breve backslash because bigcup bigcap bigoplus bigotimes bm
		nabla neq ne nu not notin neg nexists newline nmid nparallel nsubseteq ncong nsim nleq ngeq nless ngtr
		nolimits nonumber natural nearrow nwarrow ni right rho rangle rceil rfloor rightarrow rbrace rbrack rvert
		rVert rm rule rightleftharpoons rightharpoonup theta tau times text textbf textit textrm tfrac tilde to top
		triangle tan tanh tag therefore textstyle texttt thinspace triangleq tbinom tbinom textsf textnormal`) {
		m[c] = true
	}
	return m
}()

// word is the letters s begins with.
func word(s string) string {
	n := 0
	for n < len(s) && (s[n] >= 'a' && s[n] <= 'z' || s[n] >= 'A' && s[n] <= 'Z') {
		n++
	}
	return s[:n]
}

func isHex(s string) bool {
	for _, c := range s {
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return false
		}
	}
	return true
}

func short(s string) string {
	if r := []rune(s); len(r) > 120 {
		return string(r[:120]) + "…"
	}
	return s
}
