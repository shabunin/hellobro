package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestBookFamilies: the Kobo's fonts as families of four styles, Georgia
// first; one-style fonts (CJK, the UI's) left out; the Go fonts last.
func TestBookFamilies(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"georgia.ttf", "georgiab.ttf", "georgiai.ttf", "georgiaz.ttf",
		"KoboNickel.ttf", "KoboNickel-Bold.ttf", "KoboNickel-Italic.ttf", "KoboNickel-BoldItalic.ttf",
		"RakutenSerifApp-Regular.ttf", "RakutenSerifApp-Bold.ttf", "RakutenSerifApp-Italic.ttf",
		"Ryumin.otf", "Gothic MB101.otf", "readme.txt"} {
		os.WriteFile(filepath.Join(dir, n), nil, 0o644)
	}
	t.Setenv("HELLOBRO_FONTS", dir)
	var got []string
	for _, f := range bookFamilies() {
		var files []string
		for _, p := range f.files {
			files = append(files, filepath.Base(p))
		}
		got = append(got, f.name+" "+fmt.Sprint(files))
	}
	want := []string{
		"Georgia [georgia.ttf georgiab.ttf georgiai.ttf georgiaz.ttf]",
		"Kobo Nickel [KoboNickel.ttf KoboNickel-Bold.ttf KoboNickel-Italic.ttf KoboNickel-BoldItalic.ttf]",
		"Rakuten Serif [RakutenSerifApp-Regular.ttf RakutenSerifApp-Bold.ttf RakutenSerifApp-Italic.ttf .]",
		"Go [. . . .]",
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("families:\n%v\nwant\n%v", got, want)
	}
}
