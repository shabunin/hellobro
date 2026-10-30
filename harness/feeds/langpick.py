# Languages ticked (APP_EXTRA a settings file with languages=en ru uk):
# the switcher's "Languages to type in, ticked" puts up their tiles; 5
# ticks Greek, 2 unticks Russian, Esc. Ctrl+Alt+Space twice: Greek, and
# the find line takes "kalhm;era" as καλημέρα (; the accent, a dead
# key). The log says "settings: languages = en uk el", "typing in el".
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start(); time.sleep(3)
mod(32); time.sleep(1.2); keys("ticked"); time.sleep(0.6); key(ENTER); time.sleep(3)
keys("5"); time.sleep(2)                                    # Greek ticked
keys("2"); time.sleep(2)                                    # Russian not
key(ESC); time.sleep(2)
key(SPACE, ctrl=True, alt=True); key(SPACE, ctrl=True, alt=True); time.sleep(2)
keys("kalhm;era"); time.sleep(3)
key(ESC); time.sleep(1)
exit_app()
