//go:build cgo

package pdf

/*
#cgo CFLAGS: -I${SRCDIR}/../third_party/mupdf/include
#cgo LDFLAGS: ${SRCDIR}/../third_party/mupdf/libmupdf.a ${SRCDIR}/../third_party/mupdf/libmupdf-third.a -lm
#include <stdlib.h>
#include <string.h>
#include <mupdf/fitz.h>

// MuPDF throws (longjmp) where it fails: each call is wrapped here in its
// fz_try, so the jumps never cross Go's frames, and what went wrong comes
// back as a string, to be freed.

typedef struct { fz_context *ctx; fz_document *doc; } hb_doc;

static char *hb_err(fz_context *ctx) { return strdup(fz_caught_message(ctx)); }

static hb_doc *hb_open(const char *path, size_t store, char **err) {
	fz_context *ctx = fz_new_context(NULL, NULL, store);
	if (!ctx) {
		*err = strdup("no memory for MuPDF");
		return NULL;
	}
	fz_document *doc = NULL;
	fz_var(doc);
	fz_try(ctx) {
		fz_register_document_handlers(ctx);
		doc = fz_open_document(ctx, path);
		if (fz_needs_password(ctx, doc) && !fz_authenticate_password(ctx, doc, ""))
			fz_throw(ctx, FZ_ERROR_GENERIC, "it's locked with a password");
		fz_count_pages(ctx, doc); // a broken file fails here, not later
	} fz_catch(ctx) {
		*err = hb_err(ctx);
		fz_drop_document(ctx, doc);
		fz_drop_context(ctx);
		return NULL;
	}
	hb_doc *d = calloc(1, sizeof *d);
	d->ctx = ctx;
	d->doc = doc;
	return d;
}

static void hb_close(hb_doc *d) {
	fz_drop_document(d->ctx, d->doc);
	fz_drop_context(d->ctx);
	free(d);
}

static int hb_pages(hb_doc *d) {
	int n = 0;
	fz_try(d->ctx) n = fz_count_pages(d->ctx, d->doc);
	fz_catch(d->ctx) n = 0;
	return n;
}

static fz_rect hb_bounds(hb_doc *d, int i, char **err) {
	fz_rect r = fz_empty_rect;
	fz_page *page = NULL;
	fz_var(page);
	fz_try(d->ctx) {
		page = fz_load_page(d->ctx, d->doc, i);
		r = fz_bound_page(d->ctx, page);
	} fz_always(d->ctx) {
		fz_drop_page(d->ctx, page);
	} fz_catch(d->ctx) {
		*err = hb_err(d->ctx);
	}
	return r;
}

// hb_render draws page i, zoom times its size in points, its top left at
// 0,0: the part of it in clip, into samples (clip's size, a byte a pixel).
static void hb_render(hb_doc *d, int i, float zoom, fz_irect clip, unsigned char *samples, char **err) {
	fz_context *ctx = d->ctx;
	fz_page *page = NULL;
	fz_pixmap *pix = NULL;
	fz_device *dev = NULL;
	fz_var(page);
	fz_var(pix);
	fz_var(dev);
	fz_try(ctx) {
		page = fz_load_page(ctx, d->doc, i);
		fz_rect b = fz_bound_page(ctx, page);
		fz_matrix m = fz_pre_translate(fz_scale(zoom, zoom), -b.x0, -b.y0);
		pix = fz_new_pixmap_with_bbox_and_data(ctx, fz_device_gray(ctx), clip, NULL, 0, samples);
		fz_clear_pixmap_with_value(ctx, pix, 255);
		dev = fz_new_draw_device(ctx, fz_identity, pix);
		fz_run_page(ctx, page, dev, m, NULL);
		fz_close_device(ctx, dev);
	} fz_always(ctx) {
		fz_drop_device(ctx, dev);
		fz_drop_pixmap(ctx, pix);
		fz_drop_page(ctx, page);
	} fz_catch(ctx) {
		*err = hb_err(ctx);
	}
}

static char *hb_text(hb_doc *d, int i, char **err) {
	fz_buffer *buf = NULL;
	char *s = NULL;
	fz_var(buf);
	fz_try(d->ctx) {
		buf = fz_new_buffer_from_page_number(d->ctx, d->doc, i, NULL);
		s = strdup(fz_string_from_buffer(d->ctx, buf));
	} fz_always(d->ctx) {
		fz_drop_buffer(d->ctx, buf);
	} fz_catch(d->ctx) {
		*err = hb_err(d->ctx);
	}
	return s;
}

// hb_chars are page i's characters, in reading order, each with its box
// (in points, from the page's top left) and its line: *n of them, to be
// freed.
typedef struct { int c, line; float x0, y0, x1, y1; } hb_char;

static hb_char *hb_chars(hb_doc *d, int i, int *n, char **err) {
	fz_context *ctx = d->ctx;
	fz_page *page = NULL;
	fz_stext_page *st = NULL;
	hb_char *out = NULL;
	fz_var(page);
	fz_var(st);
	fz_var(out);
	*n = 0;
	fz_try(ctx) {
		page = fz_load_page(ctx, d->doc, i);
		fz_rect b = fz_bound_page(ctx, page);
		st = fz_new_stext_page_from_page(ctx, page, NULL);
		int cap = 0, line = 0;
		for (fz_stext_block *bl = st->first_block; bl; bl = bl->next) {
			if (bl->type != FZ_STEXT_BLOCK_TEXT)
				continue;
			for (fz_stext_line *ln = bl->u.t.first_line; ln; ln = ln->next, line++) {
				for (fz_stext_char *ch = ln->first_char; ch; ch = ch->next) {
					if (*n == cap) {
						cap = cap ? 2 * cap : 1024;
						hb_char *more = realloc(out, cap * sizeof *out);
						if (!more)
							fz_throw(ctx, FZ_ERROR_GENERIC, "out of memory");
						out = more;
					}
					fz_rect r = fz_rect_from_quad(ch->quad);
					out[(*n)++] = (hb_char){ch->c, line, r.x0 - b.x0, r.y0 - b.y0, r.x1 - b.x0, r.y1 - b.y0};
				}
			}
		}
	} fz_always(ctx) {
		fz_drop_stext_page(ctx, st);
		fz_drop_page(ctx, page);
	} fz_catch(ctx) {
		*err = hb_err(ctx);
		free(out);
		out = NULL;
		*n = 0;
	}
	return out;
}

static fz_outline *hb_outline(hb_doc *d) {
	fz_outline *o = NULL;
	fz_try(d->ctx) o = fz_load_outline(d->ctx, d->doc);
	fz_catch(d->ctx) o = NULL;
	return o;
}

static int hb_page_of(hb_doc *d, fz_outline *o) {
	int n = -1;
	fz_try(d->ctx) n = fz_page_number_from_location(d->ctx, d->doc, o->page);
	fz_catch(d->ctx) n = -1;
	return n;
}

static void hb_drop_outline(hb_doc *d, fz_outline *o) { fz_drop_outline(d->ctx, o); }

static char *hb_title(hb_doc *d) {
	char buf[512];
	int n = -1;
	fz_try(d->ctx) n = fz_lookup_metadata(d->ctx, d->doc, FZ_META_INFO_TITLE, buf, sizeof buf);
	fz_catch(d->ctx) n = -1;
	return n > 0 ? strdup(buf) : NULL;
}
*/
import "C"

import (
	"errors"
	"image"
	"sync"
	"unsafe"
)

// storeMax is how much MuPDF keeps of what it's read (fonts, pictures)
// for the pages after: the Kobo has 850 MB in all.
const storeMax = 48 << 20

// Doc is an open PDF. Its calls may come from any goroutine, one at a
// time: a context of MuPDF's is for one thread at once. Closed, it says
// so (a page drawn ahead may still be on its way).
type Doc struct {
	mu sync.Mutex
	d  *C.hb_doc
	n  int
}

func cErr(err *C.char) error {
	if err == nil {
		return nil
	}
	defer C.free(unsafe.Pointer(err))
	return errors.New(C.GoString(err))
}

// Open opens the PDF at path.
func Open(path string) (*Doc, error) {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	var cerr *C.char
	d := C.hb_open(cpath, storeMax, &cerr)
	if d == nil {
		return nil, cErr(cerr)
	}
	return &Doc{d: d, n: int(C.hb_pages(d))}, nil
}

func (d *Doc) Close() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.d != nil {
		C.hb_close(d.d)
		d.d = nil
	}
}

var errClosed = errors.New("the PDF is closed")

// Pages is how many pages it has.
func (d *Doc) Pages() int { return d.n }

// Title is its title, as its metadata has it, if it does.
func (d *Doc) Title() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.d == nil {
		return ""
	}
	s := C.hb_title(d.d)
	if s == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(s))
	return C.GoString(s)
}

// Size is page i's size.
func (d *Doc) Size(i int) (Size, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.d == nil {
		return Size{}, errClosed
	}
	var cerr *C.char
	r := C.hb_bounds(d.d, C.int(i), &cerr)
	if err := cErr(cerr); err != nil {
		return Size{}, err
	}
	return Size{float64(r.x1 - r.x0), float64(r.y1 - r.y0)}, nil
}

// Render draws page i zoom times its size, gray: the part of it in clip
// (in the drawn page's pixels, its top left at 0,0), which is the image's
// bounds.
func (d *Doc) Render(i int, zoom float64, clip image.Rectangle) (*image.Gray, error) {
	m := image.NewGray(clip)
	if clip.Empty() {
		return m, nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.d == nil {
		return m, errClosed
	}
	var cerr *C.char
	r := C.fz_irect{x0: C.int(clip.Min.X), y0: C.int(clip.Min.Y), x1: C.int(clip.Max.X), y1: C.int(clip.Max.Y)}
	C.hb_render(d.d, C.int(i), C.float(zoom), r, (*C.uchar)(unsafe.Pointer(&m.Pix[0])), &cerr)
	return m, cErr(cerr)
}

// Text is page i's text, as MuPDF reads it.
func (d *Doc) Text(i int) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.d == nil {
		return "", errClosed
	}
	var cerr *C.char
	s := C.hb_text(d.d, C.int(i), &cerr)
	if err := cErr(cerr); err != nil {
		return "", err
	}
	defer C.free(unsafe.Pointer(s))
	return C.GoString(s), nil
}

// Chars are page i's characters, in reading order.
func (d *Doc) Chars(i int) ([]Char, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.d == nil {
		return nil, errClosed
	}
	var cerr *C.char
	var n C.int
	cs := C.hb_chars(d.d, C.int(i), &n, &cerr)
	if err := cErr(cerr); err != nil {
		return nil, err
	}
	if cs == nil {
		return nil, nil
	}
	defer C.free(unsafe.Pointer(cs))
	out := make([]Char, int(n))
	for k, c := range unsafe.Slice(cs, int(n)) {
		out[k] = Char{R: rune(c.c), Line: int(c.line), Box: Rect{float64(c.x0), float64(c.y0), float64(c.x1), float64(c.y1)}}
	}
	return out, nil
}

// Outline is its outline (the contents), in order, if it has one.
func (d *Doc) Outline() []Entry {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.d == nil {
		return nil
	}
	o := C.hb_outline(d.d)
	if o == nil {
		return nil
	}
	defer C.hb_drop_outline(d.d, o)
	var es []Entry
	var walk func(o *C.fz_outline, depth int)
	walk = func(o *C.fz_outline, depth int) {
		for ; o != nil && len(es) < 5000; o = o.next {
			if p := int(C.hb_page_of(d.d, o)); p >= 0 && o.title != nil {
				es = append(es, Entry{Title: C.GoString(o.title), Page: p, Depth: depth})
			}
			walk(o.down, depth+1)
		}
	}
	walk(o, 0)
	return es
}
