package cal

// Calendars subscribed to: an .ics file at a URL (Google Calendar's
// secret address, a team's or a country's holidays), fetched whole and
// shown as it is, read only.

import (
	"context"
	"crypto/sha1"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const feedFile = "feed.ics"

// subID is the folder of the subscription to u.
func subID(u string) string { return fmt.Sprintf("sub-%x", sha1.Sum([]byte(u)))[:14] }

// fill makes f's entries the items in c, a whole calendar: one each UID
// (with its times moved, and the time zones).
func (f *Folder) fill(c *Comp) {
	f.Entries = map[string]*Entry{}
	var zones []*Comp
	var order []string
	by := map[string][]*Comp{}
	for _, s := range c.Comps {
		switch s.Name {
		case "VTIMEZONE":
			zones = append(zones, s)
		case Event, Task:
			uid := s.Text("UID")
			if uid == "" {
				uid = fmt.Sprintf("no-uid-%d", len(order))
			}
			if _, ok := by[uid]; !ok {
				order = append(order, uid)
			}
			by[uid] = append(by[uid], s)
		}
	}
	var head []*Prop
	for _, p := range c.Props {
		if p.Name == "VERSION" || p.Name == "PRODID" {
			head = append(head, p)
		}
	}
	for _, uid := range order {
		one := &Comp{Name: "VCALENDAR", Props: head, Comps: append(append([]*Comp{}, zones...), by[uid]...)}
		it, err := ItemOf(one)
		if err != nil {
			continue // moved times of something not in it
		}
		name := fileOf(uid)
		f.Entries[name] = &Entry{Item: it, File: name, In: f, Href: uid}
	}
}

// SyncSubs fetches the calendars subscribed to (webcal:// is https://),
// each where it changed; those no longer subscribed to go.
func (s *Store) SyncSubs(ctx context.Context, hc *http.Client, agent string, urls []string) (Synced, error) {
	var res Synced
	want := map[string]bool{}
	var firstErr error
	for _, u := range urls {
		u = strings.TrimSpace(u)
		if strings.HasPrefix(u, "webcal://") {
			u = "https://" + strings.TrimPrefix(u, "webcal://")
		}
		want[subID(u)] = true
		n, err := s.syncSub(ctx, hc, agent, u)
		res.Got += n
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var keep []*Folder
	for _, f := range s.cals {
		if f.Sub && !want[f.ID] {
			os.RemoveAll(filepath.Join(s.Dir, f.ID))
			res.Removed += len(f.Entries)
			continue
		}
		keep = append(keep, f)
	}
	if len(keep) != len(s.cals) {
		s.cals = keep
		if err := s.saveState(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return res, firstErr
}

// syncSub fetches the one at u: how many items it has, if it changed.
func (s *Store) syncSub(ctx context.Context, hc *http.Client, agent, u string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return 0, err
	}
	if agent != "" {
		req.Header.Set("User-Agent", agent)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return 0, err
	}
	pu, _ := url.Parse(u)
	if resp.StatusCode != http.StatusOK {
		return 0, &StatusError{"GET", pu.Host, resp.StatusCode, ""}
	}
	c, err := ParseString(string(b))
	if err != nil {
		return 0, fmt.Errorf("%s: %w", pu.Host, err)
	}
	hash := fmt.Sprintf("%x", sha1.Sum(b))
	id := subID(u)
	s.mu.Lock()
	defer s.mu.Unlock()
	f := s.folder(id)
	if f != nil && f.Hash == hash {
		return 0, nil
	}
	if f == nil {
		f = &Folder{ID: id, Href: u, Sub: true, Comps: []string{Event, Task}}
		s.cals = append(s.cals, f)
	}
	f.Name = c.Text("X-WR-CALNAME")
	if f.Name == "" {
		f.Name = pu.Host
	}
	if err := writeFile(filepath.Join(s.Dir, id, feedFile), b); err != nil {
		return 0, err
	}
	f.fill(c)
	f.Hash = hash
	return len(f.Entries), s.saveState()
}
