# Panels folding: on the Settings tab Wi-Fi's − box folds it, then
# Bluetooth's (now under Wi-Fi's strip); Tab skips past them; a tap on
# Wi-Fi's strip opens it, the switcher opens Bluetooth. Music's Now
# playing folds from the switcher, Files' list from its box. The header
# shows the stand-in Wi-Fi's name and address.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

K3, K4, K5 = 4, 5, 6
L = W > H
X0, Y0, X1, Y1 = 16, 160, W - 16, H - 62   # the page's body
def box(right, top): return right - 30, top + 26  # a panel's − / + box

start(); time.sleep(3)
key(K4, ctrl=True, alt=True); time.sleep(5)           # Settings
wifi_right = X0 + (X1 - X0) * 50 // 100 - 8 if L else X1
tap(*box(wifi_right, Y0)); time.sleep(3)              # Wi-Fi folded
tap(*box(X1, Y0 + 54 + 16)); time.sleep(3)            # Bluetooth folded, under it
key(TAB); time.sleep(2)                               # the focus: Settings, the open one
tap(400, Y0 + 27); time.sleep(3)                      # Wi-Fi's strip: open again
key(KEY_P, ctrl=True, shift=True); time.sleep(1)
keys("bluetooth panel open"); time.sleep(2)
key(ENTER); time.sleep(3)                             # and Bluetooth
key(K3, ctrl=True, alt=True); time.sleep(3)           # Music
key(KEY_P, ctrl=True, shift=True); time.sleep(1)
keys("now playing panel min"); time.sleep(2)
key(ENTER); time.sleep(3)                             # Now playing folded: the library has it all
key(K5, ctrl=True, alt=True); time.sleep(3)           # Files
list_top = Y0 if L else Y0 + (Y1 - Y0) * 31 // 100 + 8
tap(*box(X1, list_top)); time.sleep(3)                # its list folded
exit_app()
