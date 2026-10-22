# Starts with the layout feeds/layout.py left (APP_EXTRA=its OUT/app):
# workspace 1 as it was, then 2, its shell started again.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
time.sleep(4)
mod(3); time.sleep(4)                                 # Meta+2
exit_app()
