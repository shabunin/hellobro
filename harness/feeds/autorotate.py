# Auto-rotate (kobo/tilt.go): the Kobo held upright, then turned on its
# side, with the setting off, then on; then upright again. Wants
# HELLOBRO_TILT (a file the feed writes the KX122's byte into) and
# APP_EXTRA with a touch-calibration.conf for rotations 0 and 1.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

TILT = os.environ['HELLOBRO_TILT']
def held(v):
    with open(TILT, 'w') as f:
        f.write(v)

held('08')                                     # upright
start(); time.sleep(3)
held('20'); time.sleep(6)                      # on its side: off, nothing turns
gui("auto-rotate"); time.sleep(4)              # the setting on: it turns
time.sleep(6)
held('08'); time.sleep(8)                      # upright again
exit_app()
