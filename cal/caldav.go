package cal

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// Client talks to a CalDAV server: its calendars found from URL (a
// calendar, a calendar home, or the server's root, through the user's
// principal), their objects listed by ETag, fetched, put and deleted.
type Client struct {
	URL       string
	User      string
	Password  string
	UserAgent string
	HTTP      *http.Client
}

// Calendar is a calendar collection on the server.
type Calendar struct {
	Href  string   // its path, as the server gives it
	Name  string   // its display name
	Comps []string // what it holds: VEVENT, VTODO; none said: both
	CTag  string   // changes when anything in it does, if the server says
}

// Holds says the calendar takes items of kind.
func (c Calendar) Holds(kind string) bool {
	if len(c.Comps) == 0 {
		return true
	}
	for _, k := range c.Comps {
		if k == kind {
			return true
		}
	}
	return false
}

// Object is a calendar object's place and version, and maybe its data.
type Object struct {
	Href string
	ETag string
	Data string
}

// StatusError is a request the server refused.
type StatusError struct {
	Method, Href string
	Code         int
	Body         string
}

func (e *StatusError) Error() string {
	s := fmt.Sprintf("%s %s: %d %s", e.Method, e.Href, e.Code, http.StatusText(e.Code))
	if b := strings.TrimSpace(e.Body); b != "" && len(b) < 200 {
		s += ": " + b
	}
	return s
}

// Conflict says the object changed on the server since it was read (a
// put or a delete with its old ETag).
func Conflict(err error) bool {
	var se *StatusError
	return errors.As(err, &se) && se.Code == http.StatusPreconditionFailed
}

func (c *Client) http() *http.Client {
	if c.HTTP == nil {
		c.HTTP = &http.Client{Timeout: 60 * time.Second}
	}
	// Redirects are ours to follow: Go's would make a PROPFIND a GET.
	h := *c.HTTP
	h.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &h
}

// abs is href (a path, or a URL) made a URL on the server.
func (c *Client) abs(href string) (string, error) {
	base, err := url.Parse(c.URL)
	if err != nil {
		return "", err
	}
	r, err := url.Parse(href)
	if err != nil {
		return "", err
	}
	return base.ResolveReference(r).String(), nil
}

func (c *Client) do(ctx context.Context, method, href string, body string, hdr map[string]string) (*http.Response, []byte, error) {
	u, err := c.abs(href)
	if err != nil {
		return nil, nil, err
	}
	for range 5 {
		resp, b, err := c.do1(ctx, method, u, body, hdr)
		switch {
		case resp == nil:
			return nil, nil, err
		case resp.StatusCode == http.StatusMovedPermanently, resp.StatusCode == http.StatusFound,
			resp.StatusCode == http.StatusTemporaryRedirect, resp.StatusCode == http.StatusPermanentRedirect:
			loc, lerr := resp.Location()
			if lerr != nil {
				return resp, b, &StatusError{method, href, resp.StatusCode, string(b)}
			}
			if base, _ := url.Parse(c.URL); base != nil && (loc.Host != base.Host || loc.Scheme != base.Scheme) {
				// Another server: the rest goes there (iCloud's, say).
				nb := *base
				nb.Scheme, nb.Host = loc.Scheme, loc.Host
				c.URL = nb.String()
			}
			u = loc.String()
			continue
		case resp.StatusCode >= 300:
			return resp, b, &StatusError{method, href, resp.StatusCode, string(b)}
		}
		return resp, b, nil
	}
	return nil, nil, fmt.Errorf("%s %s: too many redirects", method, href)
}

func (c *Client) do1(ctx context.Context, method, u string, body string, hdr map[string]string) (*http.Response, []byte, error) {
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rd)
	if err != nil {
		return nil, nil, err
	}
	if c.User != "" || c.Password != "" {
		req.SetBasicAuth(c.User, c.Password)
	}
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := c.http().Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, nil, err
	}
	return resp, b, nil
}

// The XML of a multistatus answer.
type multistatus struct {
	Responses []davResponse `xml:"DAV: response"`
}

type davResponse struct {
	Href     string     `xml:"DAV: href"`
	Propstat []propstat `xml:"DAV: propstat"`
}

type propstat struct {
	Status string `xml:"DAV: status"`
	Prop   struct {
		ResourceType struct {
			Collection *struct{} `xml:"DAV: collection"`
			Calendar   *struct{} `xml:"urn:ietf:params:xml:ns:caldav calendar"`
		} `xml:"DAV: resourcetype"`
		DisplayName string `xml:"DAV: displayname"`
		ETag        string `xml:"DAV: getetag"`
		CTag        string `xml:"http://calendarserver.org/ns/ getctag"`
		SyncToken   string `xml:"DAV: sync-token"`
		Principal   string `xml:"DAV: current-user-principal>href"`
		Home        struct {
			Href string `xml:"DAV: href"`
		} `xml:"urn:ietf:params:xml:ns:caldav calendar-home-set"`
		Comps []struct {
			Name string `xml:"name,attr"`
		} `xml:"urn:ietf:params:xml:ns:caldav supported-calendar-component-set>comp"`
		Data string `xml:"urn:ietf:params:xml:ns:caldav calendar-data"`
	} `xml:"DAV: prop"`
}

// ok says the propstat is the one with what was found.
func (p propstat) ok() bool { return p.Status == "" || strings.Contains(p.Status, " 200") }

func (c *Client) propfind(ctx context.Context, href, depth, props string) (*multistatus, error) {
	body := `<?xml version="1.0" encoding="utf-8"?><d:propfind xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav" xmlns:cs="http://calendarserver.org/ns/"><d:prop>` + props + `</d:prop></d:propfind>`
	_, b, err := c.do(ctx, "PROPFIND", href, body, map[string]string{"Depth": depth, "Content-Type": "application/xml; charset=utf-8"})
	if err != nil {
		return nil, err
	}
	var ms multistatus
	if err := xml.Unmarshal(b, &ms); err != nil {
		return nil, fmt.Errorf("PROPFIND %s: %w", href, err)
	}
	return &ms, nil
}

const calProps = `<d:resourcetype/><d:displayname/><cs:getctag/><c:supported-calendar-component-set/>`

// Calendars finds the calendars: URL itself if it's one; else those in
// it (a calendar home); else those in the home of the principal it
// leads to.
func (c *Client) Calendars(ctx context.Context) ([]Calendar, error) {
	u, err := url.Parse(c.URL)
	if err != nil {
		return nil, err
	}
	start := u.Path
	if start == "" {
		start = "/"
	}
	if cs, err := c.calendarsIn(ctx, start); err != nil || len(cs) > 0 {
		return cs, err
	}
	ms, err := c.propfind(ctx, start, "0", `<d:current-user-principal/><c:calendar-home-set/>`)
	if err != nil {
		return nil, err
	}
	var principal, home string
	for _, r := range ms.Responses {
		for _, ps := range r.Propstat {
			if ps.ok() {
				principal = cmp(principal, ps.Prop.Principal)
				home = cmp(home, ps.Prop.Home.Href)
			}
		}
	}
	if home == "" && principal != "" {
		ms, err := c.propfind(ctx, principal, "0", `<c:calendar-home-set/>`)
		if err != nil {
			return nil, err
		}
		for _, r := range ms.Responses {
			for _, ps := range r.Propstat {
				if ps.ok() {
					home = cmp(home, ps.Prop.Home.Href)
				}
			}
		}
	}
	if home == "" {
		return nil, fmt.Errorf("no calendars at %s, nor a calendar home", c.URL)
	}
	return c.calendarsIn(ctx, home)
}

func cmp(a, b string) string {
	if a != "" {
		return a
	}
	return strings.TrimSpace(b)
}

// calendarsIn is href if it's a calendar, else the calendars in it.
func (c *Client) calendarsIn(ctx context.Context, href string) ([]Calendar, error) {
	ms, err := c.propfind(ctx, href, "1", calProps)
	if err != nil {
		return nil, err
	}
	var cs []Calendar
	for _, r := range ms.Responses {
		for _, ps := range r.Propstat {
			if !ps.ok() || ps.Prop.ResourceType.Calendar == nil {
				continue
			}
			cal := Calendar{Href: r.Href, Name: strings.TrimSpace(ps.Prop.DisplayName), CTag: ps.Prop.CTag}
			for _, k := range ps.Prop.Comps {
				cal.Comps = append(cal.Comps, strings.ToUpper(k.Name))
			}
			if cal.Name == "" {
				cal.Name = path.Base(strings.TrimSuffix(r.Href, "/"))
			}
			if samePath(r.Href, href) { // href is one itself: that's all
				return []Calendar{cal}, nil
			}
			cs = append(cs, cal)
		}
	}
	return cs, nil
}

func samePath(a, b string) bool {
	norm := func(s string) string {
		if u, err := url.Parse(s); err == nil {
			s = u.Path
		}
		return strings.TrimSuffix(s, "/")
	}
	return norm(a) == norm(b)
}

// CTag is the calendar's CTag now ("" if the server has none).
func (c *Client) CTag(ctx context.Context, cal string) (string, error) {
	ms, err := c.propfind(ctx, cal, "0", `<cs:getctag/>`)
	if err != nil {
		return "", err
	}
	for _, r := range ms.Responses {
		for _, ps := range r.Propstat {
			if ps.ok() && ps.Prop.CTag != "" {
				return ps.Prop.CTag, nil
			}
		}
	}
	return "", nil
}

// List is the objects in the calendar, by their hrefs: their ETags.
func (c *Client) List(ctx context.Context, cal string) ([]Object, error) {
	ms, err := c.propfind(ctx, cal, "1", `<d:getetag/><d:resourcetype/>`)
	if err != nil {
		return nil, err
	}
	var obs []Object
	for _, r := range ms.Responses {
		if samePath(r.Href, cal) {
			continue
		}
		for _, ps := range r.Propstat {
			if ps.ok() && ps.Prop.ResourceType.Collection == nil && ps.Prop.ETag != "" {
				obs = append(obs, Object{Href: r.Href, ETag: ps.Prop.ETag})
			}
		}
	}
	return obs, nil
}

// Fetch gets the objects at hrefs in the calendar, in one REPORT
// (calendar-multiget); one GET each if the server won't.
func (c *Client) Fetch(ctx context.Context, cal string, hrefs []string) ([]Object, error) {
	if len(hrefs) == 0 {
		return nil, nil
	}
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?><c:calendar-multiget xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav"><d:prop><d:getetag/><c:calendar-data/></d:prop>`)
	for _, h := range hrefs {
		b.WriteString("<d:href>")
		xml.EscapeText(&b, []byte(h))
		b.WriteString("</d:href>")
	}
	b.WriteString(`</c:calendar-multiget>`)
	_, body, err := c.do(ctx, "REPORT", cal, b.String(), map[string]string{"Depth": "1", "Content-Type": "application/xml; charset=utf-8"})
	if err == nil {
		var ms multistatus
		if err := xml.Unmarshal(body, &ms); err == nil {
			var obs []Object
			for _, r := range ms.Responses {
				for _, ps := range r.Propstat {
					if ps.ok() && ps.Prop.Data != "" {
						obs = append(obs, Object{Href: r.Href, ETag: ps.Prop.ETag, Data: ps.Prop.Data})
					}
				}
			}
			if len(obs) > 0 {
				return obs, nil
			}
		}
	}
	var obs []Object
	for _, h := range hrefs {
		resp, data, err := c.do(ctx, "GET", h, "", nil)
		if err != nil {
			return obs, err
		}
		obs = append(obs, Object{Href: h, ETag: resp.Header.Get("ETag"), Data: string(data)})
	}
	return obs, nil
}

// Put writes data at href: over the version etag ("": a new one, not
// over anything). It gives the new ETag, if the server says.
func (c *Client) Put(ctx context.Context, href, data, etag string) (string, error) {
	hdr := map[string]string{"Content-Type": "text/calendar; charset=utf-8"}
	if etag != "" {
		hdr["If-Match"] = etag
	} else {
		hdr["If-None-Match"] = "*"
	}
	resp, _, err := c.do(ctx, "PUT", href, data, hdr)
	if err != nil {
		return "", err
	}
	return resp.Header.Get("ETag"), nil
}

// Delete takes the object at href away, if it's still the version etag
// ("": whatever it is).
func (c *Client) Delete(ctx context.Context, href, etag string) error {
	hdr := map[string]string{}
	if etag != "" {
		hdr["If-Match"] = etag
	}
	_, _, err := c.do(ctx, "DELETE", href, "", hdr)
	var se *StatusError
	if errors.As(err, &se) && se.Code == http.StatusNotFound {
		return nil // gone already
	}
	return err
}

// Join is the href of name in the calendar at cal.
func Join(cal, name string) string {
	if !strings.HasSuffix(cal, "/") {
		cal += "/"
	}
	return cal + url.PathEscape(name)
}
