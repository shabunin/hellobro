# Helpers for the feeds: they play touch and keyboard input into the FIFOs
# that run.sh hands the app, as the Elan touchscreen and a NuPhy keyboard.
# A feed is run as: feeds/NAME.py INPUT_DIR ROTA
import math, os, struct, sys, time
d, rota = sys.argv[1], int(sys.argv[2])
W, H = (1872, 1404) if rota in (1, 3) else (1404, 1872)
RX, RY = 1871, 1403          # fallback raw ranges the app assumes (swapped portrait panel)

def apply(ax, u, v):
    s, mx, my = ax
    if s: u, v = v, u
    if mx: u = 1 - u
    if my: v = 1 - v
    return u, v
def composed(u, v):            # FBInk quirks (swap, mirX), then the rotation, like the app
    u, v = apply((1, 1, 0), u, v)
    return {0: (u, v), 1: (1 - v, u), 2: (1 - u, 1 - v), 3: (v, 1 - u)}[rota]
axes = min(((s, mx, my) for s in (0, 1) for mx in (0, 1) for my in (0, 1)),
           key=lambda ax: sum(math.dist(apply(ax, *p), composed(*p)) for p in [(.1, .2), (.9, .3), (.4, .8)]))
def raw(x, y):                 # inverse of the app's mapping
    a, b = x / W, y / H
    s, mx, my = axes
    if my: b = 1 - b
    if mx: a = 1 - a
    u, v = (b, a) if s else (a, b)
    return round(u * RX), round(v * RY)

ev = lambda t, c, v: struct.pack('<iiHHi', 0, 0, t, c, v)
SYN = ev(0, 0, 0)
touch = open(os.path.join(d, 'event2'), 'wb', buffering=0)
kbd = open(os.path.join(d, 'event4'), 'wb', buffering=0)
tid = [100]
def frame(slot, **kv):
    codes = {'ID': 0x39, 'TOOL': 0x37, 'X': 0x35, 'Y': 0x36, 'P': 0x3a}
    b = ev(3, 0x2f, slot)
    for k, v in kv.items():
        b += ev(3, codes[k], v)
    touch.write(b + SYN)
def at(x, y): rx, ry = raw(x, y); return dict(X=rx, Y=ry)
def tap(x, y, slot=0):
    tid[0] += 1
    frame(slot, ID=tid[0], TOOL=0, **at(x, y), P=30); time.sleep(0.05)
    frame(slot, ID=-1); time.sleep(0.9)
def hold(x, y, secs=1.2, slot=0):
    """A finger put down at x, y and kept there still for secs."""
    tid[0] += 1
    frame(slot, ID=tid[0], TOOL=0, **at(x, y), P=30); time.sleep(secs)
    frame(slot, ID=-1); time.sleep(0.9)
def stroke(pts, slot=0, pen=False, pressure=None):
    tid[0] += 1
    if pen:   # approach: hover first
        frame(slot, ID=tid[0], TOOL=1, **at(*pts[0]), P=0); time.sleep(0.2)
    for i, (x, y) in enumerate(pts):
        p = pressure(i / max(1, len(pts) - 1)) if pressure else 30
        if i == 0 and not pen:
            frame(slot, ID=tid[0], TOOL=0, **at(x, y), P=p)
        else:
            frame(slot, **at(x, y), P=p)
        time.sleep(0.012)
    if pen:
        frame(slot, P=0); time.sleep(0.3)       # lifted: hovering again
        frame(slot, ID=-1)                       # out of range
    else:
        frame(slot, ID=-1)
    time.sleep(0.6)
def pair(a, b, dx, dy, n=20):
    """Two fingers down at a and b, going dx, dy together in n steps."""
    ids = []
    for slot, (x, y) in enumerate((a, b)):
        tid[0] += 1; ids.append(tid[0])
        frame(slot, ID=tid[0], TOOL=0, **at(x, y), P=30); time.sleep(0.02)
    for i in range(1, n + 1):
        for slot, (x, y) in enumerate((a, b)):
            frame(slot, **at(x + dx * i // n, y + dy * i // n), P=30)
        time.sleep(0.012)
    for slot in (0, 1):
        frame(slot, ID=-1)
    time.sleep(0.6)

def keys(text):
    shifted = '!@#$%^&*()_+QWERTYUIOP{}ASDFGHJKL:"~|ZXCVBNM<>?'
    rows = [(2, "1234567890-=", "!@#$%^&*()_+"), (16, "qwertyuiop[]", "QWERTYUIOP{}"),
            (30, "asdfghjkl;'`", 'ASDFGHJKL:"~'), (43, "\\zxcvbnm,./", "|ZXCVBNM<>?")]
    for ch in text:
        code, shift = {' ': (57, False), '\n': (28, False)}.get(ch, (None, False))
        for first, plain, sh in rows:
            if ch in plain: code = first + plain.index(ch)
            if ch in sh: code, shift = first + sh.index(ch), True
        if shift: kbd.write(ev(1, 42, 1) + SYN)
        kbd.write(ev(1, code, 1) + SYN + ev(1, code, 0) + SYN)
        if shift: kbd.write(ev(1, 42, 0) + SYN)
        time.sleep(0.04)


def start():
    """Waits for the app, then does its touch calibration (a fresh app dir has none)."""
    time.sleep(2.5)
    for t in [(.15, .3), (.75, .8)]:                     # calibration
        tap(t[0] * W, t[1] * H); time.sleep(0.8)
    time.sleep(1.5)

TABS = int(os.environ.get('TABS', 5))  # hellobro: its workspaces (4 for the feeds of its tabs); brotools: Keys Finger Stylus Net Sound System Info
BAR = TABS != 7                        # hellobro: one bar at the top, no header; its first workspace at its left end
tabs_y = 23 if BAR else 76 + 34
tab_x = lambda i: 40 if BAR else (i + 0.5) * min(W // TABS, W // 4)  # hellobro: its first workspace, whatever i
land = W > H
cx0, cy0 = (60, 200)
cw, ch = ((W - 32) * 0.66 - 60, H - 76 - 68 - 46 - 32 - 100) if land else (W - 120, (H - 76 - 68 - 46 - 32) * 0.66 - 110)

def key(code, shift=False, ctrl=False, alt=False, meta=False):
    if not meta and legacy(code, shift, ctrl, alt):
        return
    mods = [m for m, on in ((125, meta), (29, ctrl), (42, shift), (56, alt)) if on]
    for m in mods: kbd.write(ev(1, m, 1) + SYN)
    kbd.write(ev(1, code, 1) + SYN + ev(1, code, 0) + SYN)
    for m in reversed(mods): kbd.write(ev(1, m, 0) + SYN)
    time.sleep(0.15)
UP, DOWN, ENTER, ESC, TAB, BACKSPACE, SPACE, F11 = 103, 108, 28, 1, 15, 14, 57, 87
PGUP, PGDN, KEY_T, LEFT, RIGHT, KEY_P = 104, 109, 20, 105, 106, 25
KEY_E, KEY_F, KEY_M = 18, 33, 50
HOME, END = 102, 107
MINUS = 12
def mod(code, **kv): key(code, meta=True, **kv)       # Meta+key: the window manager's

def fullscreen(): mod(KEY_F)                           # on and off
F = lambda n: 58 + n                                  # F1..F10

def power():
    """Presses the power button: its GPIO (run.sh's stand-in for debugfs) low a moment."""
    g = os.path.join(os.path.dirname(d), 'gpio')
    line = open(g).read()
    open(g, 'w').write(line.replace(') in  hi', ') in  lo'))
    time.sleep(0.4)
    open(g, 'w').write(line)
    time.sleep(0.6)

def menu():
    """hellobro's menu: a swipe up along the right edge."""
    stroke([(W - 50, H * 2 // 3 - i * 15) for i in range(21)]); time.sleep(2)

def exit_app():
    """Taps the header's exit button; in hellobro, Meta+Shift+E (quit):
    full screen, the menu's swipe is the window's."""
    if BAR:
        mod(KEY_E, shift=True)
    else:
        tap(W - 16 - 45, 38)
    time.sleep(3)

# The tabs that were, for the feeds written for them: Ctrl+Alt+1-4 go to
# a workspace laid out as the tab was (opened the first time), and the
# keys that moved about the tabs and panels are the window manager's
# now. Taps at the tabs' places may land elsewhere.
def gui(name):
    """Opens (or focuses) a settings' window by its name, through the switcher."""
    mod(32); time.sleep(1.2); keys(name); time.sleep(0.6); key(ENTER); time.sleep(2.5)
made = set()
def tab(n):
    """Workspace n, as tab n was: 1 SSH (a launcher), 2 Shell (the Kobo's sh), 3 Settings, 4 Power."""
    mod(1 + n); time.sleep(1.5)
    if n in made or n == 1:
        return
    made.add(n)
    if n == 2:
        mod(ENTER); time.sleep(1.5); keys("2"); time.sleep(1.5)
    elif n == 3:                                      # Wi-Fi and Sharing above, Bluetooth and Settings below
        gui("wi-fi"); mod(48); gui("sharing")         # Meta+B: beside it
        mod(105); mod(47); gui("bluetooth")           # Meta+Left, Meta+V
        mod(106); mod(47); gui("settings")
        mod(105); mod(103)                            # back to Wi-Fi
    elif n == 4:
        gui("battery"); mod(48); gui("what uses")
        mod(105)
def legacy(code, shift, ctrl, alt):
    if ctrl and alt and not shift and 2 <= code <= 5:
        tab(code - 1)
    elif ctrl and not alt and code == TAB:
        mod(TAB, shift=shift)                         # the next window
    elif ctrl and shift and not alt and code == KEY_P:
        mod(32)                                       # the switcher
    elif ctrl and shift and not alt and code == KEY_T:
        mod(ENTER)                                    # a new window
    elif ctrl and shift and not alt and code == KEY_F:
        mod(KEY_F)
    else:
        return False
    time.sleep(0.15)
    return True
