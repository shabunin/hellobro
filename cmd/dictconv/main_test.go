package main

import (
	"bytes"
	"compress/gzip"
	"io"
	"math/rand"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"hellobro/dict"
)

const sample = `{"word":"σκύλος","pos":"noun","head_templates":[{"expansion":"σκῠ́λος • (skŭ́los) n (genitive σκῠ́λεος)"}],"forms":[{"form":"σκῠ́λος","tags":["canonical"]},{"form":"skŭ́los","tags":["romanization"]},{"form":"σκῠ́λεος","tags":["genitive"]},{"form":"no-table-tags","tags":["table-tags"]},{"form":"two words"},{"form":"-"}],"senses":[{"glosses":["skin, hide"]}]}
{"word":"σκύλος","pos":"verb","senses":[{"glosses":["to <flay>","& to skin"]},{"glosses":["second"]},{"tags":["no-gloss"],"glosses":["nothing"]}]}
{"word":"nomeaning","pos":"noun","senses":[{"tags":["no-gloss"]}],"forms":[{"form":"nomeanings"}]}
not json
{"word":"Hund","pos":"noun","senses":[{"glosses":["dog"]}]}
{"word":"cut short","pos":`

func TestConvert(t *testing.T) {
	arts, err := readSource(strings.NewReader(sample))
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 2 {
		t.Errorf("%d articles", len(arts))
	}
	base := filepath.Join(t.TempDir(), "sub", "grc-en")
	if err := write(base, "Greek – English", arts); err != nil {
		t.Fatal(err)
	}
	d, err := dict.Open(base + ".ifo")
	if err != nil {
		t.Fatal(err)
	}
	if d.Name != "Greek – English" {
		t.Errorf("name %q", d.Name)
	}
	want := `<p><i>noun</i> σκῠ́λος • (skŭ́los) n (genitive σκῠ́λεος)</p><p>skin, hide</p><p><i>verb</i></p><ol><li>to &lt;flay&gt;: &amp; to skin</li><li>second</li></ol>`
	for _, word := range []string{"σκύλος", "σκῠ́λος", "σκῠ́λεος", "σκυλεος"} {
		es, err := d.Lookup(word, 3)
		if err != nil {
			t.Fatal(err)
		}
		if len(es) != 1 || es[0].Word != "σκύλος" || es[0].Fields[0].Text != want {
			t.Errorf("%s: %+v", word, es)
		}
	}
	for _, word := range []string{"skŭ́los", "no-table-tags", "nomeanings", "cut short"} {
		if es, _ := d.Lookup(word, 3); len(es) != 0 {
			t.Errorf("%s: %+v", word, es)
		}
	}
}

// What's written is dictzip, for us, and plain gzip for anyone else.
func TestDictzip(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for _, n := range []int{0, 10, dzChunk - 1, dzChunk, dzChunk + 1, 5*dzChunk + 7} {
		data := make([]byte, n)
		r.Read(data[:n/2]) // the worst for a chunk's size, and the best
		dz, err := dictzip(data)
		if err != nil {
			t.Fatal(err)
		}
		z, err := gzip.NewReader(bytes.NewReader(dz))
		if err != nil {
			t.Fatalf("%d: %v", n, err)
		}
		got, err := io.ReadAll(z)
		if err != nil {
			t.Fatalf("%d: %v", n, err)
		}
		if !slices.Equal(got, data) {
			t.Errorf("%d: not the same", n)
		}
	}
}

func TestCompareWords(t *testing.T) {
	ws := []string{"b", "Hund", "a", "hund", "α", "Z"}
	slices.SortFunc(ws, compareWords)
	if want := []string{"a", "b", "Hund", "hund", "Z", "α"}; !slices.Equal(ws, want) {
		t.Errorf("%q, want %q", ws, want)
	}
}
