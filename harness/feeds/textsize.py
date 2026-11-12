# Settings, Text size: ↓ to its slider, → three times: 150%, the settings,
# then the launcher (Mod+V) bigger; ← back to 100%.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start(); time.sleep(2)
gui("settings"); time.sleep(2)
fullscreen(); time.sleep(2)
for _ in range(3):
    key(DOWN); time.sleep(0.4)
for _ in range(3):
    key(RIGHT); time.sleep(1.5)
time.sleep(2)                                         # 150%
fullscreen(); time.sleep(1)
mod(47); time.sleep(3)                                # the launcher, under it
mod(47); time.sleep(2)
fullscreen(); time.sleep(2)
for _ in range(3):
    key(LEFT); time.sleep(1.5)
time.sleep(2)                                         # back to 100%
exit_app()
