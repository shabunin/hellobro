# Fullscreen for any panel, as a tiling window manager's: Ctrl+Shift+F
# gives the one in focus the screen, Ctrl+Tab goes on to the next one,
# fullscreen too. On Settings (Wi-Fi, Bluetooth, Settings; Ctrl+Shift+M
# does nothing there) and Files, left by a swipe in from the top edge.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

K3, K4 = 4, 5  # Settings, Files

start(); time.sleep(3)
key(K3, ctrl=True, alt=True); time.sleep(5)           # Settings, Wi-Fi in focus
fullscreen(); time.sleep(3)
key(TAB, ctrl=True); time.sleep(3)                    # Bluetooth
key(TAB, ctrl=True); time.sleep(3)                    # Settings
key(KEY_M, ctrl=True, shift=True); time.sleep(2)      # no folding fullscreen
fullscreen(); time.sleep(3)
key(K4, ctrl=True, alt=True); time.sleep(3)           # Files
fullscreen(); time.sleep(3)                           # the list alone
stroke([(W // 2, 10 + i * 40) for i in range(8)]); time.sleep(3)   # in from the top: back
exit_app()
