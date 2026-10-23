# The frame for fingers (HELLOBRO_TOUCH=big): a taller bar, title
# strips, tabs and menu rows. Two workspaces, the second tabbed; swipes
# along the bottom go from one to the other (left: the next, right: the
# one before); then the menu.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

KEY_2, KEY_W = 3, 17
def bottom(dx):
    """A swipe along the bottom, dx px across."""
    stroke([(W // 2 - dx // 2 + dx * i // 20, H - 20) for i in range(21)]); time.sleep(3)

start()
tap(tab_x(0), tabs_y); time.sleep(2)                 # workspace 1: the launcher
keys("2"); time.sleep(3)                             # the Kobo's sh
keys("echo one\n"); time.sleep(1)
mod(KEY_2); time.sleep(2)                            # workspace 2
keys("2"); time.sleep(3)
mod(ENTER); time.sleep(2); keys("2"); time.sleep(3)  # a second window
mod(KEY_W); time.sleep(3)                            # tabbed
bottom(-500)                                         # left: the next, 1
bottom(500)                                          # right: the one before, 2
menu()
exit_app()
