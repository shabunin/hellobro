# The keys in a question and on the windows. A saved host's menu (3, after
# Alpine and the Kobo's sh): ↓ to "mount its files…", Enter; read-only or
# read-write: → to read-write, Esc. The menu again: ↓ ↓ to edit, Enter,
# Esc. Then forget (f) in it: a question with two answers side by side,
# → to no, ← back to forget, Esc leaves it, the host still saved.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
keys("3"); time.sleep(2)                             # the host's menu: connect selected
key(DOWN); time.sleep(2)                             # mount its files…
key(ENTER); time.sleep(2)                            # read-only or read-write?
key(RIGHT); time.sleep(2)                            # read-write
key(ESC); time.sleep(1)
keys("3"); time.sleep(1)
key(DOWN); key(DOWN); time.sleep(2)                  # edit
key(ENTER); time.sleep(2)                            # its line
key(ESC); time.sleep(1)
keys("3"); time.sleep(1)
keys("f"); time.sleep(2)                             # Forget wish-server?
key(RIGHT); time.sleep(2)                            # no
key(LEFT); time.sleep(2)                             # forget
key(ESC); time.sleep(2)
exit_app()
