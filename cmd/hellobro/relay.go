package main

// The relay: a way in to the Kobo from anywhere, through a server with a
// fixed address that both ends reach (a VPS). While Settings has it on,
// and file sharing too, we keep an SSH connection to that server open
// with a remote forward (ssh -R): its 127.0.0.1:2023 comes to our share
// server, port 2022. The computer gets there with
//
//	ssh -J me@vps -p 2023 root@127.0.0.1
//
// end to end: the server only carries the bytes, and our own pairing
// keys still decide who gets in. relay.conf, next to us, says where:
//
//	kobo@203.0.113.7[:port]  [2023]
//
// we log in with our own key (id_ed25519.pub), which the server's
// authorized_keys lets do this and nothing else (the README has how).
// The connection comes back by itself when it drops: Wi-Fi gone, the
// Kobo asleep. While it's on, hellobro doesn't quit or sleep by itself.

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"hellobro/kobo"
)

const (
	relayPort      = 2023 // on the server, if relay.conf doesn't say
	relayKeepalive = 30 * time.Second
	relayRetryMin  = 5 * time.Second
	relayRetryMax  = time.Minute
)

const relayConfExample = `# hellobro relay: where the Kobo keeps its way in open, one line:
#   user@server[:port]  [port on the server, 2023 if not given]
# The server's 127.0.0.1:2023 then comes to the Kobo's file sharing;
# from a computer: ssh -J user@server -p 2023 root@127.0.0.1
# Put the Kobo's key (id_ed25519.pub, next to this file) in that user's
# authorized_keys on the server, as the README says.
# kobo@203.0.113.7  2023
`

type relayConf struct {
	host sshHost // the server
	port int     // where it listens for us, on its loopback
}

type relayState struct {
	stop   chan struct{} // closed to end it
	conf   relayConf
	status string // how it is, for the Sharing panel
	up     bool
	err    string // why it isn't running at all
}

// loadRelayConf reads relay.conf, writing the example if there's none.
func loadRelayConf() (relayConf, error) {
	path := kobo.AppFile("relay.conf")
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		os.WriteFile(path, []byte(relayConfExample), 0o644)
		return relayConf{}, errors.New("say where in relay.conf, next to hellobro")
	}
	if err != nil {
		return relayConf{}, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if c, ok, err := parseRelayLine(sc.Text()); ok || err != nil {
			return c, err
		}
	}
	return relayConf{}, errors.New("relay.conf says nothing: user@server [port]")
}

// parseRelayLine reads a line of relay.conf; false for a comment or a
// blank.
func parseRelayLine(line string) (relayConf, bool, error) {
	fields := strings.Fields(line)
	if len(fields) == 0 || strings.HasPrefix(fields[0], "#") {
		return relayConf{}, false, nil
	}
	h, err := parseSSHTarget(fields[0])
	if err != nil {
		return relayConf{}, true, fmt.Errorf("relay.conf: %w", err)
	}
	c := relayConf{host: h, port: relayPort}
	if len(fields) > 1 {
		if c.port, err = strconv.Atoi(fields[1]); err != nil || c.port < 1 || c.port > 65535 {
			return relayConf{}, true, fmt.Errorf("relay.conf: bad port %q", fields[1])
		}
	}
	return c, true, nil
}

func (c relayConf) String() string { return fmt.Sprintf("%s, port %d", c.host, c.port) }

// server is the server's address as ssh -J takes it: host, or host:port.
func (c relayConf) server() string {
	if h, p, _ := net.SplitHostPort(c.host.addr); p == "22" {
		return h
	}
	return c.host.addr
}

// relayApply starts or stops the relay as the settings say: on, with
// file sharing up.
func (a *bro) relayApply() {
	r := &a.relay
	on := a.setting("relay") == "on" && a.share.srv != nil
	switch {
	case on && r.stop == nil:
		if _, err := clientKey(); err != nil { // its .pub goes on the server: there from the first
			log.Printf("relay: client key: %v", err)
		}
		c, err := loadRelayConf()
		if err != nil {
			r.err = err.Error()
			log.Printf("relay: %v", err)
			break
		}
		srv := a.share.srv
		r.conf, r.err, r.status, r.up = c, "", "connecting…", false
		r.stop = make(chan struct{})
		log.Printf("relay: on, through %s", c)
		a.DrawFooter() // no auto-exit
		go runRelay(c, srv.ServeConn, r.stop, func(up bool, s string) {
			a.Do(func() {
				if r.stop != nil {
					r.up, r.status = up, s
					a.drawShare()
				}
			})
		})
	case !on && r.stop != nil:
		close(r.stop)
		r.stop, r.up, r.status = nil, false, ""
		log.Print("relay: off")
		a.DrawFooter() // auto-exit, again
	case !on:
		r.err = ""
	}
	a.drawShare()
}

// relayOn says whether the relay is meant to be up (it may be retrying).
func (a *bro) relayOn() bool { return a.relay.stop != nil }

// runRelay keeps the relay up till stop closes, telling status how it is.
// serve takes each connection that comes through it.
func runRelay(c relayConf, serve func(net.Conn), stop <-chan struct{}, status func(up bool, s string)) {
	wait := relayRetryMin
	for {
		start := time.Now()
		err := relayOnce(c, serve, stop, func() { status(true, "up") })
		select {
		case <-stop:
			return
		default:
		}
		if time.Since(start) > 2*relayRetryMax { // it was up a good while: try again soon
			wait = relayRetryMin
		}
		log.Printf("relay: %v; again in %v", err, wait)
		status(false, fmt.Sprintf("down (%v), again at %s", err, time.Now().Add(wait).Format("15:04:05")))
		select {
		case <-stop:
			return
		case <-time.After(wait):
		}
		wait = min(2*wait, relayRetryMax)
	}
}

// relayOnce connects to the server, has it listen for us and hands what
// comes to serve, till the connection goes or stop closes. Not through
// 127.0.0.1: the Kobo leaves its loopback down.
func relayOnce(c relayConf, serve func(net.Conn), stop <-chan struct{}, up func()) error {
	signer, err := clientKey()
	if err != nil {
		return fmt.Errorf("client key: %w", err)
	}
	cfg := &ssh.ClientConfig{
		Config: ssh.Config{Ciphers: sshCiphers},
		User:   c.host.user, Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: tofuHostKeys(func(s string) { log.Printf("relay: %s: %s", c.host.addr, s) }),
		Timeout:         20 * time.Second,
	}
	client, err := sshDial(c.host, cfg, signer)
	if err != nil {
		return err
	}
	defer client.Close()
	ln, err := client.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(c.port)))
	if err != nil {
		return fmt.Errorf("listening on the server's port %d: %w", c.port, err)
	}
	log.Printf("relay: up, %s port %d comes to file sharing", c.host.addr, c.port)
	up()

	done := make(chan error, 2)
	go func() { done <- client.Wait() }()
	go func() { // a server gone quiet (Wi-Fi off under us) shows as no answer
		t := time.NewTicker(relayKeepalive)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				answered := make(chan error, 1)
				go func() {
					_, _, err := client.SendRequest("keepalive@openssh.com", true, nil)
					answered <- err
				}()
				select {
				case err := <-answered:
					if err != nil {
						done <- err
						return
					}
				case <-time.After(relayKeepalive):
					done <- errors.New("the server stopped answering")
					return
				}
			case <-stop:
				done <- nil
				return
			}
		}
	}()
	var wg sync.WaitGroup
	go func() {
		for {
			rc, err := ln.Accept()
			if err != nil {
				return
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				serve(rc)
			}()
		}
	}()
	err = <-done
	client.Close() // and with it, the listener and what it carries
	wg.Wait()
	if err == nil || errors.Is(err, io.EOF) {
		err = errors.New("the connection closed")
	}
	return err
}
