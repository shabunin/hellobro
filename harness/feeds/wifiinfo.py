# The Wi-Fi panel's info: i on the Settings tab puts the details up, c
# copies them, Esc closes; then the info button, tapped, and a tap
# outside closes it.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

K3, KI, KC = 4, 23, 46  # Settings, I, C
L = W > H
X0, Y0, X1 = 16, 160, W - 16

start(); time.sleep(3)
key(K3, ctrl=True, alt=True); time.sleep(5)           # Settings, Wi-Fi in focus
key(KI); time.sleep(3)                                # the details
key(KC); time.sleep(2)                                # copied
key(ESC); time.sleep(2)
right = X0 + (X1 - X0) * 50 // 100 - 8 if L else X1   # the Wi-Fi panel's right edge
bw = min(250, (right - 3 - 16 - X0 - 3 - 16 - 36) // 4)
tap(X0 + 3 + 16 + 3 * (bw + 12) + bw // 2, Y0 + 3 + 48 + 16 + 32); time.sleep(3)  # info, tapped
tap(W // 2, 20); time.sleep(2)                        # outside: closed
exit_app()
