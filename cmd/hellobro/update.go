package main

// Updating over the air: a new build sent as hellobro.new next to us
// (make push, or any SFTP client) is checked (an ARM binary that runs and
// says it's hellobro), then hellobro restarts into it, keeping the one
// before as hellobro.old. make push asks for that with hellobro-update;
// one that just turns up gets a question first.
//
// The new build is on trial till it's going (its first Tick, and file
// sharing up if it's meant to be): one that hangs gets trialTime, then
// the one before comes back; one that crashes, launch.sh puts back. So a
// bad push from afar, through the relay, doesn't leave the Kobo out of
// reach.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"hellobro/kobo"
)

const (
	newBinary = "hellobro.new"
	oldBinary = "hellobro.old"
	badBinary = "hellobro.bad" // one that failed its trial, to look at
	trialFile = "update-trial" // the version on trial, till it's going
	trialTime = 45 * time.Second
)

var onTrial atomic.Bool

// version is set by the Makefile: the commit and when it was built.
var version = "dev"

type updateState struct {
	seen     string // the hellobro.new looked at last: its size and time
	stable   string // and the time before, so a file still arriving waits
	checkAt  time.Time
	checking bool
}

// checkBinary says what version path is, if it's a hellobro that runs here.
func checkBinary(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", errors.New("no " + newBinary + " here: send it first")
	}
	head := make([]byte, 20)
	_, err = f.ReadAt(head, 0)
	f.Close()
	if err != nil || !bytes.Equal(head[:4], []byte("\x7fELF")) || head[4] != 1 || head[18] != 40 {
		return "", errors.New(newBinary + " isn't a 32-bit ARM program")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, "--version").Output()
	ver, ok := strings.CutPrefix(strings.TrimSpace(string(out)), "hellobro ")
	if err != nil || !ok {
		return "", fmt.Errorf("%s doesn't run here, or isn't hellobro (%v)", newBinary, err)
	}
	return ver, nil
}

// updateTick looks for a new build every few seconds, and asks once one
// has stopped growing.
func (a *bro) updateTick() {
	u := &a.upd
	if u.checking || time.Now().Before(u.checkAt) {
		return
	}
	u.checkAt = time.Now().Add(3 * time.Second)
	path := kobo.AppFile(newBinary)
	st, err := os.Stat(path)
	if err != nil {
		u.seen, u.stable = "", ""
		return
	}
	id := fmt.Sprint(st.Size(), st.ModTime().UnixNano())
	if id != u.stable {
		u.stable = id // once more, three seconds on
		return
	}
	if id == u.seen {
		return // asked about it already
	}
	u.seen, u.checking = id, true
	go func() {
		ver, err := checkBinary(path)
		a.Do(func() {
			u.checking = false
			if err != nil {
				log.Printf("update: %v", err)
				a.share.note = err.Error()
				a.drawShare()
				return
			}
			log.Printf("update: %s is %s (this is %s)", newBinary, ver, version)
			if a.overlay() || a.Calibrating() {
				u.seen = "" // ask once that's gone
				return
			}
			a.ask("A new hellobro is here", fmt.Sprintf("%s (this is %s). Restart into it? The sessions close.", ver, version),
				"restart", func() { a.installUpdate(path, ver) })
		})
	}()
}

// installUpdate puts the new build (version ver) in our place and
// restarts into it.
func (a *bro) installUpdate(path, ver string) {
	exe, err := os.Executable()
	if err != nil {
		log.Printf("update: %v", err)
		return
	}
	old := kobo.AppFile(oldBinary)
	os.Remove(old)
	if err := os.Rename(exe, old); err != nil {
		log.Printf("update: keeping the old one: %v", err)
		return
	}
	if err := os.Rename(path, exe); err != nil {
		log.Printf("update: %v", err)
		os.Rename(old, exe)
		return
	}
	os.Chmod(exe, 0o755)
	if err := os.WriteFile(kobo.AppFile(trialFile), []byte(ver+"\n"), 0o644); err != nil {
		log.Printf("update: no trial: %v", err)
	}
	log.Printf("update: %s in place, %s kept", exe, old)
	a.App.Restart(exe)
}

// updateTrial, first thing: if we're a build on trial, the one before
// comes back unless we're going within trialTime.
func updateTrial() {
	path := kobo.AppFile(trialFile)
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	if strings.TrimSpace(string(b)) != version { // the one before, back: the trial's over
		os.Remove(path)
		return
	}
	onTrial.Store(true) // the log isn't open yet: trialTick says how it went
	time.AfterFunc(trialTime, func() {
		if onTrial.CompareAndSwap(true, false) {
			rollBack(fmt.Sprintf("not going after %v", trialTime))
		}
	})
}

// trialTick passes the trial once we're going.
func (a *bro) trialTick() {
	if onTrial.Load() && (a.setting("share") != "on" || a.share.srv != nil) && onTrial.CompareAndSwap(true, false) {
		os.Remove(kobo.AppFile(trialFile))
		log.Printf("update: %s is going: kept", version)
	}
}

// rollBack puts the build before back in our place and runs it, whatever
// we're doing (stuck, likely).
func rollBack(why string) {
	exe, err := os.Executable()
	old := kobo.AppFile(oldBinary)
	if err == nil {
		_, err = os.Stat(old)
	}
	if err != nil {
		log.Printf("update: %s %s, and there's none before: %v", version, why, err)
		return
	}
	log.Printf("update: %s %s: back to the one before", version, why)
	bad := kobo.AppFile(badBinary)
	os.Remove(bad)
	if err := os.Rename(exe, bad); err != nil {
		log.Printf("update: rollback: %v", err)
		return
	}
	if err := os.Rename(old, exe); err != nil {
		log.Printf("update: rollback: %v", err)
		os.Rename(bad, exe)
		return
	}
	os.Remove(kobo.AppFile(trialFile))
	err = syscall.Exec(exe, []string{exe}, os.Environ())
	log.Printf("update: rollback: exec: %v", err)
}
