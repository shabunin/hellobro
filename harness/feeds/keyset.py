# Settings' Keys: a command's new key. "keys" in the switcher lists the
# commands; "fullscreen" narrows them, Enter asks for its key. a alone
# won't do (it's for typing: the question says so); Ctrl+Alt+F does,
# and the list comes back with it. Enter again, Delete: its defaults
# back. Then Esc twice, and the log says what keys.conf got.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

KEY_A, KEY_F, KEY_DELETE = 30, 33, 111
start()
gui("keys"); time.sleep(2)                            # the commands
keys("fullscreen"); time.sleep(2)
key(ENTER); time.sleep(3)                             # Key for: …
key(KEY_A); time.sleep(3)                             # alone: no
key(KEY_F, ctrl=True, alt=True); time.sleep(3)        # set: the list again
key(ENTER); time.sleep(3)                             # the question again
key(KEY_DELETE); time.sleep(3)                        # the defaults back
key(ESC); time.sleep(1)
exit_app()
