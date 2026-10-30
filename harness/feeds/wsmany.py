# The workspaces' tiles with a lot in them (workspaces.go): 1 two
# shells, 2 an illustrated EPUB, 3 a PDF with a part cut out and copied
# beside a notebook it's pasted in, 4 a notebook written in, 5 Wi-Fi
# beside Bluetooth, 6 Settings, 7 a PDF's pages small, 8 the books;
# Meta+O puts them up. A new notebook comes from the launcher's line:
# the switcher's "notebook" finds 3's. The shells start again with nothing of the PC's
# (env -i, a prompt and a title of their own), as in wsshots. No window
# is left showing the launcher (its Kobo shell tile names the PC's
# user). snap.txt in OUT says which snapshot is which. HELLOBRO_BOOKS:
# an EPUB, then two PDFs (the paper third), by name.
# app: hellobro
import os, sys, math
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

OUT = os.path.dirname(os.path.abspath(sys.argv[1]))
notes = open(os.path.join(OUT, "snap.txt"), "w", buffering=1)
def note(what):
    n = len([f for f in os.listdir(os.path.join(OUT, "screen")) if f.endswith("after.png")])
    notes.write("%d %s\n" % (n, what))

KEY_B, KEY_V, KEY_O, KEY_T, KEY_P = 48, 47, 24, 20, 25
def ws(n): mod(1 + n); time.sleep(2)
def book(downs):                                              # the shelf's All tab (by name): Alice, bash, paper
    gui("books"); time.sleep(3)
    left, right = 35 + 230 + 16, W - 51 - min(300, W // 4)    # the tabs, between All Books and back
    tap((left + right) // 2, 160); time.sleep(2)
    for _ in range(downs): key(DOWN)
    key(ENTER); time.sleep(6)
def shell(title, cmd):
    keys("exec env -i TERM=xterm-256color PS1='kobo:~# ' /bin/sh"); key(ENTER); time.sleep(2)
    keys("printf '\\033]0;%s\\007'; clear; %s" % (title, cmd)); key(ENTER); time.sleep(3)
def line(x0, y0, x1, y1, n=16):
    return [(x0 + (x1 - x0) * i / n, y0 + (y1 - y0) * i / n) for i in range(n + 1)]
def wave(x0, x1, y0, amp=40, n=40):
    return [(x0 + (x1 - x0) * i / n, y0 + amp * math.sin(i / n * 4 * math.pi)) for i in range(n + 1)]
def spiral(cx, cy, r, turns=3, n=90):
    return [(cx + r * i / n * math.cos(i / n * turns * 2 * math.pi), cy + r * i / n * math.sin(i / n * turns * 2 * math.pi)) for i in range(n + 1)]
def star(cx, cy, r):
    return [(cx + r * math.sin(k * 4 * math.pi / 5), cy - r * math.cos(k * 4 * math.pi / 5)) for k in range(6)]

start(); time.sleep(3)
keys("2"); time.sleep(3)                                      # 1: the Kobo's sh, twice
shell("sh", "cal; echo; echo 'hello bro'; echo; echo 'scale=40; 4*a(1)' | bc -l")
mod(KEY_B); mod(ENTER); time.sleep(2); keys("2"); time.sleep(3)
shell("squares", "for i in 1 2 3 4 5 6 7 8 9 10 11 12; do printf '%3d %5d %7d\\n' $i $((i*i)) $((i*i*i)); done")

ws(2)                                                         # 2: Alice, a picture
book(0)
for _ in range(3): key(SPACE); time.sleep(3)
note("alice")

ws(3)                                                         # 3: the paper, a part copied, pasted beside
book(2)
half = int(min(W, H) * 0.2)
x0, y0, x1, y1 = W // 2 - half, int(H * 0.30), W // 2 + half, int(H * 0.44)
stroke(line(x0, y0, x1, y1, 20), pen=True); time.sleep(4)
note("cut")
tap(x1 - 524 + 70, y1 + 42); time.sleep(4)                    # copy
mod(KEY_B); gui("notebook"); time.sleep(4)
key(47, ctrl=True); time.sleep(4)                             # Ctrl+V
key(KEY_P); time.sleep(1)                                     # the pen: the picture comes in selected
nx = W * 5 // 8                                               # under the picture, on the page
stroke(wave(nx, W - 100, int(H * 0.36), 25), pen=True); time.sleep(2)
stroke(line(nx, int(H * 0.42), W - 140, int(H * 0.42)), pen=True); time.sleep(3)

ws(4)                                                         # 4: a notebook, written in
mod(ENTER); time.sleep(2)                                     # the launcher's line, not the switcher's,
keys("new notebook"); time.sleep(1); key(ENTER); time.sleep(4) # which goes to 3's notebook
key(KEY_T); time.sleep(2)
tap(W // 6, H // 5); time.sleep(3)
keys("hello bro"); key(ENTER); keys("written on the Kobo"); time.sleep(2)
key(ESC); time.sleep(3)
key(KEY_P); time.sleep(1)
stroke(wave(W // 6, W * 5 // 6, H * 2 // 5, 50), pen=True); time.sleep(2)
stroke(spiral(W // 3, H * 3 // 5, min(W, H) // 9), pen=True); time.sleep(2)
stroke(star(W * 2 // 3, H * 3 // 5, min(W, H) // 9), pen=True); time.sleep(2)
stroke(line(W // 6, H * 4 // 5, W * 5 // 6, H * 4 // 5), pen=True); time.sleep(3)

ws(5)                                                         # 5: the radios
gui("wi-fi"); mod(KEY_B); gui("bluetooth"); time.sleep(3)

ws(6)                                                         # 6: Settings
gui("settings"); time.sleep(3)

ws(7)                                                         # 7: bash's pages, small
book(1)
keys("o"); time.sleep(12)
note("pages")

ws(8)                                                         # 8: the books
gui("books"); time.sleep(4)

mod(KEY_O); time.sleep(8)
note("overview")
time.sleep(4)
note("overview again")
key(ESC); time.sleep(2)
exit_app()
