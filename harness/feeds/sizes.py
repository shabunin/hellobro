# Sizes (run it with HELLOBRO_TOUCH empty: the settings say, for fingers
# first): the launcher, All Books, Settings, a notebook and the switcher
# for fingers; "Sizes" in the switcher: compact, all of it laid out
# again; and back.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

def sizes():
    mod(32); time.sleep(1.2); keys("setting: sizes"); time.sleep(1); key(ENTER); time.sleep(4)

start(); time.sleep(3)
gui("books"); time.sleep(4)
gui("settings"); time.sleep(3)
gui("notebook"); time.sleep(3)
mod(32); time.sleep(2); key(ESC); time.sleep(1)                       # the switcher's rows
sizes()                                                               # compact
mod(32); time.sleep(2); key(ESC); time.sleep(1)
sizes()                                                               # for fingers again
exit_app()
