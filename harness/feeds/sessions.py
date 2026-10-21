# Several SSH sessions: one to the server on :2222, one to a second server
# on :2223 that wants a password (this feed runs it: SRV=its binary,
# SRV_DIR=a scratch dir for its host key and authorized_keys). Logs in with
# the password, takes the offer to install the key, switches sessions,
# kills the second server and brings it back (reconnect, with the key
# now), then the Shell tab (two shells) and the System tab.
import os, signal, subprocess, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

def server():
    return subprocess.Popen([os.environ['SRV'], '-listen', ':2223', '-password', 'bro'], cwd=os.environ['SRV_DIR'],
                            stdout=open(os.path.join(os.environ['SRV_DIR'], 'log'), 'a'), stderr=subprocess.STDOUT)
srv = server()
start()
tap(tab_x(5), tabs_y); time.sleep(2)                 # SSH tab
keys("1"); time.sleep(5)                             # session 1: saved host 1
key(KEY_T, shift=True, ctrl=True); time.sleep(1.5)   # session 2
keys("ssh -p 2223 bro@127.0.0.1\n"); time.sleep(4)   # wants a password
keys("bro\n"); time.sleep(3)                         # the key offer
key(ENTER); time.sleep(5)                            # yes: installed, then the shell
key(PGUP, ctrl=True); time.sleep(2)                  # back to session 1
key(PGDN, ctrl=True); time.sleep(2)                  # and 2
srv.send_signal(signal.SIGKILL); srv.wait()          # the connection drops
time.sleep(8)                                        # a reconnect or two fails
srv = server(); time.sleep(12)                       # it comes back, by key
key(F(7), alt=True); time.sleep(3)                   # Shell tab: a shell starts
keys("echo hello from $(whoami); tty; stty size\n"); time.sleep(2)
key(KEY_T, shift=True, ctrl=True); time.sleep(2)     # shell 2
keys("exit\n"); time.sleep(2)                        # exits: Enter starts another
key(ENTER); time.sleep(2)
keys("ls /\n"); time.sleep(2)
tap(tab_x(7), tabs_y); time.sleep(1)                 # System tab
time.sleep(8)
tap(tab_x(5), tabs_y); time.sleep(2)                 # SSH again: session 2 still there
srv.send_signal(signal.SIGTERM)
exit_app()
