# The docs' screenshot of the workspaces' tiles (workspaces.go): a
# shell beside Wi-Fi on 1, a book on 2, Bluetooth over Settings on 3;
# Meta+O puts them up. The shell starts again with nothing of the PC's
# (env -i, a prompt and a title of its own) and the screen cleared: the
# harness's would show the PC's user and host. The battery's window is
# left out, as it lists the PC's processes. snap.txt in OUT says which
# snapshot is which. HELLOBRO_BOOKS: a.pdf, then an EPUB.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

OUT = os.path.dirname(os.path.abspath(sys.argv[1]))
notes = open(os.path.join(OUT, "snap.txt"), "w", buffering=1)
def note(what):
    n = len([f for f in os.listdir(os.path.join(OUT, "screen")) if f.endswith("after.png")])
    notes.write("%d %s\n" % (n, what))

KEY_B, KEY_V, KEY_O = 48, 47, 24
start(); time.sleep(3)
keys("2"); time.sleep(3)                                      # the Kobo's sh
keys("exec env -i TERM=xterm-256color PS1='kobo:~# ' /bin/sh"); key(ENTER); time.sleep(2)
keys("printf '\\033]0;sh\\007'; clear; cal; echo; echo 'hello bro'"); key(ENTER); time.sleep(3)
mod(KEY_B); gui("wi-fi"); time.sleep(2)                       # beside it
mod(3); time.sleep(2)                                         # 2: a book
gui("books"); time.sleep(3)
key(DOWN); time.sleep(1); key(ENTER); time.sleep(6)
for _ in range(2): key(SPACE); time.sleep(3)
mod(4); time.sleep(2)                                         # 3: Bluetooth over Settings
gui("bluetooth"); mod(KEY_V); gui("settings"); time.sleep(3)
mod(KEY_O); time.sleep(6)
note("overview")
key(ESC); time.sleep(2)
exit_app()
