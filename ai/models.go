package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Model is one the API offers: its id, its name, what it reads (Images:
// pictures; Known says the list said), what it can do, its limits, and
// its price, in dollars a million tokens in and out (-1: not said).
type Model struct {
	ID, Name      string
	Images, Known bool
	In, Out       float64
	Proto         string // as Config.Proto has it, if the list says; "-": none we speak (Google's)

	About       string  `json:",omitempty"` // what the list says of it
	Family      string  `json:",omitempty"`
	PDF         bool    `json:",omitempty"` // reads PDFs
	Audio       bool    `json:",omitempty"` // hears sound
	Video       bool    `json:",omitempty"` // watches videos
	Reasoning   bool    `json:",omitempty"` // thinks before it answers
	Tools       bool    `json:",omitempty"` // calls tools
	JSON        bool    `json:",omitempty"` // answers in a JSON schema given
	OpenWeights bool    `json:",omitempty"`
	Context     int     `json:",omitempty"` // tokens it takes in, in all
	MaxOut      int     `json:",omitempty"` // tokens it writes at most
	Cached      float64 `json:",omitempty"` // dollars a million tokens read from the cache (0: not said)
	Peak        bool    `json:",omitempty"` // dearer at some hours
	Knowledge   string  `json:",omitempty"` // what it knows is up to: "2025-05"
	Released    string  `json:",omitempty"` // "2026-10-17"
}

// Features are what it reads and can do, in a few words.
func (m Model) Features() []string {
	var fs []string
	for _, f := range []struct {
		on bool
		s  string
	}{{m.Images, "pictures"}, {m.PDF, "PDF"}, {m.Audio, "audio"}, {m.Video, "video"},
		{m.Reasoning, "reasoning"}, {m.Tools, "tools"}, {m.JSON, "JSON"}, {m.OpenWeights, "open weights"}} {
		if f.on {
			fs = append(fs, f.s)
		}
	}
	return fs
}

// Models are the models the API lists, as OpenCode's list has them
// (modalities, cost) or OpenRouter's (architecture, pricing a token):
// the cheapest first.
func (c Config) Models(ctx context.Context) ([]Model, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.url("openai", "/models"), nil)
	if err != nil {
		return nil, err
	}
	if c.Key != "" {
		req.Header.Set("Authorization", "Bearer "+c.Key)
	}
	if c.Agent != "" {
		req.Header.Set("User-Agent", c.Agent)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", resp.Status, errorText(body))
	}
	ms, err := parseModels(body)
	if err == nil && len(ms) == 0 {
		err = errors.New("it lists no models")
	}
	return ms, err
}

// supported is a capability as OpenCode's list says it: true, or
// {"supported": true}.
type supported bool

func (s *supported) UnmarshalJSON(b []byte) error {
	var v struct{ Supported bool }
	if json.Unmarshal(b, &v) == nil {
		*s = supported(v.Supported)
		return nil
	}
	var on bool
	json.Unmarshal(b, &on)
	*s = supported(on)
	return nil
}

func parseModels(body []byte) ([]Model, error) {
	var list struct {
		Data []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Type        string `json:"type"`
			Description string `json:"description"`
			Family      string `json:"family"`
			OpenWeights bool   `json:"open_weights"`
			Knowledge   string `json:"knowledge"`
			Release     string `json:"release_date"`
			Created     int64  `json:"created"`
			Modalities  *struct {
				Input []string `json:"input"`
			} `json:"modalities"`
			Caps *struct {
				Tools      supported `json:"tools"`
				Reasoning  supported `json:"reasoning"`
				Structured supported `json:"structured_output"`
			} `json:"capabilities"`
			Limit *struct {
				Context int `json:"context"`
				Output  int `json:"output"`
			} `json:"limit"`
			Cost *struct {
				Text struct {
					Input struct {
						Price float64
						Cache struct{ Read float64 }
					} `json:"input"`
					Output struct{ Price float64 } `json:"output"`
				} `json:"text"`
				Multipliers []json.RawMessage `json:"multipliers"`
			} `json:"cost"`
			API map[string]json.RawMessage `json:"api"`

			// OpenRouter's
			Architecture *struct {
				Input []string `json:"input_modalities"`
			} `json:"architecture"`
			Pricing *struct {
				Prompt     string `json:"prompt"`
				Completion string `json:"completion"`
				CacheRead  string `json:"input_cache_read"`
			} `json:"pricing"`
			ContextLength int      `json:"context_length"`
			Params        []string `json:"supported_parameters"`
			TopProvider   *struct {
				MaxOut int `json:"max_completion_tokens"`
			} `json:"top_provider"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("the models' list: %w", err)
	}
	var ms []Model
	for _, d := range list.Data {
		if d.ID == "" || d.Type != "" && d.Type != "chat" {
			continue
		}
		m := Model{ID: d.ID, Name: d.Name, In: -1, Out: -1, About: d.Description, Family: d.Family,
			OpenWeights: d.OpenWeights, Knowledge: d.Knowledge, Released: d.Release, Context: d.ContextLength}
		var input []string
		switch {
		case d.Modalities != nil:
			input, m.Known = d.Modalities.Input, true
		case d.Architecture != nil:
			input, m.Known = d.Architecture.Input, true
		}
		for k := range d.API { // OpenCode's: how each is reached
			switch k {
			case "messages":
				m.Proto = "anthropic"
			case "chat-completions":
				m.Proto = "openai"
			case "responses":
				m.Proto = "responses"
			default:
				if m.Proto == "" {
					m.Proto = "-"
				}
			}
		}
		has := func(p string) bool {
			return slices.ContainsFunc(input, func(s string) bool { return strings.HasPrefix(s, p) })
		}
		m.Images, m.Audio, m.Video = has("image"), has("audio"), has("video")
		m.PDF = has("application/pdf") || has("file")
		if c := d.Caps; c != nil {
			m.Tools, m.Reasoning, m.JSON = bool(c.Tools), bool(c.Reasoning), bool(c.Structured)
		}
		if len(d.Params) > 0 {
			m.Tools = slices.Contains(d.Params, "tools")
			m.Reasoning = slices.Contains(d.Params, "reasoning")
			m.JSON = slices.Contains(d.Params, "structured_outputs")
		}
		if d.Limit != nil {
			m.Context, m.MaxOut = d.Limit.Context, d.Limit.Output
		}
		if d.TopProvider != nil && m.MaxOut == 0 {
			m.MaxOut = d.TopProvider.MaxOut
		}
		if m.Released == "" && d.Created > 0 && d.Pricing != nil { // OpenRouter's: when it came
			m.Released = time.Unix(d.Created, 0).UTC().Format("2006-01-02")
		}
		switch {
		case d.Cost != nil:
			m.In, m.Out = d.Cost.Text.Input.Price, d.Cost.Text.Output.Price
			m.Cached, m.Peak = d.Cost.Text.Input.Cache.Read, len(d.Cost.Multipliers) > 0
		case d.Pricing != nil:
			m.In, m.Out = perMillion(d.Pricing.Prompt), perMillion(d.Pricing.Completion)
			m.Cached = max(perMillion(d.Pricing.CacheRead), 0)
		}
		ms = append(ms, m)
	}
	slices.SortStableFunc(ms, func(a, b Model) int {
		pa, pb := a.In+a.Out, b.In+b.Out
		switch {
		case pa < 0 && pb >= 0:
			return 1
		case pb < 0 && pa >= 0:
			return -1
		case pa < pb:
			return -1
		case pa > pb:
			return 1
		}
		return strings.Compare(a.ID, b.ID)
	})
	return ms, nil
}

// perMillion is a price a token, as a string, a million tokens.
func perMillion(s string) float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f < 0 {
		return -1
	}
	return f * 1e6
}
