# Switches server pages until the terminal earns its cleanup flash.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
tap(tab_x(0), tabs_y); time.sleep(2)                 # workspace 1: the launcher
keys("3"); key(ENTER); time.sleep(5)                                     # saved host 3, connect: the wish server
for p in "14" * 15:
    keys(p); time.sleep(0.8)
keys("4")
time.sleep(4)
keys("q"); time.sleep(2)
exit_app()
