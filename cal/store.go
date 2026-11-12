package cal

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Store is the calendars kept on the device, in Dir: a folder each, an
// .ics file per item, and state.json, what we know of the server's
// copies. It's what the calendar shows; Sync keeps it in step with a
// server's. Its methods are safe to call while a Sync runs.
type Store struct {
	Dir  string
	mu   sync.Mutex
	cals []*Folder
	ver  int // changed each time anything is

	between struct { // Between's last answer
		from, to time.Time
		ver      int
		occ      []Occurrence
	}
}

// Folder is a calendar on the device.
type Folder struct {
	ID      string            `json:"id"`   // its folder's name
	Href    string            `json:"href"` // on the server; "": ours alone
	Name    string            `json:"name"`
	Comps   []string          `json:"comps,omitempty"`
	Sub     bool              `json:"sub,omitempty"`  // subscribed to: an .ics at Href, read only (its items in feed.ics)
	Hash    string            `json:"hash,omitempty"` // a subscription's, as last fetched
	CTag    string            `json:"ctag,omitempty"`
	Entries map[string]*Entry `json:"entries"` // by file name
}

// Holds says the calendar takes items of kind.
func (f *Folder) Holds(kind string) bool { return Calendar{Comps: f.Comps}.Holds(kind) }

// ReadOnly says its items can't be changed here.
func (f *Folder) ReadOnly() bool { return f.Sub }

// Entry is an item in a calendar on the device.
type Entry struct {
	Href  string    `json:"href,omitempty"` // on the server; "": not there yet
	ETag  string    `json:"etag,omitempty"`
	Dirty int       `json:"dirty,omitempty"` // changed here n times since the server had it
	Gone  bool      `json:"gone,omitempty"`  // deleted here, still to go from the server
	When  time.Time `json:"when,omitempty"`  // when it was deleted
	Item  *Item     `json:"-"`
	File  string    `json:"-"`
	In    *Folder   `json:"-"`
}

type state struct {
	Cals []*Folder `json:"cals"`
}

// Open reads the store in dir, made if it's not there; with one calendar
// of our own if it has none.
func Open(dir string) (*Store, error) {
	s := &Store{Dir: dir}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	var st state
	if len(b) > 0 {
		if err := json.Unmarshal(b, &st); err != nil {
			return nil, fmt.Errorf("state.json: %w", err)
		}
	}
	for _, f := range st.Cals {
		if f.Entries == nil {
			f.Entries = map[string]*Entry{}
		}
		if f.Sub {
			if data, err := os.ReadFile(filepath.Join(dir, f.ID, feedFile)); err == nil {
				if c, err := ParseString(string(data)); err == nil {
					f.fill(c)
				}
			}
			s.cals = append(s.cals, f)
			continue
		}
		for name, e := range f.Entries {
			e.File, e.In = name, f
			data, err := os.ReadFile(filepath.Join(dir, f.ID, name))
			if err != nil {
				delete(f.Entries, name)
				continue
			}
			if c, err := ParseString(string(data)); err == nil {
				e.Item, _ = ItemOf(c)
			}
			if e.Item == nil {
				delete(f.Entries, name)
			}
		}
		s.cals = append(s.cals, f)
	}
	if len(s.cals) == 0 {
		s.cals = []*Folder{{ID: "local", Name: "calendar", Entries: map[string]*Entry{}}}
	}
	return s, nil
}

// saveState writes state.json; s.mu held.
func (s *Store) saveState() error {
	s.ver++
	var st state
	for _, f := range s.cals {
		if f.Sub { // its items are its feed's
			cp := *f
			cp.Entries = nil
			f = &cp
		}
		st.Cals = append(st.Cals, f)
	}
	b, err := json.MarshalIndent(st, "", "\t")
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(s.Dir, "state.json"), b)
}

// writeFile writes name whole or not at all: a new file put over it.
func writeFile(name string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return err
	}
	tmp := name + ".new"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, name)
}

// fileOf is the file name for an item of uid: the UID itself if it's
// plain, else a hash of it (FAT takes few letters, and not long names).
func fileOf(uid string) string {
	ok := uid != "" && len(uid) <= 80
	for _, r := range uid {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("-_.@", r)) {
			ok = false
		}
	}
	if ok {
		return uid + ".ics"
	}
	return fmt.Sprintf("%x", sha1.Sum([]byte(uid)))[:24] + ".ics"
}

func (s *Store) writeEntry(e *Entry) error {
	s.ver++
	return writeFile(filepath.Join(s.Dir, e.In.ID, e.File), []byte(e.Item.Cal.String()))
}

func (s *Store) removeEntry(e *Entry) {
	s.ver++
	os.Remove(filepath.Join(s.Dir, e.In.ID, e.File))
	delete(e.In.Entries, e.File)
}

// Folders are the calendars (copies: their entries are not).
func (s *Store) Folders() []Folder {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Folder, len(s.cals))
	for i, f := range s.cals {
		out[i] = *f
		out[i].Entries = nil
	}
	return out
}

// Home is the calendar new items of kind go to: the first that holds it.
func (s *Store) Home(kind string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, f := range s.cals {
		if f.Holds(kind) && !f.Sub {
			return f.ID
		}
	}
	for _, f := range s.cals {
		if !f.Sub {
			return f.ID
		}
	}
	return ""
}

// Occurrence is an item at one of its times. Its Entry is a copy, as
// Tasks' are: what was there when asked. An item is changed through Save,
// a copy of it (its Cal may be shared: Save alone writes into it).
type Occurrence struct {
	*Entry
	At time.Time
}

// End is when this time of it ends.
func (o Occurrence) End() time.Time {
	if o.Item.Start.IsZero() || o.Item.End.IsZero() {
		return o.At
	}
	return o.At.Add(o.Item.End.Sub(o.Item.Start))
}

// Between is the items happening from from to to (tasks by their due,
// if not done), in order.
func (s *Store) Between(from, to time.Time) []Occurrence {
	s.mu.Lock()
	defer s.mu.Unlock()
	if b := &s.between; b.ver == s.ver && b.from.Equal(from) && b.to.Equal(to) && b.occ != nil {
		return b.occ
	}
	out := []Occurrence{}
	for _, f := range s.cals {
		for _, e := range f.Entries {
			if e.Gone || e.Item.Kind == Task && (e.Item.Done || e.Item.End.IsZero()) {
				continue
			}
			for _, t := range e.Item.Occurs(from, to) {
				cp := *e
				out = append(out, Occurrence{&cp, t})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if !a.At.Equal(b.At) {
			return a.At.Before(b.At)
		}
		if a.Item.AllDay != b.Item.AllDay {
			return a.Item.AllDay
		}
		return a.Item.Summary < b.Item.Summary
	})
	s.between.from, s.between.to, s.between.ver, s.between.occ = from, to, s.ver, out
	return out
}

// Tasks are the tasks: not done first, by due (none last), then done,
// last done first.
func (s *Store) Tasks() []*Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*Entry
	for _, f := range s.cals {
		for _, e := range f.Entries {
			if !e.Gone && e.Item.Kind == Task {
				cp := *e
				out = append(out, &cp)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].Item, out[j].Item
		if a.Done != b.Done {
			return !a.Done
		}
		if a.Done {
			return a.DoneAt.After(b.DoneAt)
		}
		if a.End.IsZero() != b.End.IsZero() {
			return !a.End.IsZero()
		}
		if !a.End.Equal(b.End) {
			return a.End.Before(b.End)
		}
		if a.Priority != b.Priority {
			return a.Priority != 0 && (b.Priority == 0 || a.Priority < b.Priority)
		}
		return a.Summary < b.Summary
	})
	return out
}

// Save writes the item (its fields applied, now) into the calendar id,
// to go to the server on the next Sync. An item it had is replaced; one
// moved from another calendar is taken from there.
func (s *Store) Save(id string, it *Item, now time.Time) (*Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f := s.folder(id)
	if f == nil {
		return nil, fmt.Errorf("no calendar %q", id)
	}
	if f.Sub {
		return nil, fmt.Errorf("%s is read only", f.Name)
	}
	it.Apply(now)
	var e *Entry
	for _, g := range s.cals {
		if g.Sub {
			continue
		}
		for _, x := range g.Entries {
			if x.Item.UID == it.UID && !x.Gone {
				e = x
			}
		}
	}
	if e != nil && e.In != f { // moved: gone from there, new here
		s.drop(e)
		e.When = now
		e = nil
	}
	if e == nil {
		e = &Entry{File: fileOf(it.UID), In: f}
		f.Entries[e.File] = e
	}
	e.Item = it
	e.Dirty++
	if err := s.writeEntry(e); err != nil {
		return nil, err
	}
	return e, s.saveState()
}

// Delete takes the item of uid away, now (from the server on the next
// Sync).
func (s *Store) Delete(uid string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, f := range s.cals {
		if f.Sub {
			continue
		}
		for _, e := range f.Entries {
			if e.Item.UID == uid && !e.Gone {
				s.drop(e)
				e.When = now
			}
		}
	}
	return s.saveState()
}

// drop deletes e here: at once if the server never had it, else when
// Sync has taken it from there too.
func (s *Store) drop(e *Entry) {
	s.ver++
	if e.Href == "" {
		s.removeEntry(e)
		return
	}
	e.Gone = true
	e.Dirty++
	// Another of the same UID may come in its place: its own file.
	delete(e.In.Entries, e.File)
	e.File = "gone-" + e.File
	e.In.Entries[e.File] = e
	os.Rename(filepath.Join(s.Dir, e.In.ID, strings.TrimPrefix(e.File, "gone-")), filepath.Join(s.Dir, e.In.ID, e.File))
}

func (s *Store) folder(id string) *Folder {
	for _, f := range s.cals {
		if f.ID == id {
			return f
		}
	}
	return nil
}

// Pending is how many changes here the server hasn't had yet.
func (s *Store) Pending() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, f := range s.cals {
		for _, e := range f.Entries {
			if e.Dirty > 0 {
				n++
			}
		}
	}
	return n
}

// Synced is what a Sync did.
type Synced struct {
	Got, Sent, Removed int
	Conflicts          int // changed on both sides: the later change kept
}

func (r Synced) String() string {
	var parts []string
	if r.Got > 0 {
		parts = append(parts, fmt.Sprintf("%d in", r.Got))
	}
	if r.Sent > 0 {
		parts = append(parts, fmt.Sprintf("%d out", r.Sent))
	}
	if r.Removed > 0 {
		parts = append(parts, fmt.Sprintf("%d gone", r.Removed))
	}
	if r.Conflicts > 0 {
		parts = append(parts, fmt.Sprintf("%d clashed", r.Conflicts))
	}
	if len(parts) == 0 {
		return "nothing new"
	}
	return strings.Join(parts, ", ")
}

// Sync brings the store and the server's calendars in step: ours sent
// (new, changed, deleted), theirs fetched where they changed. Changed
// on both sides, the later change wins. Calendars gone from the server
// go here too; a calendar of our own alone gives its items to the
// server's first that takes them.
func (s *Store) Sync(ctx context.Context, c *Client, now func() time.Time) (Synced, error) {
	var res Synced
	cals, err := c.Calendars(ctx)
	if err != nil {
		return res, err
	}
	if len(cals) == 0 {
		return res, errors.New("no calendars on the server")
	}
	s.mu.Lock()
	var keep []*Folder
	var orphans []*Entry
	for _, sc := range cals {
		f := s.byHref(sc.Href)
		if f == nil {
			f = &Folder{ID: s.newID(sc.Href), Href: sc.Href, Entries: map[string]*Entry{}}
		}
		f.Name, f.Comps = sc.Name, sc.Comps
		keep = append(keep, f)
	}
	var subs []*Folder
	for _, f := range s.cals {
		if f.Sub {
			subs = append(subs, f)
			continue
		}
		if f.Href != "" && !hasFolder(keep, f) {
			os.RemoveAll(filepath.Join(s.Dir, f.ID))
			continue
		}
		if f.Href == "" {
			for _, e := range f.Entries {
				orphans = append(orphans, e)
			}
			os.RemoveAll(filepath.Join(s.Dir, f.ID))
		}
	}
	s.cals = append(keep, subs...)
	for _, e := range orphans { // ours alone: to the server's
		if e.Gone {
			continue
		}
		f := s.cals[0]
		for _, g := range s.cals {
			if g.Holds(e.Item.Kind) && !g.Sub {
				f = g
				break
			}
		}
		e.In, e.Href, e.ETag = f, "", ""
		e.Dirty++
		f.Entries[e.File] = e
		s.writeEntry(e)
	}
	err = s.saveState()
	s.mu.Unlock()
	if err != nil {
		return res, err
	}
	for i, sc := range cals {
		if err := s.syncOne(ctx, c, keep[i], sc.CTag, now, &res); err != nil {
			return res, fmt.Errorf("%s: %w", sc.Name, err)
		}
	}
	return res, nil
}

func hasFolder(fs []*Folder, f *Folder) bool {
	for _, g := range fs {
		if g == f {
			return true
		}
	}
	return false
}

func (s *Store) byHref(href string) *Folder {
	for _, f := range s.cals {
		if !f.Sub && f.Href != "" && samePath(f.Href, href) {
			return f
		}
	}
	return nil
}

// newID is a folder name for the calendar at href: its last part, made
// plain, numbered if it's taken.
func (s *Store) newID(href string) string {
	base := fileOf(filepathBase(href))
	base = strings.TrimSuffix(base, ".ics")
	if len(base) > 24 {
		base = base[:24]
	}
	id := base
	for n := 2; s.folder(id) != nil || id == "local"; n++ {
		id = fmt.Sprintf("%s-%d", base, n)
	}
	return id
}

func filepathBase(href string) string {
	h := strings.TrimSuffix(href, "/")
	if i := strings.LastIndexByte(h, '/'); i >= 0 {
		h = h[i+1:]
	}
	if h == "" {
		h = "cal"
	}
	return h
}

// pending is a change to send: the entry, and its Dirty when we read it.
type pending struct {
	e     *Entry
	dirty int
	data  string
	href  string
	etag  string
	gone  bool
	mod   time.Time
}

func (s *Store) syncOne(ctx context.Context, c *Client, f *Folder, ctag string, now func() time.Time, res *Synced) error {
	s.mu.Lock()
	var send []pending
	for _, e := range f.Entries {
		if e.Dirty == 0 {
			continue
		}
		p := pending{e: e, dirty: e.Dirty, href: e.Href, etag: e.ETag, gone: e.Gone, data: e.Item.Cal.String(), mod: modified(e.Item)}
		if e.Gone {
			p.mod = e.When
		}
		if p.href == "" {
			p.href = Join(f.Href, fileOf(e.Item.UID))
		}
		send = append(send, p)
	}
	known := f.CTag
	s.mu.Unlock()

	for _, p := range send {
		var err error
		var etag string
		if p.gone {
			err = c.Delete(ctx, p.href, p.etag)
		} else {
			etag, err = c.Put(ctx, p.href, p.data, p.etag)
		}
		if Conflict(err) {
			res.Conflicts++
			// Changed there too: theirs, if it's the later.
			obs, ferr := c.Fetch(ctx, f.Href, []string{p.href})
			if ferr != nil {
				return ferr
			}
			var theirs *Item
			if len(obs) == 1 {
				if cc, perr := ParseString(obs[0].Data); perr == nil {
					theirs, _ = ItemOf(cc)
				}
			}
			if theirs != nil && modified(theirs).After(p.mod) {
				s.mu.Lock()
				if p.e.Dirty == p.dirty { // not changed again meanwhile: theirs over ours
					p.e.Dirty, p.e.ETag, p.e.Href = 0, "", p.href
					if p.gone { // back: theirs is fetched into it
						p.e.Gone = false
						delete(f.Entries, p.e.File)
						os.Remove(filepath.Join(s.Dir, f.ID, p.e.File))
						p.e.File = strings.TrimPrefix(p.e.File, "gone-")
						f.Entries[p.e.File] = p.e
					}
				}
				s.mu.Unlock()
				continue // fetched below, its ETag being new
			}
			// Ours is the later: over theirs.
			cur := ""
			if len(obs) == 1 {
				cur = obs[0].ETag
			}
			if p.gone {
				err = c.Delete(ctx, p.href, cur)
			} else {
				etag, err = c.Put(ctx, p.href, p.data, cur)
			}
		}
		if err != nil {
			return err
		}
		res.Sent++
		s.mu.Lock()
		if p.gone {
			s.removeEntry(p.e)
		} else {
			p.e.Href = p.href
			p.e.ETag = etag // "" if the server didn't say: the list below tells
			if p.e.Dirty == p.dirty {
				p.e.Dirty = 0
			}
		}
		s.mu.Unlock()
	}

	if len(send) == 0 && ctag != "" && ctag == known {
		return nil // nothing changed there, nor here
	}
	obs, err := c.List(ctx, f.Href)
	if err != nil {
		return err
	}
	there := map[string]string{}
	for _, o := range obs {
		there[canon(o.Href)] = o.ETag
	}
	var want []string
	s.mu.Lock()
	byHref := map[string]*Entry{}
	for _, e := range f.Entries {
		if e.Href == "" {
			continue
		}
		byHref[canon(e.Href)] = e
		if _, ok := there[canon(e.Href)]; !ok && e.Dirty == 0 { // gone from there
			s.removeEntry(e)
			res.Removed++
		}
	}
	for _, o := range obs {
		e := byHref[canon(o.Href)]
		if e == nil || e.Dirty == 0 && e.ETag != o.ETag {
			want = append(want, o.Href)
		}
	}
	s.mu.Unlock()

	for len(want) > 0 {
		n := min(len(want), 50)
		got, err := c.Fetch(ctx, f.Href, want[:n])
		if err != nil {
			return err
		}
		want = want[n:]
		s.mu.Lock()
		for _, o := range got {
			cc, err := ParseString(o.Data)
			if err != nil {
				continue
			}
			it, err := ItemOf(cc)
			if err != nil {
				continue
			}
			e := byHref[canon(o.Href)]
			if e != nil && e.Dirty > 0 {
				continue // changed here meanwhile: ours goes next time
			}
			if e == nil {
				e = &Entry{File: fileOf(it.UID), In: f, Href: o.Href}
				if old, ok := f.Entries[e.File]; ok && canon(old.Href) != canon(o.Href) {
					e.File = fmt.Sprintf("%x", sha1.Sum([]byte(o.Href)))[:24] + ".ics"
				}
				f.Entries[e.File] = e
			}
			e.Item, e.ETag = it, o.ETag
			if e.ETag == "" {
				e.ETag = there[canon(o.Href)]
			}
			s.writeEntry(e)
			res.Got++
		}
		s.mu.Unlock()
	}
	ctag, _ = c.CTag(ctx, f.Href)
	s.mu.Lock()
	f.CTag = ctag
	err = s.saveState()
	s.mu.Unlock()
	return err
}

// canon is an href to compare by: its path, escapes undone.
func canon(href string) string {
	h := href
	if i := strings.Index(h, "://"); i >= 0 {
		if j := strings.IndexByte(h[i+3:], '/'); j >= 0 {
			h = h[i+3+j:]
		}
	}
	if u, err := url.PathUnescape(h); err == nil {
		h = u
	}
	return h
}

// modified is when the item was last changed, as it says.
func modified(it *Item) time.Time {
	c := it.comp()
	if c == nil {
		return time.Time{}
	}
	for _, name := range []string{"LAST-MODIFIED", "DTSTAMP", "CREATED"} {
		if p := c.Get(name); p != nil {
			if t, _ := parseTime(p); !t.IsZero() {
				return t
			}
		}
	}
	return time.Time{}
}
