# For a screenshot: saved host 1 (APP_EXTRA's ssh.conf: a real sshd, say
# on this PC), tmux attach there, framed and fullscreen, then detach.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
tap(tab_x(0), tabs_y); time.sleep(2)                 # workspace 1: the launcher
keys("3"); key(ENTER); time.sleep(5)                                     # saved host 3, connect: a login shell
keys("tmux attach\n"); time.sleep(8)
fullscreen(); time.sleep(8)                              # fullscreen
fullscreen(); time.sleep(3)
key(48, ctrl=True); keys("d"); time.sleep(2)         # Ctrl+B d: detach, the session goes on
keys("exit\n"); time.sleep(2)
exit_app()
