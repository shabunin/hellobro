//go:build !cgo

package kobo

import "errors"

// Without cgo there's no linked-in FBInk: always use the CLI.
func openFbinkLib() (fbBackend, error) { return nil, errors.New("built without cgo") }
