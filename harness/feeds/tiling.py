# The window manager: a launcher on workspace 1, Meta+Enter another beside
# it, a settings window from the switcher, split below, tabbed and back,
# a shell on workspace 2 moved to 1, floating, resized, fullscreen, put
# away and back, closed.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

KEY_B, KEY_V, KEY_W, KEY_E, KEY_Q, KEY_R, KEY_D = 48, 47, 17, 18, 16, 19, 32
NUM = lambda n: 1 + n                                  # the 1..9 keys' codes
start()
time.sleep(2)                                         # the launcher, alone
mod(ENTER); time.sleep(2.5)                           # another, beside it
mod(KEY_D); time.sleep(1.5)                           # the switcher
keys("wi-fi"); time.sleep(1)
key(ENTER); time.sleep(3)                             # the Wi-Fi window, beside them
mod(KEY_V); mod(ENTER); time.sleep(2.5)               # a launcher below it
mod(KEY_W); time.sleep(2.5)                           # tabbed
mod(KEY_E); time.sleep(2.5)                           # split again
mod(NUM(2)); time.sleep(2)                            # workspace 2: empty
mod(ENTER); time.sleep(2)
keys("2"); time.sleep(2)                              # the Kobo's shell
keys("echo hello from 2\n"); time.sleep(2)
mod(NUM(1), shift=True); time.sleep(2)                # to workspace 1
mod(NUM(1)); time.sleep(2.5)
mod(SPACE, shift=True); time.sleep(2.5)               # floating
mod(KEY_R); time.sleep(1)                             # resize mode
key(RIGHT); key(RIGHT); key(DOWN); time.sleep(1.5)
key(ESC); time.sleep(1.5)
mod(SPACE, shift=True); time.sleep(2.5)               # tiled again
mod(KEY_F); time.sleep(2.5)                           # fullscreen
mod(KEY_F); time.sleep(2.5)
mod(MINUS, shift=True); time.sleep(2.5)               # put away
mod(MINUS); time.sleep(2.5)                           # back, floating
mod(KEY_Q, shift=True); time.sleep(2.5)               # closed
exit_app()
