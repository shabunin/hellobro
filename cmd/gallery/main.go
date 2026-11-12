// Command gallery shows ink's controls (ink/gallery) on the Kobo, an app
// of its own beside hellobro: where the components are made and tried
// before the apps move onto them. Started from NickelMenu (nm/gallery)
// through launch.sh, as brotools is.
package main

import (
	"hellobro/ink/gallery"
	"hellobro/ink/inkkobo"
	"hellobro/ink/theme"
	"hellobro/kobo"
)

var version = "dev" // set by the build (-X main.version)

func main() {
	g := gallery.New(theme.New())
	h := inkkobo.New(kobo.Config{
		Name: "gallery", Title: "ink gallery", Tabs: []string{"Gallery"},
		About: []string{"ink gallery " + version, "github.com/shabunin/hellobro"},
	}, g.Layout)
	h.Quit = func() bool { return g.Quit }
	h.Run()
}
