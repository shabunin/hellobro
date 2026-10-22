# File sharing with a computer, the computer played by OpenSSH on this
# PC (on 127.0.0.1, and with the pairing code the harness fixes:
# HELLOBRO_FAKE_PAIR_CODE): the Settings tab's Sharing, on, pairing, an
# album sent with scp, a command, then make push's way: hellobro.new, hellobro-update, and the
# restart into it. Wants ssh, scp and ssh-keygen.
# app: hellobro
import os, subprocess, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

K3 = 4  # Settings
OUT = os.path.dirname(os.path.abspath(sys.argv[1]))
PC = os.path.join(OUT, "pc")
os.makedirs(PC, exist_ok=True)
log = open(os.path.join(OUT, "pc.log"), "w")
KEY = os.path.join(PC, "key")
subprocess.run(["ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", KEY], stdin=subprocess.DEVNULL)
ASK = os.path.join(PC, "askpass.sh")
with open(ASK, "w") as f:
    f.write("#!/bin/sh\necho 246810\n")
os.chmod(ASK, 0o755)
OPTS = ["-F", "/dev/null", "-i", KEY, "-o", "IdentitiesOnly=yes", "-o", "UserKnownHostsFile=" + os.path.join(PC, "known_hosts"),
        "-o", "StrictHostKeyChecking=accept-new", "-o", "ConnectTimeout=10"]

def pc(*cmd, ask=False):
    env = dict(os.environ, SSH_ASKPASS=ASK, SSH_ASKPASS_REQUIRE="force") if ask else None
    if not ask:
        cmd = (cmd[0], "-o", "BatchMode=yes") + cmd[1:]
    r = subprocess.run(list(cmd), env=env, stdin=subprocess.DEVNULL, capture_output=True, text=True, timeout=60,
                       start_new_session=True)
    log.write("$ %s\n%s%s(exit %d)\n" % (" ".join(cmd[-2:]), r.stdout, r.stderr, r.returncode)); log.flush()
    return r

start()
key(K3, ctrl=True, alt=True); time.sleep(4)            # Settings: sharing off
key(TAB, ctrl=True); key(TAB, ctrl=True); time.sleep(2)  # Sharing in focus
keys("o"); time.sleep(3)                               # on
keys("p"); time.sleep(3)                               # pairing: the code
pc("ssh", *OPTS, "-p", "2022", "bro@127.0.0.1", "echo paired; uname -m", ask=True); time.sleep(3)
album = os.path.join(PC, "New Album")
os.makedirs(album, exist_ok=True)
for i in range(1, 4):
    with open(os.path.join(album, "%02d. Song.mp3" % i), "wb") as f:
        f.write(os.urandom(400000))
pc("scp", *OPTS, "-P", "2022", "-r", album, "bro@127.0.0.1:Music/"); time.sleep(5)
pc("ssh", *OPTS, "-p", "2022", "bro@127.0.0.1", "hellobro-version"); time.sleep(1)
app = os.path.join(OUT, "app")
pc("scp", *OPTS, "-P", "2022", os.path.join(app, "hellobro"), "bro@127.0.0.1:" + os.path.join(app, "hellobro.new"))
pc("ssh", *OPTS, "-p", "2022", "bro@127.0.0.1", "hellobro-update"); time.sleep(12)   # restarted into it
key(K3, ctrl=True, alt=True); time.sleep(4)            # sharing is on again, after the restart
pc("ssh", *OPTS, "-p", "2022", "bro@127.0.0.1", "hellobro-version"); time.sleep(2)
exit_app()
