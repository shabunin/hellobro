# The Alpine shell, as far as a PC shows it: Settings, End to its last
# row and up two to "Alpine: system bus access", → turns it on. Then
# a new window, 1 in its launcher: an Alpine session, which can't be a
# jail here (no root): it says why and ends. The log says
# "settings: alpine-bus = on".
# On the Kobo: ssh -t -p 2022 kobo.local hellobro-alpine.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
gui("settings")
key(END); key(UP); key(UP); time.sleep(2)             # Alpine: system bus access
key(RIGHT); time.sleep(3)                             # on
mod(ENTER); time.sleep(2)                             # a launcher beside it
keys("1"); time.sleep(6)                              # Alpine: no jail on a PC
exit_app()
