// Package davtest is a small CalDAV server in memory, for tests: a
// principal at /principals/USER/, its home at /cal/USER/, calendars in
// it. It answers what cal's client asks (PROPFIND, calendar-multiget,
// GET, PUT and DELETE with ETags), and no more.
package davtest

import (
	"crypto/sha1"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
)

// Server is the calendars of one user.
type Server struct {
	User, Password string

	mu   sync.Mutex
	cals map[string]*Cal // by href
	n    int
}

// Cal is a calendar on the server.
type Cal struct {
	Name  string
	Comps []string
	Objs  map[string]string // object's href: its data
	ctag  int
}

// New is a server for user, with calendars named names (an event's and
// a task's each).
func New(user, password string, names ...string) *Server {
	s := &Server{User: user, Password: password, cals: map[string]*Cal{}}
	for _, n := range names {
		s.AddCal(n, nil)
	}
	return s
}

func (s *Server) home() string { return "/cal/" + s.User + "/" }

// AddCal makes a calendar called name holding comps (nil: both); it
// gives its href.
func (s *Server) AddCal(name string, comps []string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	href := s.home() + strings.ToLower(strings.ReplaceAll(name, " ", "-")) + "/"
	s.cals[href] = &Cal{Name: name, Comps: comps, Objs: map[string]string{}}
	return href
}

// Put puts an object in the calendar at cal as if another client had.
func (s *Server) Put(cal, name, data string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.cals[cal]
	c.Objs[cal+name] = data
	c.ctag++
}

// Remove takes an object away as if another client had.
func (s *Server) Remove(href string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.cals {
		if _, ok := c.Objs[href]; ok {
			delete(c.Objs, href)
			c.ctag++
		}
	}
}

// Objects are the calendar's objects: href, data.
func (s *Server) Objects(cal string) map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string]string{}
	for k, v := range s.cals[cal].Objs {
		out[k] = v
	}
	return out
}

// Requests is how many requests it has had.
func (s *Server) Requests() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.n
}

func etag(data string) string { return fmt.Sprintf(`"%x"`, sha1.Sum([]byte(data)))[:18] + `"` }

func (s *Server) find(href string) (*Cal, string) {
	for h, c := range s.cals {
		if href == h || href+"/" == h {
			return c, h
		}
	}
	return nil, ""
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.n++
	if r.URL.Path == "/.well-known/caldav" {
		http.Redirect(w, r, "/", http.StatusMovedPermanently)
		return
	}
	if u, p, ok := r.BasicAuth(); !ok || u != s.User || p != s.Password {
		w.Header().Set("WWW-Authenticate", `Basic realm="cal"`)
		http.Error(w, "who are you", http.StatusUnauthorized)
		return
	}
	body, _ := io.ReadAll(r.Body)
	path := r.URL.Path
	switch r.Method {
	case "PROPFIND":
		s.propfind(w, path, r.Header.Get("Depth"), string(body))
	case "REPORT":
		s.report(w, path, string(body))
	case "GET":
		for _, c := range s.cals {
			if d, ok := c.Objs[path]; ok {
				w.Header().Set("ETag", etag(d))
				w.Header().Set("Content-Type", "text/calendar")
				io.WriteString(w, d)
				return
			}
		}
		http.NotFound(w, r)
	case "PUT":
		dir := path[:strings.LastIndexByte(path, '/')+1]
		c, _ := s.find(dir)
		if c == nil {
			http.Error(w, "no calendar", http.StatusConflict)
			return
		}
		old, had := c.Objs[path]
		if m := r.Header.Get("If-Match"); m != "" && (!had || m != etag(old)) {
			http.Error(w, "changed", http.StatusPreconditionFailed)
			return
		}
		if r.Header.Get("If-None-Match") == "*" && had {
			http.Error(w, "there", http.StatusPreconditionFailed)
			return
		}
		c.Objs[path] = string(body)
		c.ctag++
		w.Header().Set("ETag", etag(string(body)))
		if had {
			w.WriteHeader(http.StatusNoContent)
		} else {
			w.WriteHeader(http.StatusCreated)
		}
	case "DELETE":
		for _, c := range s.cals {
			if d, ok := c.Objs[path]; ok {
				if m := r.Header.Get("If-Match"); m != "" && m != etag(d) {
					http.Error(w, "changed", http.StatusPreconditionFailed)
					return
				}
				delete(c.Objs, path)
				c.ctag++
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		http.NotFound(w, r)
	default:
		http.Error(w, "no", http.StatusMethodNotAllowed)
	}
}

type resp struct {
	href  string
	props []string // XML of each property found
}

func (s *Server) write(w http.ResponseWriter, rs []resp) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(207)
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?><d:multistatus xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav" xmlns:cs="http://calendarserver.org/ns/">`)
	for _, r := range rs {
		b.WriteString("<d:response><d:href>" + html.EscapeString(r.href) + "</d:href><d:propstat><d:prop>")
		b.WriteString(strings.Join(r.props, ""))
		b.WriteString("</d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>")
	}
	b.WriteString("</d:multistatus>")
	io.WriteString(w, b.String())
}

func (s *Server) calProps(href string, c *Cal, asked string) []string {
	var ps []string
	if strings.Contains(asked, "resourcetype") {
		ps = append(ps, `<d:resourcetype><d:collection/><c:calendar/></d:resourcetype>`)
	}
	if strings.Contains(asked, "displayname") {
		ps = append(ps, "<d:displayname>"+html.EscapeString(c.Name)+"</d:displayname>")
	}
	if strings.Contains(asked, "getctag") {
		ps = append(ps, fmt.Sprintf("<cs:getctag>%d</cs:getctag>", c.ctag))
	}
	if strings.Contains(asked, "supported-calendar-component-set") {
		comps := c.Comps
		if comps == nil {
			comps = []string{"VEVENT", "VTODO"}
		}
		var b strings.Builder
		b.WriteString("<c:supported-calendar-component-set>")
		for _, k := range comps {
			b.WriteString(`<c:comp name="` + k + `"/>`)
		}
		b.WriteString("</c:supported-calendar-component-set>")
		ps = append(ps, b.String())
	}
	return ps
}

func (s *Server) propfind(w http.ResponseWriter, path, depth, asked string) {
	principal := "/principals/" + s.User + "/"
	var rs []resp
	switch {
	case path == "/" || path == "" || path == principal:
		var ps []string
		if strings.Contains(asked, "current-user-principal") {
			ps = append(ps, "<d:current-user-principal><d:href>"+principal+"</d:href></d:current-user-principal>")
		}
		if path == principal && strings.Contains(asked, "calendar-home-set") {
			ps = append(ps, "<c:calendar-home-set><d:href>"+s.home()+"</d:href></c:calendar-home-set>")
		}
		if strings.Contains(asked, "resourcetype") {
			ps = append(ps, "<d:resourcetype><d:collection/></d:resourcetype>")
		}
		rs = append(rs, resp{path, ps})
	case path == s.home() || path+"/" == s.home():
		rs = append(rs, resp{s.home(), []string{"<d:resourcetype><d:collection/></d:resourcetype>"}})
		if depth == "1" {
			var hrefs []string
			for h := range s.cals {
				hrefs = append(hrefs, h)
			}
			sort.Strings(hrefs)
			for _, h := range hrefs {
				rs = append(rs, resp{h, s.calProps(h, s.cals[h], asked)})
			}
		}
	default:
		c, h := s.find(path)
		if c == nil {
			http.NotFound(w, nil)
			return
		}
		rs = append(rs, resp{h, s.calProps(h, c, asked)})
		if depth == "1" {
			var hrefs []string
			for o := range c.Objs {
				hrefs = append(hrefs, o)
			}
			sort.Strings(hrefs)
			for _, o := range hrefs {
				rs = append(rs, resp{o, []string{"<d:getetag>" + html.EscapeString(etag(c.Objs[o])) + "</d:getetag>", "<d:resourcetype/>"}})
			}
		}
	}
	s.write(w, rs)
}

func (s *Server) report(w http.ResponseWriter, path, body string) {
	c, _ := s.find(path)
	if c == nil || !strings.Contains(body, "calendar-multiget") {
		http.Error(w, "no", http.StatusBadRequest)
		return
	}
	var q struct {
		Hrefs []string `xml:"DAV: href"`
	}
	if err := xml.Unmarshal([]byte(body), &q); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var rs []resp
	for _, h := range q.Hrefs {
		if d, ok := c.Objs[h]; ok {
			rs = append(rs, resp{h, []string{"<d:getetag>" + html.EscapeString(etag(d)) + "</d:getetag>", "<c:calendar-data>" + html.EscapeString(d) + "</c:calendar-data>"}})
		}
	}
	s.write(w, rs)
}
