# Ctrl+= / Ctrl+- / Ctrl+0 font sizes in a session.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
tap(tab_x(0), tabs_y); time.sleep(2)                 # SSH tab
keys("3"); key(ENTER); time.sleep(5)                                     # saved host 3, connect: the wish server
keys("4"); time.sleep(2)                              # Showcase: shows the window size
for _ in range(3): key(13, ctrl=True); time.sleep(2)  # Ctrl+= x3: 32 px
for _ in range(5): key(12, ctrl=True); time.sleep(2)  # Ctrl+- x5: 22 px
key(11, ctrl=True); time.sleep(3)                     # Ctrl+0: 26 px
keys("q"); time.sleep(2)
exit_app()
