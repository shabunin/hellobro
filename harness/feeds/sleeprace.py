# Asleep again right after waking, while Wi-Fi is still turning back on
# (the stand-in takes 2 s): the sleep waits for that before turning it
# off, and the next waking turns it on again. The log: "radios off (wifi
# was on true" both times, and Wi-Fi on at the end (the bar's network).
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

KEY_S = 31
start(); time.sleep(6)
key(KEY_S, shift=True, meta=True); time.sleep(5.6)  # asleep (pretended 3 s), awake: Wi-Fi turning on
key(KEY_S, shift=True, meta=True); time.sleep(14)   # asleep again before it's on
exit_app()
