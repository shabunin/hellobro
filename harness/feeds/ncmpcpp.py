# For a screenshot: saved host 1 (APP_EXTRA's ssh.conf: an sshd with mpd
# playing, say in a container whose session bus HELLOBRO_MPRIS_BUS names,
# mpd-mpris on it), ncmpcpp there, then q.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
keys("3"); key(ENTER); time.sleep(5)                 # saved host 1: a login shell
keys("ncmpcpp\n"); time.sleep(12)                    # the playlist, a track playing
keys("q"); time.sleep(2)
keys("exit\n"); time.sleep(2)
exit_app()
