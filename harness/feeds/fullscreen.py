# Fullscreen (Ctrl+Shift+F) on the connect screen and in a session, on and off.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
tap(tab_x(0), tabs_y); time.sleep(2)                 # workspace 1: the launcher
fullscreen(); time.sleep(2)                              # fullscreen connect screen
keys("3"); key(ENTER); time.sleep(5)                                     # connect, fullscreen
key(DOWN); key(DOWN); key(DOWN); key(ENTER); time.sleep(2.5)  # Showcase
fullscreen(); time.sleep(2.5)                            # back to the framed terminal
fullscreen(); time.sleep(2.5)                            # and full again
key(ESC); key(UP); key(UP); key(UP); key(ENTER); time.sleep(2)  # Dashboard
keys("q"); time.sleep(2.5)                           # connection closes, still full
fullscreen(); time.sleep(2.5)                            # framed again
exit_app()
