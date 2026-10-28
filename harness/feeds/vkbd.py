# The on-screen keyboard, on auto (APP_EXTRA a settings file with
# vkbd=auto): hidden while the keyboard's there; gone 10 s, it shows. The
# Kobo's sh typed by touch: Shift held for one key, | and arrows, Ctrl+L,
# Backspace held. The menu swipe starts on the right strip.
# The keyboard back: it hides; the menu's "On-screen keyboard" shows it
# again (on, now).
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
import feedlib
from feedlib import *

# Where vkbd.go's vkLayout puts the keys, the page being the screen
# under the bar (46 px).
u = W / 15
h = min(max(min(W, H - 46) * 4 // 5 // 15, 56), 84)
sw = min(int(u), 100)
top = H - 5 * h
ROWS = [[(1, '`')] + [(1, c) for c in '1234567890-='] + [(2, 'BS')],
        [(1.5, None)] + [(1, c) for c in 'qwertyuiop[]'] + [(1.5, '\\')],
        [(1.75, None)] + [(1, c) for c in "asdfghjkl;'"] + [(2.25, 'ENTER')],
        [(2.25, None)] + [(1, c) for c in 'zxcvbnm,./'] + [(0.75, None), (1, 'UP'), (1, None)],
        [(2.5, None), (9, ' '), (0.5, None), (1, 'LEFT'), (1, 'DOWN'), (1, 'RIGHT')]]
KEYS = {}
for i, row in enumerate(ROWS):
    x = 0
    for w, k in row:
        if k: KEYS[k] = (int((x + w / 2) * u), top + i * h + h // 2)
        x += w
for side, strip in enumerate([['HOME', 'PGUP', 'PGDN', 'END', 'TAB', 'SHIFT'],
                              ['ESC', 'DEL', 'TAB2', 'MOD', 'ALT', 'CTRL', 'SHIFT2']]):
    kh = min(h, top // len(strip))
    for i, k in enumerate(strip):
        KEYS[k] = (sw // 2 if side == 0 else W - sw // 2, top - (len(strip) - i) * kh + kh // 2)
def vk(*ks):
    for k in ks:
        x, y = KEYS[k]; tap(x, y); time.sleep(0.1)
def hold(k, secs):
    x, y = KEYS[k]; tid[0] += 1
    frame(0, ID=tid[0], TOOL=0, **at(x, y), P=30); time.sleep(secs)
    frame(0, ID=-1); time.sleep(0.9)
def vtype(s):
    for c in s:
        if c == '|': vk('SHIFT', '\\')
        elif c.isupper(): vk('SHIFT', c.lower())
        else: vk(c)

start()
tap(tab_x(0), tabs_y); time.sleep(2)                     # workspace 1: the launcher
feedlib.kbd.close(); time.sleep(14)                       # no keyboard: it shows
vk('2', 'ENTER'); time.sleep(3)                           # the Kobo's sh
vtype("echo hi there | tr a-z A-Z"); vk('ENTER'); time.sleep(1.5)
vk('SHIFT'); time.sleep(1.5)                              # Shift held: the caps show
vk('e', 'c', 'h', 'o', ' ', 'x'); vk('ENTER'); time.sleep(1.5)   # "Echo x": not found
vk('UP', 'UP', 'ENTER'); time.sleep(1.5)                  # the one before again
vk('CTRL'); time.sleep(1); vk('l'); time.sleep(2)         # Ctrl+L: clear
vtype("echo gone gone gone"); hold('BS', 1.5); time.sleep(1)   # Backspace held: it goes on
vk('ESC'); time.sleep(1)                                  # Esc at the edge: a tap
vtype("ls /"); vk('ENTER'); time.sleep(2)
feedlib.kbd = open(os.path.join(feedlib.d, 'event4'), 'wb', buffering=0); time.sleep(4)   # back: it hides
menu(); keys("on-screen"); time.sleep(1); key(ENTER); time.sleep(3)                      # shown again
exit_app()
