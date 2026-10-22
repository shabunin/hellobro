# The light is remembered across starts: a swipe down the left edge to
# off, kept in hellobro.json; run again with APP_EXTRA=the first run's
# app, it starts off. See the log ("light swipe", "as last time").
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
stroke([(50, H * 0.3 + i * 16) for i in range(50)])   # down the left edge: off
time.sleep(2)
exit_app()
