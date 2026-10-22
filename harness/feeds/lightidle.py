# The light goes off after 30 s untouched (the default), and a touch
# brings it back: see the log ("light: off after", "light: back to").
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
time.sleep(34)                                        # untouched: off
tap(W // 2, H // 2); time.sleep(2)                    # back
exit_app()
