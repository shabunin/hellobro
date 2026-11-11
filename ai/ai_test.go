package ai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	for _, c := range []struct {
		file       string
		key, model string
		base       string
		err        bool
	}{
		{"sk-bare\n", "sk-bare", DefaultModel, Base, false},
		{"# mine\nkey = sk-1\nmodel = kimi-k3\nbase = http://x/\n", "sk-1", "kimi-k3", "http://x", false},
		{"model = glm-5.3\n", "", "glm-5.3", Base, true},
	} {
		p := filepath.Join(dir, "ai.conf")
		os.WriteFile(p, []byte(c.file), 0o600)
		got, err := Load(p)
		if (err != nil) != c.err || got.Key != c.key || got.Model != c.model || got.Base != c.base {
			t.Errorf("Load(%q) = %+v, %v", c.file, got, err)
		}
	}
	if _, err := Load(filepath.Join(dir, "none")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("no file: %v", err)
	}
}

// server answers as the model would, after checking what it was sent.
func server(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		if err := json.Unmarshal(body, &req); err != nil {
			t.Errorf("%s: %v", r.URL.Path, err)
		}
		if r.Header.Get("Authorization") != "Bearer sk-test" || r.Header.Get("User-Agent") != "hellobro/test" || r.Header.Get("x-opencode-session") != "s1" {
			t.Errorf("%s: headers %v", r.URL.Path, r.Header)
		}
		hasPic := strings.Contains(string(body), "iVBOR") // the PNG's start, in base64
		switch r.URL.Path {
		case "/anthropic/v1/messages":
			if r.Header.Get("x-api-key") != "sk-test" || req["system"] != "be a diary" || !hasPic || !strings.Contains(string(body), `"media_type":"image/png"`) {
				t.Errorf("anthropic: %s", body)
			}
			io.WriteString(w, `{"content":[{"type":"text","text":"{\"status\":"},{"type":"text","text":"\"wait\"}"}],"usage":{"input_tokens":11,"output_tokens":14,"opencode":{"list_cost":0.0000081,"allowance":0.0000324}}}`)
		case "/openai/v1/chat/completions":
			if !hasPic || !strings.Contains(string(body), `"role":"system"`) || !strings.Contains(string(body), "data:image/png;base64,") {
				t.Errorf("openai: %s", body)
			}
			io.WriteString(w, "{\"choices\":[{\"message\":{\"content\":\"```json\\n{\\\"status\\\":\\\"answer\\\",\\\"answer\\\":\\\"Yes.\\\"}\\n```\"}}]}")
		default:
			w.WriteHeader(404)
			io.WriteString(w, `{"error":{"message":"no such model"}}`)
		}
	}))
}

func TestAsk(t *testing.T) {
	s := server(t)
	defer s.Close()
	png := []byte("\x89PNG\r\n\x1a\nrest")
	msgs := []Msg{{Role: "user", Parts: []Part{{Text: "what's written?"}, {PNG: png}}}}
	for _, model := range []string{"claude-haiku-5-5", "kimi-k3"} {
		c := Config{Key: "sk-test", Model: model, Base: s.URL, Agent: "hellobro/test", Session: "s1", MaxOut: 100}
		out, err := c.Ask(context.Background(), "be a diary", msgs)
		if err != nil {
			t.Fatalf("%s: %v", model, err)
		}
		var v struct{ Status, Answer string }
		if model != "kimi-k3" && (out.In != 11 || out.Out != 14 || out.Cost != 0.0000324) {
			t.Errorf("%s: usage %+v", model, out)
		}
		if err := JSON(out.Text, &v); err != nil {
			t.Fatalf("%s: %v", model, err)
		}
		if model == "kimi-k3" && (v.Status != "answer" || v.Answer != "Yes.") || model != "kimi-k3" && v.Status != "wait" {
			t.Errorf("%s: %+v", model, v)
		}
	}
	c := Config{Key: "sk-test", Model: "kimi-k3", Base: s.URL + "/nowhere", Agent: "hellobro/test", Session: "s1"}
	if _, err := c.Ask(context.Background(), "x", msgs); err == nil || !strings.Contains(err.Error(), "no such model") {
		t.Errorf("an error's text: %v", err)
	}
}

func TestTeXEscapes(t *testing.T) {
	var v struct{ Answer string }
	for in, want := range map[string]string{
		`{"answer":"$\frac{a}{b}$, \theta, \alpha, \nabla"}`: `$\frac{a}{b}$, \theta, \alpha, \nabla`,
		`{"answer":"line\nnext\tTab \"q\" é \\frac"}`:        "line\nnext\tTab \"q\" é \\frac",
		`{"answer":"\neq vs\nnot\nevery \( x \)"}`:           `\neq vs` + "\n" + `not` + "\n" + `every \( x \)`,
	} {
		v.Answer = ""
		if err := JSON(in, &v); err != nil || v.Answer != want {
			t.Errorf("%s: %q, %v; want %q", in, v.Answer, err, want)
		}
	}
}
