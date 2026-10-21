# Font size swipes on the right edge, and a scroll mid-screen.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
tap(tab_x(0), tabs_y); time.sleep(2)                 # SSH tab
keys("1"); time.sleep(5)
keys("4"); time.sleep(2)
stroke([(W - 50, 1300 - i * 15) for i in range(41)]); time.sleep(3)   # up 600 px
stroke([(W - 50, 900 + i * 15) for i in range(21)]); time.sleep(3)    # down 300 px
stroke([(700, 1300 - i * 15) for i in range(41)]); time.sleep(2)      # mid-screen: a scroll, not font
keys("q"); time.sleep(2)
exit_app()
