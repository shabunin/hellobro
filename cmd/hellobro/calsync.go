package main

// The calendar's store and its syncing are the app's, not the window's:
// with calendar.conf, it syncs whenever there's a network, every 15
// minutes (calendar.conf's every), the window open or not, so the sleep
// screen's agenda is as fresh as it can be.

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"hellobro/cal"
	"hellobro/kobo"
)

type calState struct {
	st      *cal.Store
	openErr string
	syncing bool
	again   bool      // asked for while one ran: another after it
	synced  time.Time // last in step
	said    string    // what went wrong, if it did
	news    string    // what the last sync brought, for the window to say
	next    time.Time // the next look at whether to sync
}

// calStore is the store, opened the first time; nil if it can't be.
func (a *bro) calStore() *cal.Store {
	c := &a.cal
	if c.st == nil && c.openErr == "" {
		st, err := cal.Open(kobo.AppFile("calendar"))
		if err != nil {
			log.Printf("calendar: %v", err)
			c.openErr = err.Error()
			return nil
		}
		c.st = st
	}
	return c.st
}

// online says Wi-Fi has a network and an address, as last looked at.
func (a *bro) online() bool { ws := a.net.ws; return ws.On && ws.State == "COMPLETED" && ws.IP != "" }

// calTick syncs when it's time, if there's calendar.conf and a network.
func (a *bro) calTick() {
	c := &a.cal
	now := time.Now()
	if c.syncing || a.slp.asleep || now.Before(c.next) {
		return
	}
	c.next = now.Add(time.Minute) // a look again in a minute, if not now
	if _, err := os.Stat(kobo.AppFile(calConf)); err != nil {
		return
	}
	if !a.online() {
		c.next = now.Add(10 * time.Second) // the network may be just coming
		return
	}
	a.calSyncNow()
}

// calSoon has it sync in a few seconds: something changed.
func (a *bro) calSoon() {
	if a.cal.syncing {
		a.cal.again = true
		return
	}
	a.cal.next = time.Now().Add(calAfter)
}

// calSyncNow syncs with what calendar.conf says: the CalDAV server, the
// calendars subscribed to.
func (a *bro) calSyncNow() {
	c := &a.cal
	st := a.calStore()
	if st == nil {
		return
	}
	if c.syncing {
		c.again = true
		return
	}
	conf, err := cal.LoadConf(kobo.AppFile(calConf))
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			c.said = calConf + ": " + err.Error()
		}
		return
	}
	c.syncing, c.said = true, ""
	agent := "hellobro/" + version
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		var res cal.Synced
		var errs []string
		if conf.URL != "" {
			cl := &cal.Client{URL: conf.URL, User: conf.User, Password: conf.Password, UserAgent: agent}
			r, err := st.Sync(ctx, cl, time.Now)
			res = r
			if err != nil {
				log.Printf("calendar: sync: %v", err)
				errs = append(errs, calErr(err))
			}
		}
		if len(conf.ICS) > 0 || conf.URL != "" { // and those no longer subscribed to go
			r, err := st.SyncSubs(ctx, &http.Client{Timeout: time.Minute}, agent, conf.ICS)
			res.Got, res.Removed = res.Got+r.Got, res.Removed+r.Removed
			if err != nil {
				log.Printf("calendar: subscriptions: %v", err)
				errs = append(errs, calErr(err))
			}
		}
		a.Do(func() {
			c.syncing = false
			c.next = time.Now().Add(conf.Every)
			if len(errs) > 0 {
				c.said = "not synced: " + strings.Join(errs, "; ")
				c.next = time.Now().Add(min(conf.Every, 5*time.Minute))
			} else {
				c.synced = time.Now()
				log.Printf("calendar: synced: %s", res)
			}
			if res != (cal.Synced{}) {
				c.news = res.String()
			}
			if c.again {
				c.again = false
				c.next = time.Now()
			}
			a.inkRedraw(calKey)
		})
	}()
}

// calErr is err, short: the strip has room for a few words.
func calErr(err error) string {
	s := err.Error()
	var se *cal.StatusError
	switch {
	case errors.As(err, &se) && se.Code == 401:
		return "the server says the user or password is wrong"
	case strings.Contains(s, "no such host"), strings.Contains(s, "network is unreachable"):
		return "offline?"
	case errors.Is(err, context.DeadlineExceeded):
		return "the server took too long"
	}
	if len(s) > 120 {
		s = s[:120] + "…"
	}
	return s
}
