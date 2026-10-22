# The light is remembered across starts, in hellobro.json: Ctrl+Alt+L
# turns it off (from Nickel's 30%). Run again with APP_EXTRA=the first
# run's app: it starts off, and Ctrl+Alt+L brings back the 30%. See the
# log ("as last time", "light:").
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
key(38, ctrl=True, alt=True); time.sleep(3)          # Ctrl+Alt+L
exit_app()
