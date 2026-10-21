# Types lines into the Keys tab.
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
tap(tab_x(0), tabs_y); time.sleep(2)                 # Keys tab
for i in range(4):
    keys("hello bro this is a typing test number %d " % i); key(28)
    time.sleep(1)
time.sleep(2)
exit_app()
