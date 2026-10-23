# Types ssh command lines on the fullscreen connect screen, then connects.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
tap(tab_x(0), tabs_y); time.sleep(2)                 # workspace 1: the launcher
fullscreen(); time.sleep(2)                              # fullscreen
for _ in range(3):                                   # type a command line, slowly, and wipe it
    keys("ssh -p 2222 bro@127.0.0.1"); time.sleep(1)
    for _ in range(25): key(14)
    time.sleep(1)
keys("ssh -p 2222 bro@127.0.0.1"); key(ENTER); time.sleep(5)   # connects
keys("q"); time.sleep(3)
fullscreen(); time.sleep(2)
exit_app()
