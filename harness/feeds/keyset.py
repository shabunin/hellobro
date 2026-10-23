# Settings' Keys: the list, then a command's key asked for (a plain
# letter turned down, Meta+Y taken), the list again; the new key dims the
# light, Meta+] brightens it, Meta+\ turns it off and on. The log says
# "keys: light.down: [meta+y]", then "light:" four times.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

KA, KY, KRB, KBS = 30, 21, 27, 43

start(); time.sleep(3)
gui("settings"); time.sleep(2)
key(END); time.sleep(1)                               # its last row: Keys
key(ENTER); time.sleep(3)                             # the commands, with their keys
keys("dimmer"); time.sleep(2)
key(ENTER); time.sleep(3)                             # its key, asked for
key(KA); time.sleep(2)                                # a plain letter: no
key(KY, meta=True); time.sleep(3)                     # taken: the list again
key(ESC); time.sleep(2)
key(KY, meta=True); time.sleep(1)                     # dimmer
key(KRB, meta=True); time.sleep(3)                    # brighter (Meta+]), the gauge up a moment
key(KBS, meta=True); time.sleep(3)                    # off (Meta+\)
key(KBS, meta=True); time.sleep(3)                    # on, as it was
exit_app()
