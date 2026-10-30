# The workspaces at a glance (workspaces.go): the Kobo's sh beside the
# launcher on 1, a launcher on 2; a tap on the bar's empty part puts
# them up, drawn small, with + for a new one. "new", Enter: workspace 3
# with a launcher. Meta+O again, 1: back on 1. The log says "workspace
# number 3", then "workspace \"1\"".
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

KEY_O = 24
start(); time.sleep(3)
mod(ENTER); time.sleep(2); keys("2"); time.sleep(3)          # a launcher beside, the Kobo's sh in it
mod(3); time.sleep(2)                                        # workspace 2
mod(ENTER); time.sleep(3)
tap(W // 2, 20); time.sleep(3)                               # the bar: the workspaces
keys("new"); time.sleep(1); key(ENTER); time.sleep(3)        # a new one: 3
mod(KEY_O); time.sleep(3)
keys("1"); time.sleep(3)                                     # back to 1
exit_app()
