# The frame for fingers (HELLOBRO_TOUCH=big): a taller bar, title
# strips, tabs and menu rows. Two workspaces, the second tabbed; two
# fingers across go from one to the other (left: the next, right: the
# one before), one finger across doesn't; then the menu.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

KEY_2, KEY_W = 3, 17
def two(dx, y=None):
    """Two fingers across together, dx px, a finger's width apart."""
    y = y or H // 2
    tid[0] += 2
    for i in range(21):
        x = W // 2 - dx // 2 + dx * i // 20
        if i == 0:
            frame(0, ID=tid[0] - 1, TOOL=0, **at(x, y), P=30)
            frame(1, ID=tid[0], TOOL=0, **at(x, y + 150), P=30)
        else:
            frame(0, **at(x, y), P=30)
            frame(1, **at(x, y + 150), P=30)
        time.sleep(0.012)
    frame(0, ID=-1); frame(1, ID=-1)
    time.sleep(3)

start()
tap(tab_x(0), tabs_y); time.sleep(2)                 # workspace 1: the launcher
keys("2"); time.sleep(3)                             # the Kobo's sh
keys("echo one\n"); time.sleep(1)
mod(KEY_2); time.sleep(2)                            # workspace 2
keys("2"); time.sleep(3)
mod(ENTER); time.sleep(2); keys("2"); time.sleep(3)  # a second window
mod(KEY_W); time.sleep(3)                            # tabbed
two(-500)                                            # left: the next, 1
two(500)                                             # right: the one before, 2
stroke([(W // 2 + 250 - 25 * i, H // 2) for i in range(21)]); time.sleep(3)  # one finger: still 2
menu()
exit_app()
