# A saved host's menu in the launcher (3), mounting its files (m), read-
# only (r): without root the harness can't mount, so it says why. Then
# edit (3, e): the line to change, Esc leaves it.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
keys("3"); time.sleep(2)                             # the host's menu
keys("m"); time.sleep(2)                             # mount: read-only or read-write?
keys("r"); time.sleep(4)                             # it tries
keys("3"); time.sleep(1); keys("e"); time.sleep(2)   # edit its line
key(ESC); time.sleep(1)
exit_app()
