# i3's IPC: a subscriber to the window and workspace events, commands
# (a launcher, a split, a workspace, a refused Kobo shell, a kill), the
# tree and the workspaces, through the socket, hellobro-msg (built for
# the PC into OUT) and swaymsg if there is one. All of it in OUT/ipc.txt.
# app: hellobro
import json, os, socket, struct, subprocess, sys, threading
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

OUT = os.path.dirname(os.path.abspath(sys.argv[1]))
SOCK = os.path.join(OUT, "ipc", "ipc.sock")
log = open(os.path.join(OUT, "ipc.txt"), "w", buffering=1)

def conn():
    s = socket.socket(socket.AF_UNIX)
    s.connect(SOCK)
    return s
def send(s, typ, payload=""):
    b = payload.encode()
    s.sendall(b"i3-ipc" + struct.pack("<II", len(b), typ) + b)
def recv(s):
    h = b""
    while len(h) < 14:
        h += s.recv(14 - len(h))
    n, typ = struct.unpack("<II", h[6:])
    b = b""
    while len(b) < n:
        b += s.recv(n - len(b))
    return typ, json.loads(b)
def ask(typ, payload=""):
    s = conn(); send(s, typ, payload); _, r = recv(s); s.close()
    return r
def cmd(line):
    r = ask(0, line)
    log.write("command %r: %s\n" % (line, json.dumps(r)))
    time.sleep(2)
    return r

def events():
    s = conn()
    send(s, 2, '["window", "workspace", "mode", "binding", "tick"]')
    while True:
        try:
            typ, r = recv(s)
        except Exception:
            return
        if typ & 0x80000000:
            c = r.get("container") or r.get("current") or {}
            log.write("event %d %s %s\n" % (typ & 0xff, r.get("change") or r.get("payload"), c.get("name")))
        else:
            log.write("subscribed: %s\n" % json.dumps(r))

start()
time.sleep(1)
threading.Thread(target=events, daemon=True).start()
time.sleep(1)
cmd("open launcher")
cmd("split v; open wifi")
cmd("workspace 3")
cmd("open sh")                                          # refused
cmd("paste")                                            # refused
cmd("workspace 1; [instance=wifi] focus")
cmd("layout tabbed")
cmd("mode resize")
cmd("mode default")
cmd("[instance=launcher] kill")
cmd("frobnicate")
ask(10, "hello")                                        # a tick
def walk(n, d=0):
    name = n.get("name")
    log.write("  " * d + "%s %s %s%s\n" % (n["type"], n["layout"], name, " *" if n["focused"] else ""))
    for c in n["nodes"] + n["floating_nodes"]:
        walk(c, d + 1)
walk(ask(4))
log.write("workspaces: %s\n" % json.dumps([w["name"] for w in ask(1)]))
log.write("version: %s\n" % ask(7)["human_readable"])
log.write("modes: %s\n" % ask(8))
msg = os.environ.get("HELLOBRO_MSG", os.path.join(OUT, "hellobro-msg"))
for args in (["-t", "get_workspaces"], ["focus", "left"], ["open", "sh"], ["-t", "get_binding_state"]):
    p = subprocess.run([msg, "-s", SOCK] + args, capture_output=True, text=True)
    log.write("hellobro-msg %s: %d %s%s\n" % (" ".join(args), p.returncode, p.stdout.strip()[:200], p.stderr.strip()))
    time.sleep(1)
if subprocess.run(["sh", "-c", "command -v swaymsg"], capture_output=True).returncode == 0:
    p = subprocess.run(["swaymsg", "-s", SOCK, "-t", "get_workspaces"], capture_output=True, text=True)
    log.write("swaymsg get_workspaces: %d %s%s\n" % (p.returncode, p.stdout.strip()[:300], p.stderr.strip()[:300]))
    p = subprocess.run(["swaymsg", "-s", SOCK, "workspace", "2"], capture_output=True, text=True)
    log.write("swaymsg workspace 2: %d %s%s\n" % (p.returncode, p.stdout.strip()[:300], p.stderr.strip()[:300]))
time.sleep(2)
exit_app()
