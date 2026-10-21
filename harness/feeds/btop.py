# For a screenshot: saved host 1 (APP_EXTRA's ssh.conf: a real sshd, say
# on this PC), btop there, framed and fullscreen, then q.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
tap(tab_x(0), tabs_y); time.sleep(2)                 # SSH tab: connect screen
keys("1"); time.sleep(5)                             # saved host 1: a login shell
keys("btop\n"); time.sleep(10)                       # its graphs fill in
key(F11); time.sleep(10)                             # fullscreen
key(F11); time.sleep(3)
keys("q"); time.sleep(2)
keys("exit\n"); time.sleep(2)
exit_app()
