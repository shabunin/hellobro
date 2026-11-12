package cal_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"hellobro/cal"
	"hellobro/cal/davtest"
)

func event(uid, summary, start string) string {
	return "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:" + uid + "\r\nDTSTAMP:20261101T000000Z\r\nLAST-MODIFIED:20261101T000000Z\r\n" +
		"DTSTART:" + start + "\r\nDURATION:PT1H\r\nSUMMARY:" + summary + "\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
}

type clock struct{ t time.Time }

func (c *clock) now() time.Time { c.t = c.t.Add(time.Minute); return c.t }

func setup(t *testing.T) (*davtest.Server, *cal.Client, *cal.Store, *clock) {
	srv := davtest.New("me", "pw", "Home", "Work")
	hs := httptest.NewServer(srv)
	t.Cleanup(hs.Close)
	st, err := cal.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return srv, &cal.Client{URL: hs.URL + "/", User: "me", Password: "pw"}, st, &clock{time.Date(2026, 11, 12, 8, 0, 0, 0, time.UTC)}
}

func sync(t *testing.T, st *cal.Store, c *cal.Client, ck *clock) cal.Synced {
	t.Helper()
	r, err := st.Sync(context.Background(), c, ck.now)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func summaries(st *cal.Store) string {
	var s []string
	for _, o := range st.Between(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)) {
		s = append(s, o.Item.Summary)
	}
	for _, e := range st.Tasks() {
		s = append(s, "task "+e.Item.Summary)
	}
	return strings.Join(s, ", ")
}

func TestDiscover(t *testing.T) {
	srv, c, _, _ := setup(t)
	srv.AddCal("Todo", []string{"VTODO"})
	cals, err := c.Calendars(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, k := range cals {
		names = append(names, k.Name)
	}
	if strings.Join(names, " ") != "Home Todo Work" {
		t.Errorf("calendars %v", names)
	}
	if cals[1].Holds(cal.Event) || !cals[1].Holds(cal.Task) || !cals[0].Holds(cal.Event) {
		t.Errorf("comps %+v", cals)
	}
	// A calendar's own URL: that one alone.
	one := *c
	one.URL += "cal/me/work/"
	cals, err = one.Calendars(context.Background())
	if err != nil || len(cals) != 1 || cals[0].Name != "Work" {
		t.Errorf("one: %+v %v", cals, err)
	}
	wk := *c
	wk.URL += ".well-known/caldav"
	if cals, err = wk.Calendars(context.Background()); err != nil || len(cals) != 3 {
		t.Errorf("through .well-known: %+v %v", cals, err)
	}
	bad := *c
	bad.Password = "no"
	if _, err := bad.Calendars(context.Background()); err == nil || !strings.Contains(err.Error(), "401") {
		t.Errorf("bad password: %v", err)
	}
}

func TestSync(t *testing.T) {
	srv, c, st, ck := setup(t)
	srv.Put("/cal/me/home/", "a.ics", event("a", "dentist", "20261113T090000Z"))
	srv.Put("/cal/me/work/", "b.ics", event("b", "review", "20261114T090000Z"))

	// Ours first, before any server: it goes to the server's first.
	task := cal.New(cal.Task)
	task.Summary = "buy milk"
	if _, err := st.Save(st.Home(cal.Task), task, ck.now()); err != nil {
		t.Fatal(err)
	}
	r := sync(t, st, c, ck)
	if r.Got != 2 || r.Sent != 1 {
		t.Errorf("first: %v", r)
	}
	if got := summaries(st); got != "dentist, review, task buy milk" {
		t.Errorf("first: %s", got)
	}
	if objs := srv.Objects("/cal/me/home/"); len(objs) != 2 {
		t.Errorf("home on the server: %v", objs)
	}

	// Nothing changed: the ctags tell, nothing fetched.
	n := srv.Requests()
	if r := sync(t, st, c, ck); r != (cal.Synced{}) {
		t.Errorf("again: %v", r)
	}
	if d := srv.Requests() - n; d > 6 {
		t.Errorf("%d requests for nothing", d)
	}

	// Changed there: fetched; removed there: gone here.
	srv.Put("/cal/me/home/", "a.ics", event("a", "dentist, moved", "20261113T100000Z"))
	srv.Remove("/cal/me/work/b.ics")
	r = sync(t, st, c, ck)
	if r.Got != 1 || r.Removed != 1 {
		t.Errorf("theirs: %v", r)
	}
	if got := summaries(st); got != "dentist, moved, task buy milk" {
		t.Errorf("theirs: %s", got)
	}

	// Changed here: sent; deleted here: gone there.
	tasks := st.Tasks()
	done := *tasks[0].Item
	done.Done = true
	st.Save(tasks[0].In.ID, &done, ck.now())
	ev := st.Between(time.Time{}, time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC))[0]
	st.Delete(ev.Item.UID, ck.now())
	if st.Pending() != 2 {
		t.Errorf("pending %d", st.Pending())
	}
	r = sync(t, st, c, ck)
	if r.Sent != 2 || st.Pending() != 0 {
		t.Errorf("ours: %v, %d pending", r, st.Pending())
	}
	objs := srv.Objects("/cal/me/home/")
	if len(objs) != 1 {
		t.Fatalf("home on the server: %v", objs)
	}
	for _, d := range objs {
		if !strings.Contains(d, "STATUS:COMPLETED") {
			t.Errorf("not done there:\n%s", d)
		}
	}

	// Again from what's on disk: the same.
	st2, err := cal.Open(st.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := summaries(st2); got != "task buy milk" {
		t.Errorf("reopened: %s", got)
	}
	if r := sync(t, st2, c, ck); r != (cal.Synced{}) {
		t.Errorf("reopened, synced: %v", r)
	}
}

func TestConflict(t *testing.T) {
	srv, c, st, ck := setup(t)
	srv.Put("/cal/me/home/", "a.ics", event("a", "dentist", "20261113T090000Z"))
	sync(t, st, c, ck)

	// Both changed; ours the later: ours kept.
	srv.Put("/cal/me/home/", "a.ics", event("a", "theirs", "20261113T090000Z")) // last modified 1 Nov
	ev := st.Between(time.Time{}, time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC))[0]
	mine := *ev.Item
	mine.Summary = "ours"
	st.Save(ev.In.ID, &mine, ck.now())
	r := sync(t, st, c, ck)
	if r.Conflicts != 1 || summaries(st) != "ours" || !strings.Contains(srv.Objects("/cal/me/home/")["/cal/me/home/a.ics"], "SUMMARY:ours") {
		t.Errorf("ours later: %v, %s", r, summaries(st))
	}

	// Theirs the later: theirs kept.
	later := strings.Replace(event("a", "theirs again", "20261113T090000Z"), "LAST-MODIFIED:20261101", "LAST-MODIFIED:20271101", 1)
	srv.Put("/cal/me/home/", "a.ics", later)
	ev = st.Between(time.Time{}, time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC))[0]
	mine = *ev.Item
	mine.Summary = "ours again"
	st.Save(ev.In.ID, &mine, ck.now())
	r = sync(t, st, c, ck)
	if r.Conflicts != 1 || summaries(st) != "theirs again" {
		t.Errorf("theirs later: %v, %s", r, summaries(st))
	}

	// Deleted here, changed there later: it's back.
	ev = st.Between(time.Time{}, time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC))[0]
	st.Delete(ev.Item.UID, ck.now())
	srv.Put("/cal/me/home/", "a.ics", strings.NewReplacer("theirs again", "theirs once more", "20271101", "20281101").Replace(later))
	sync(t, st, c, ck)
	if got := summaries(st); got != "theirs once more" {
		t.Errorf("deleted here: %s", got)
	}
}

func TestSubscribe(t *testing.T) {
	feed := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nX-WR-CALNAME:Holidays\r\n" +
		"BEGIN:VEVENT\r\nUID:x\r\nDTSTART;VALUE=DATE:20261225\r\nSUMMARY:Christmas\r\nRRULE:FREQ=YEARLY\r\nEND:VEVENT\r\n" +
		"BEGIN:VEVENT\r\nUID:y\r\nDTSTART:20261113T100000Z\r\nDTEND:20261113T110000Z\r\nSUMMARY:talk\r\nEND:VEVENT\r\n" +
		"BEGIN:VEVENT\r\nUID:y\r\nRECURRENCE-ID:20261113T100000Z\r\nDTSTART:20261113T120000Z\r\nSUMMARY:talk, moved\r\nEND:VEVENT\r\n" +
		"END:VCALENDAR\r\n"
	n := 0
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		io.WriteString(w, feed)
	}))
	defer hs.Close()
	st, _ := cal.Open(t.TempDir())
	urls := []string{hs.URL + "/basic.ics"}
	r, err := st.SyncSubs(context.Background(), hs.Client(), "", urls)
	if err != nil || r.Got != 2 {
		t.Fatalf("%v %v", r, err)
	}
	if got := summaries(st); got != "talk, Christmas" {
		t.Errorf("got %s", got)
	}
	var sub cal.Folder
	for _, f := range st.Folders() {
		if f.ReadOnly() {
			sub = f
		}
	}
	if sub.Name != "Holidays" {
		t.Errorf("folders %+v", st.Folders())
	}
	it := cal.New(cal.Event)
	if _, err := st.Save(sub.ID, it, time.Now()); err == nil {
		t.Error("saved into a subscription")
	}
	if home := st.Home(cal.Event); home == sub.ID {
		t.Error("new ones into a subscription")
	}
	// The same again: nothing new. Then reopened: still there.
	if r, _ := st.SyncSubs(context.Background(), hs.Client(), "", urls); r.Got != 0 {
		t.Errorf("again: %v", r)
	}
	st2, _ := cal.Open(st.Dir)
	if got := summaries(st2); got != "talk, Christmas" {
		t.Errorf("reopened: %s", got)
	}
	// Unsubscribed: gone.
	if r, _ := st2.SyncSubs(context.Background(), hs.Client(), "", nil); r.Removed != 2 || summaries(st2) != "" {
		t.Errorf("unsubscribed: %v %s", r, summaries(st2))
	}
}

// TestHidden: a hidden calendar's items aren't shown, and new ones go to
// another; it stays hidden through syncs and reopening.
func TestHidden(t *testing.T) {
	_, c, st, ck := setup(t)
	sync(t, st, c, ck)
	home := st.Home(cal.Event)
	it := cal.New(cal.Event)
	it.Summary, it.Start = "lunch", time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	it.End = it.Start.Add(time.Hour)
	if _, err := st.Save(home, it, ck.now()); err != nil {
		t.Fatal(err)
	}
	if err := st.SetHidden(home, true); err != nil {
		t.Fatal(err)
	}
	if got := summaries(st); got != "" {
		t.Errorf("hidden, shown: %s", got)
	}
	if other := st.Home(cal.Event); other == home || other == "" {
		t.Errorf("new ones into the hidden one: %q", other)
	}
	sync(t, st, c, ck)
	st2, _ := cal.Open(st.Dir)
	hidden := 0
	for _, f := range st2.Folders() {
		if f.Hidden {
			hidden++
		}
	}
	if hidden != 1 || summaries(st2) != "" {
		t.Errorf("synced and reopened: %d hidden, %q", hidden, summaries(st2))
	}
	if err := st2.SetHidden(home, false); err != nil || summaries(st2) != "lunch" {
		t.Errorf("shown again: %v %q", err, summaries(st2))
	}
	if st2.SetHidden("nonesuch", true) == nil {
		t.Error("hid a calendar that isn't")
	}
}
