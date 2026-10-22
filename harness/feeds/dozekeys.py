# The keys after a light sleep: Ctrl+Alt+S dozes (the music's on), and
# its Ctrl and Alt come up while dozing; an x wakes it. Then plain keys
# must be plain: ↓ moves in the Wi-Fi list (not the light, as
# Ctrl+Alt+↓ would), s scans (not Ctrl+Alt+S, sleep again).
# Wants MUSIC=a folder of albums. The log says: one "doze: dozing", no
# "key: light.down".
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

K4, KS, KX = 5, 31, 45

start(); time.sleep(4)
key(SPACE, ctrl=True, shift=True); time.sleep(6)      # play
key(K4, ctrl=True, alt=True); time.sleep(4)           # Settings
key(KS, ctrl=True, alt=True); time.sleep(5)           # dozes: the music plays on
key(KX); time.sleep(4)                                # wakes
key(DOWN); key(DOWN); time.sleep(3)                   # down the Wi-Fi list
key(KS); time.sleep(4)                                # scan, no sleep
key(KS, ctrl=True, alt=True); time.sleep(5)           # Ctrl+Alt+S still dozes
key(KX); time.sleep(4)
exit_app()
