# F11 on the connect screen and in a session, on and off.
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
tap(tab_x(5), tabs_y); time.sleep(2)                 # SSH tab
key(F11); time.sleep(2)                              # fullscreen connect screen
keys("1"); time.sleep(5)                             # connect, fullscreen
key(DOWN); key(DOWN); key(DOWN); key(ENTER); time.sleep(2.5)  # Showcase
key(F11); time.sleep(2.5)                            # back to the framed terminal
key(F11); time.sleep(2.5)                            # and full again
key(ESC); key(UP); key(UP); key(UP); key(ENTER); time.sleep(2)  # Dashboard
keys("q"); time.sleep(2.5)                           # connection closes, still full
key(F11); time.sleep(2.5)                            # framed again
exit_app()
