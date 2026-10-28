//go:build !cgo

package pdf

import "image"

// Doc is an open PDF; without cgo there's none.
type Doc struct{}

func Open(path string) (*Doc, error) { return nil, ErrNoMuPDF }

func (d *Doc) Close()                   {}
func (d *Doc) Pages() int               { return 0 }
func (d *Doc) Title() string            { return "" }
func (d *Doc) Size(i int) (Size, error) { return Size{}, ErrNoMuPDF }
func (d *Doc) Render(i int, zoom float64, clip image.Rectangle) (*image.Gray, error) {
	return nil, ErrNoMuPDF
}
func (d *Doc) Text(i int) (string, error) { return "", ErrNoMuPDF }
func (d *Doc) Outline() []Entry           { return nil }
