# Folding from the keys: Ctrl+Shift+M folds the panel in focus (the
# focus goes to an open one) or opens it; Ctrl+Tab lands on a folded
# strip too, where keys wait and Enter opens it. On Settings, Music and
# two SSH sessions (the test server, :2222).
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

K3, K4 = 4, 5

start(); time.sleep(3)
key(K4, ctrl=True, alt=True); time.sleep(5)           # Settings, Wi-Fi in focus
key(KEY_M, ctrl=True, shift=True); time.sleep(3)      # Wi-Fi folded, Bluetooth in focus
key(TAB, ctrl=True, shift=True); time.sleep(2)        # back on Wi-Fi's strip
keys("n"); time.sleep(2)                              # waits: nothing
key(ENTER); time.sleep(3)                             # Wi-Fi open, in focus
key(K3, ctrl=True, alt=True); time.sleep(3)           # Music, the library in focus
key(KEY_M, ctrl=True, shift=True); time.sleep(3)      # the library folded: Now playing has it
key(KEY_M, ctrl=True, shift=True); time.sleep(3)      # Now playing folded too
key(TAB, ctrl=True); time.sleep(2)                    # Now playing's strip
key(KEY_M, ctrl=True, shift=True); time.sleep(3)      # opened
tap(tab_x(0), tabs_y); time.sleep(2)                  # SSH tab
keys("1"); time.sleep(5)
key(KEY_T, shift=True, ctrl=True); time.sleep(2)
keys("1"); time.sleep(5)                              # session 2, in focus
key(KEY_M, ctrl=True, shift=True); time.sleep(3)      # folded: session 1 has it all
key(TAB, ctrl=True); time.sleep(2)                    # session 2's strip
keys("lost"); time.sleep(2)                           # waits: not typed
key(ENTER); time.sleep(3)                             # session 2 open
keys("echo two\n"); time.sleep(2)
exit_app()
