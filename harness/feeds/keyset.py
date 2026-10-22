# Settings' Keys: the list, then a command's key asked for (a plain
# letter turned down, Ctrl+Alt+J taken), the list again; the new key
# dims the light, Ctrl+Alt+Up brightens it, Ctrl+Alt+L turns it off and
# on. Then the switcher turns the screen to landscape (a first time
# there: the calibration comes up).
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

K3, KJ, KA, KL = 4, 36, 30, 38

start(); time.sleep(3)
key(K3, ctrl=True, alt=True); time.sleep(4)           # Settings
key(TAB); key(TAB); time.sleep(1)                     # the settings panel
for _ in range(9): key(DOWN)
time.sleep(1)                                         # its last row: Keys
key(ENTER); time.sleep(3)                             # the commands, with their keys
keys("dimmer"); time.sleep(2)
key(ENTER); time.sleep(3)                             # its key, asked for
key(KA); time.sleep(2)                                # a plain letter: no
key(KJ, ctrl=True, alt=True); time.sleep(3)           # taken: the list again
key(ESC); time.sleep(2)
key(KJ, ctrl=True, alt=True); time.sleep(1)           # dimmer
key(UP, ctrl=True, alt=True); time.sleep(3)           # brighter, the gauge up a moment
key(KL, ctrl=True, alt=True); time.sleep(3)           # off
key(KL, ctrl=True, alt=True); time.sleep(3)           # on, as it was
key(KEY_P, ctrl=True, shift=True); time.sleep(1)
keys("landsc"); time.sleep(2)
key(ENTER); time.sleep(5)                             # landscape
exit_app()
