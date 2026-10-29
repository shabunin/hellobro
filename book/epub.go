// Package book reads an EPUB: its spine, its contents, and each chapter
// as blocks of styled text and pictures, then lays those out in pages
// (layout.go). No drawing here: hellobro's reader window draws the pages.
package book

import (
	"archive/zip"
	"encoding/xml"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/url"
	"path"
	"strings"

	_ "golang.org/x/image/webp"
)

// Book is an open EPUB.
type Book struct {
	Title, Author string
	Lang          string   // dc:language: "en", "en-US"…
	Spine         []string // the chapters' files, in reading order
	TOC           []TOCEntry
	z             *zip.ReadCloser
	files         map[string]*zip.File
	css           map[string]*sheet // stylesheets read so far
}

// TOCEntry is a line of the contents: where it goes, how deep it is.
type TOCEntry struct {
	Title string
	Spine int    // the chapter
	Frag  string // the anchor in it, "" for its start
	Depth int
}

// Open opens the EPUB at name.
func Open(name string) (*Book, error) {
	z, err := zip.OpenReader(name)
	if err != nil {
		return nil, err
	}
	b := &Book{z: z, files: map[string]*zip.File{}, css: map[string]*sheet{}}
	for _, f := range z.File {
		b.files[f.Name] = f
	}
	if err := b.readOPF(); err != nil {
		z.Close()
		return nil, err
	}
	return b, nil
}

func (b *Book) Close() error { return b.z.Close() }

// read reads the file name in the zip, at most 64 MB of it.
func (b *Book) read(name string) ([]byte, error) {
	f := b.files[name]
	if f == nil {
		return nil, fmt.Errorf("%s: not in the book", name)
	}
	r, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(io.LimitReader(r, 64<<20))
}

// resolve is href, as written in the file at from, as a path in the zip,
// and its fragment.
func resolve(from, href string) (string, string) {
	href, frag, _ := strings.Cut(href, "#")
	if u, err := url.PathUnescape(href); err == nil {
		href = u
	}
	if href == "" {
		return from, frag
	}
	if strings.HasPrefix(href, "/") {
		return strings.TrimPrefix(path.Clean(href), "/"), frag
	}
	return path.Join(path.Dir(from), href), frag
}

func (b *Book) readOPF() error {
	data, err := b.read("META-INF/container.xml")
	if err != nil {
		return err
	}
	var c struct {
		Rootfiles []struct {
			Path string `xml:"full-path,attr"`
		} `xml:"rootfiles>rootfile"`
	}
	if err := xml.Unmarshal(data, &c); err != nil {
		return err
	}
	if len(c.Rootfiles) == 0 {
		return errors.New("no package file")
	}
	opf := c.Rootfiles[0].Path
	if data, err = b.read(opf); err != nil {
		return err
	}
	var p struct {
		Title   []string `xml:"metadata>title"`
		Creator []string `xml:"metadata>creator"`
		Lang    []string `xml:"metadata>language"`
		Items   []struct {
			ID    string `xml:"id,attr"`
			Href  string `xml:"href,attr"`
			Type  string `xml:"media-type,attr"`
			Props string `xml:"properties,attr"`
		} `xml:"manifest>item"`
		Spine struct {
			TOC   string `xml:"toc,attr"`
			Items []struct {
				IDRef  string `xml:"idref,attr"`
				Linear string `xml:"linear,attr"`
			} `xml:"itemref"`
		} `xml:"spine"`
	}
	if err := xml.Unmarshal(data, &p); err != nil {
		return err
	}
	if len(p.Title) > 0 {
		b.Title = strings.TrimSpace(p.Title[0])
	}
	if len(p.Lang) > 0 {
		b.Lang = strings.TrimSpace(p.Lang[0])
	}
	if len(p.Creator) > 0 {
		b.Author = strings.TrimSpace(p.Creator[0])
	}
	byID := map[string]string{}
	var nav, ncx string
	for _, it := range p.Items {
		href, _ := resolve(opf, it.Href)
		byID[it.ID] = href
		if strings.Contains(" "+it.Props+" ", " nav ") {
			nav = href
		}
		if it.ID == p.Spine.TOC || (ncx == "" && it.Type == "application/x-dtbncx+xml") {
			ncx = href
		}
	}
	for _, it := range p.Spine.Items {
		if href, ok := byID[it.IDRef]; ok && it.Linear != "no" {
			b.Spine = append(b.Spine, href)
		}
	}
	if len(b.Spine) == 0 {
		return errors.New("nothing to read in it")
	}
	if nav != "" {
		b.readNav(nav)
	}
	if len(b.TOC) == 0 && ncx != "" {
		b.readNCX(ncx)
	}
	return nil
}

// spineOf is the chapter that file is, -1 if none.
func (b *Book) spineOf(file string) int {
	for i, s := range b.Spine {
		if s == file {
			return i
		}
	}
	return -1
}

func (b *Book) addTOC(from, title, href string, depth int) {
	title = strings.Join(strings.Fields(title), " ")
	file, frag := resolve(from, href)
	if i := b.spineOf(file); i >= 0 && title != "" {
		b.TOC = append(b.TOC, TOCEntry{title, i, frag, depth})
	}
}

// readNav reads EPUB 3's contents: the toc nav's links, nested in lists.
func (b *Book) readNav(name string) {
	data, err := b.read(name)
	if err != nil {
		return
	}
	d := newDecoder(data)
	inTOC, navDepth, depth := false, 0, 0
	var href, text string
	inA := false
	for {
		tok, err := d.Token()
		if err != nil {
			return
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "nav":
				navDepth++
				if attr(t, "type") == "toc" || (!inTOC && len(b.TOC) == 0 && attr(t, "type") == "") {
					inTOC = true
				}
			case "ol", "ul":
				depth++
			case "a":
				if inTOC {
					inA, href, text = true, attr(t, "href"), ""
				}
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "nav":
				navDepth--
				if inTOC && len(b.TOC) > 0 {
					return
				}
				inTOC = false
			case "ol", "ul":
				depth--
			case "a":
				if inA {
					b.addTOC(name, text, href, max(0, depth-1))
					inA = false
				}
			}
		case xml.CharData:
			if inA {
				text += string(t)
			}
		}
	}
}

// readNCX reads EPUB 2's contents.
func (b *Book) readNCX(name string) {
	data, err := b.read(name)
	if err != nil {
		return
	}
	type navPoint struct {
		Label   string `xml:"navLabel>text"`
		Content struct {
			Src string `xml:"src,attr"`
		} `xml:"content"`
		Points []navPoint `xml:"navPoint"`
	}
	var n struct {
		Points []navPoint `xml:"navMap>navPoint"`
	}
	if err := xml.Unmarshal(data, &n); err != nil {
		return
	}
	var add func(ps []navPoint, depth int)
	add = func(ps []navPoint, depth int) {
		for _, p := range ps {
			b.addTOC(name, p.Label, p.Content.Src, depth)
			add(p.Points, depth+1)
		}
	}
	add(n.Points, 0)
}

// MaxPicture is the most pixels a picture is decoded with: a bigger one
// (a camera's, or a header that says so) would be hundreds of MB.
const MaxPicture = 16 << 20

// Image is the picture at src (a path in the zip, as a Block has it).
func (b *Book) Image(src string) (image.Image, error) {
	f := b.files[src]
	if f == nil {
		return nil, fmt.Errorf("%s: not in the book", src)
	}
	size, err := b.ImageSize(src)
	if err != nil {
		return nil, err
	}
	if size.X <= 0 || size.Y <= 0 || size.X > MaxPicture/size.Y {
		return nil, fmt.Errorf("%s: %d × %d, too big a picture", src, size.X, size.Y)
	}
	r, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	m, _, err := image.Decode(r)
	return m, err
}

// ImageSize is the picture's size, without decoding all of it.
func (b *Book) ImageSize(src string) (image.Point, error) {
	f := b.files[src]
	if f == nil {
		return image.Point{}, fmt.Errorf("%s: not in the book", src)
	}
	r, err := f.Open()
	if err != nil {
		return image.Point{}, err
	}
	defer r.Close()
	c, _, err := image.DecodeConfig(r)
	return image.Pt(c.Width, c.Height), err
}

func newDecoder(data []byte) *xml.Decoder {
	d := xml.NewDecoder(strings.NewReader(string(data)))
	d.Strict = false
	d.AutoClose = xml.HTMLAutoClose
	d.Entity = xml.HTMLEntity
	d.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }
	return d
}

func attr(t xml.StartElement, name string) string {
	for _, a := range t.Attr {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}
