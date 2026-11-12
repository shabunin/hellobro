package ai

import (
	"strings"
	"testing"
)

func TestParseModels(t *testing.T) {
	opencode := `{"object":"list","data":[
		{"id":"big","type":"chat","api":{"responses":{}},"modalities":{"input":["text","image/png"]},"cost":{"text":{"input":{"price":2},"output":{"price":6}}}},
		{"id":"words","type":"chat","modalities":{"input":["text"]},"cost":{"text":{"input":{"price":0.1},"output":{"price":0.2}}}},
		{"id":"small","type":"chat","modalities":{"input":["text","image/jpeg"]},"cost":{"text":{"input":{"price":0.1},"output":{"price":0.5}}}},
		{"id":"embed","type":"embedding"}]}`
	ms, err := parseModels([]byte(opencode))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, m := range ms {
		got = append(got, m.ID)
	}
	if strings.Join(got, " ") != "words small big" || ms[0].Images || !ms[1].Images || !ms[1].Known || ms[2].Out != 6 || ms[2].Proto != "responses" {
		t.Errorf("opencode: %+v", ms)
	}
	openrouter := `{"data":[{"id":"anthropic/claude-x","name":"Claude X","architecture":{"input_modalities":["text","image"]},
		"pricing":{"prompt":"0.000001","completion":"0.000005"}},{"id":"other/thing","name":"Thing"}]}`
	ms, err = parseModels([]byte(openrouter))
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 2 || ms[0].ID != "anthropic/claude-x" || !ms[0].Images || ms[0].In != 1 || ms[0].Out != 5 || ms[1].Known || ms[1].In != -1 {
		t.Errorf("openrouter: %+v", ms)
	}
}

func TestURL(t *testing.T) {
	oc := Config{Base: Base, Model: "claude-haiku-5-5"}
	if u := oc.url("openai", "/models"); u != Base+"/openai/v1/models" || !oc.anthropic() {
		t.Errorf("opencode: %s", u)
	}
	or := Config{Base: "https://openrouter.ai/api/v1", Model: "anthropic/claude-x"}
	if u := or.url("openai", "/chat/completions"); u != "https://openrouter.ai/api/v1/chat/completions" || or.anthropic() {
		t.Errorf("openrouter: %s", u)
	}
}

func TestResponses(t *testing.T) {
	c := Config{Model: "gpt-x", MaxOut: 100}
	b := responsesBody(c, "be brief", []Msg{{Role: "user", Parts: []Part{{PNG: []byte{1}}, {Text: "hi"}}}, {Role: "assistant", Parts: []Part{{Text: "yo"}}}})
	in := b["input"].([]map[string]any)
	if b["instructions"] != "be brief" || len(in) != 2 || in[0]["content"].([]map[string]any)[0]["type"] != "input_image" ||
		in[1]["content"].([]map[string]any)[0]["type"] != "output_text" {
		t.Errorf("%v", b)
	}
	s, err := responsesText([]byte(`{"output":[{"type":"reasoning","content":[]},{"type":"message","content":[{"type":"output_text","text":"hello"}]}]}`))
	if err != nil || s != "hello" {
		t.Errorf("%q %v", s, err)
	}
}
