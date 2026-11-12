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
)

// Model is one the API offers: its id, its name, whether it reads
// pictures (Images; unknown if the list doesn't say), and its price, in
// dollars a million tokens in and out (-1: not said).
type Model struct {
	ID, Name      string
	Images, Known bool
	In, Out       float64
	Proto         string // as Config.Proto has it, if the list says; "-": none we speak (Google's)
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

func parseModels(body []byte) ([]Model, error) {
	var list struct {
		Data []struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			Type       string `json:"type"`
			Modalities *struct {
				Input []string `json:"input"`
			} `json:"modalities"`
			Cost *struct {
				Text struct {
					Input  struct{ Price float64 } `json:"input"`
					Output struct{ Price float64 } `json:"output"`
				} `json:"text"`
			} `json:"cost"`
			API          map[string]json.RawMessage `json:"api"`
			Architecture *struct {
				Input []string `json:"input_modalities"`
			} `json:"architecture"`
			Pricing *struct {
				Prompt     string `json:"prompt"`
				Completion string `json:"completion"`
			} `json:"pricing"`
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
		m := Model{ID: d.ID, Name: d.Name, In: -1, Out: -1}
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
		m.Images = slices.ContainsFunc(input, func(s string) bool { return strings.HasPrefix(s, "image") })
		switch {
		case d.Cost != nil:
			m.In, m.Out = d.Cost.Text.Input.Price, d.Cost.Text.Output.Price
		case d.Pricing != nil:
			m.In, m.Out = perMillion(d.Pricing.Prompt), perMillion(d.Pricing.Completion)
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
