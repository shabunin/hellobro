# For a screenshot: saved host 1 (APP_EXTRA's ssh.conf: a real sshd, say
# on this PC), tmux attach there, framed and fullscreen, then detach.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
tap(tab_x(0), tabs_y); time.sleep(2)                 # SSH tab: connect screen
keys("1"); time.sleep(5)                             # saved host 1: a login shell
keys("tmux attach\n"); time.sleep(8)
key(F11); time.sleep(8)                              # fullscreen
key(F11); time.sleep(3)
key(48, ctrl=True); keys("d"); time.sleep(2)         # Ctrl+B d: detach, the session goes on
keys("exit\n"); time.sleep(2)
exit_app()
