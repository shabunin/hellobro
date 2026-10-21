# The relay, with OpenSSH on this PC playing the VPS and the computer,
# and hellobro-server (cd server && go run .) the computer's sshd: the
# Kobo keeps its way in open on the "VPS", the computer comes in through
# it (a command, a file, hellobro-update and the restart, the trial
# passed), and the Kobo reaches the "computer" through the "VPS" (via in
# ssh.conf), where the computer keeps a tunnel to its sshd.
#
# It wants it all set up first, and says how in its env:
#   RELAY_VPS   the "VPS", user@127.0.0.1:port, an sshd taking PC_KEY
#   RELAY_PORT  where the Kobo listens on it (its relay.conf says so too)
#   PC_KEY      the computer's key, paired with the Kobo (APP_EXTRA has
#               it in authorized_keys) and let in on the "VPS"
# and APP_EXTRA with settings (share=on, relay=on), relay.conf, the
# Kobo's id_ed25519, and an ssh.conf with a host "home" via the "VPS".
# app: hellobro
import os, subprocess, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

K1, K5 = 2, 6
OUT = os.path.dirname(os.path.abspath(sys.argv[1]))
VPS, PORT, KEY = os.environ["RELAY_VPS"], os.environ["RELAY_PORT"], os.environ["PC_KEY"]
log = open(os.path.join(OUT, "pc.log"), "w")
user, hostport = VPS.split("@")
vhost, vport = hostport.rsplit(":", 1)
BASE = ["-F", "/dev/null", "-i", KEY, "-o", "IdentitiesOnly=yes", "-o", "BatchMode=yes", "-o", "ConnectTimeout=10",
        "-o", "UserKnownHostsFile=" + os.path.join(OUT, "pc_known_hosts"), "-o", "StrictHostKeyChecking=accept-new"]
JUMP = ["-o", "ProxyCommand=ssh " + " ".join(BASE) + " -p %s -W %%h:%%p %s@%s" % (vport, user, vhost)]  # ssh -J, with our options

def pc(*cmd):
    r = subprocess.run(list(cmd), stdin=subprocess.DEVNULL, capture_output=True, text=True, timeout=60, start_new_session=True)
    log.write("$ %s\n%s%s(exit %d)\n" % (" ".join(cmd[-2:]), r.stdout, r.stderr, r.returncode)); log.flush()
    return r

start()
key(K5, ctrl=True, alt=True); time.sleep(8)            # Files: sharing on, the relay up
pc("ssh", *BASE, *JUMP, "-p", PORT, "root@127.0.0.1", "hellobro-version"); time.sleep(2)
with open(os.path.join(OUT, "hello.txt"), "w") as f:
    f.write("through the relay\n")
pc("scp", *BASE, *JUMP, "-P", PORT, os.path.join(OUT, "hello.txt"), "root@127.0.0.1:"); time.sleep(4)
app = os.path.join(OUT, "app", "hellobro")
pc("scp", *BASE, *JUMP, "-P", PORT, app, "root@127.0.0.1:" + os.path.join(OUT, "app", "hellobro.new"))
pc("ssh", *BASE, *JUMP, "-p", PORT, "root@127.0.0.1", "hellobro-update"); time.sleep(14)  # the restart, on trial
key(K5, ctrl=True, alt=True); time.sleep(8)            # the relay back up, after it
pc("ssh", *BASE, *JUMP, "-p", PORT, "root@127.0.0.1", "hellobro-version"); time.sleep(2)
key(TAB, ctrl=True, shift=True); time.sleep(1.5)       # the switcher: home, via the "VPS"
keys("home"); time.sleep(1)
key(ENTER); time.sleep(10)
key(K1, ctrl=True, alt=True); time.sleep(3)
