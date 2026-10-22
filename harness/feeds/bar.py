# The bar, with no header: the workspaces with their windows' titles,
# the status, the ≡ menu (tapped open, closed by Esc); an empty
# workspace's keys; a notification over the IPC socket; the Battery
# window with the busiest processes; and quitting by Shift+Meta+E.
# snap.txt in OUT says which snapshot is which.
# app: hellobro
import json, os, socket, struct, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

OUT = os.path.dirname(os.path.abspath(sys.argv[1]))
notes = open(os.path.join(OUT, "snap.txt"), "w", buffering=1)
def note(what):
    n = len([f for f in os.listdir(os.path.join(OUT, "screen")) if f.endswith("after.png")])
    notes.write("%d %s\n" % (n, what))

def command(line):
    s = socket.socket(socket.AF_UNIX)
    s.connect(os.path.join(OUT, "ipc", "ipc.sock"))
    b = line.encode()
    s.sendall(b"i3-ipc" + struct.pack("<II", len(b), 0) + b)
    h = s.recv(14)
    n = struct.unpack("<II", h[6:])[0]
    notes.write("ipc %s: %s\n" % (line, s.recv(n).decode()))
    s.close()

KEY_B, KEY_V, KEY_E = 48, 47, 18
start(); time.sleep(3)
note("launcher")
mod(KEY_B); gui("wi-fi")                              # beside the launcher
mod(KEY_V); gui("battery"); time.sleep(8)             # under Wi-Fi: a sample, then what changed
note("battery")
mod(3); time.sleep(3)                                 # workspace 3: empty, its keys
note("help")
mod(ENTER); time.sleep(2); keys("2"); time.sleep(3)   # a Kobo sh there
note("bar with two")
tap(W - 30, 23); time.sleep(3)                        # the menu
note("menu")
key(ESC); time.sleep(2)
command('notify "the build is done, bro"'); time.sleep(2)
note("notify")
command("notify"); time.sleep(1)                      # nothing to say: an error
time.sleep(3)
key(KEY_E, shift=True, meta=True); time.sleep(3)      # quit, as i3's exit
