package cal

import (
	"bufio"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

// Conf is calendar.conf: the CalDAV server's URL (a calendar, the
// calendars' home, or the server's root), the user and the password (an
// app's password, mostly), and how often to sync while the calendar is
// open.
//
//	url = https://dav.example.com/
//	user = me
//	password = an app password
//	every = 15
type Conf struct {
	URL, User, Password string
	Every               time.Duration
}

// LoadConf reads the conf at path.
func LoadConf(path string) (Conf, error) {
	c := Conf{Every: 15 * time.Minute}
	data, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	for sc.Scan() {
		l := strings.TrimSpace(sc.Text())
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		name, val, ok := strings.Cut(l, "=")
		if !ok {
			continue
		}
		name, val = strings.ToLower(strings.TrimSpace(name)), strings.TrimSpace(val)
		switch name {
		case "url", "server":
			c.URL = val
		case "user", "username", "login":
			c.User = val
		case "password", "pass":
			c.Password = val
		case "every":
			if n, err := strconv.Atoi(strings.TrimSuffix(val, "m")); err == nil && n > 0 {
				c.Every = time.Duration(n) * time.Minute
			}
		}
	}
	if c.URL == "" {
		return c, errors.New("no url in it")
	}
	if !strings.HasPrefix(c.URL, "http://") && !strings.HasPrefix(c.URL, "https://") {
		c.URL = "https://" + c.URL
	}
	return c, sc.Err()
}
