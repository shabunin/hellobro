# Leaves a layout to come back to (feeds/restore.py): on workspace 1 a
# launcher and, below it, Wi-Fi; on 2 the Kobo's shell and Battery
# floating over it; workspace 1 shown.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

KEY_V, KEY_D = 47, 32
NUM = lambda n: 1 + n
start()
mod(KEY_V)
mod(KEY_D); time.sleep(1.5); keys("wi-fi"); time.sleep(1); key(ENTER); time.sleep(3)
mod(NUM(2)); time.sleep(1.5)
mod(ENTER); time.sleep(2); keys("2"); time.sleep(2); keys("echo before\n"); time.sleep(1.5)
mod(KEY_D); time.sleep(1.5); keys("battery"); time.sleep(1); key(ENTER); time.sleep(3)
mod(SPACE, shift=True); time.sleep(2)                 # floating
mod(NUM(1)); time.sleep(3)
exit_app()
