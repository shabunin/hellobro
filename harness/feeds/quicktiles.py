# The quick menu as tiles: the swipe puts it up, the arrows move along
# and down it; 5 is Airplane (the radios off: the log says "airplane
# mode"), and the menu again shows it on; 5 again turns them back on.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start(); time.sleep(3)
menu(); time.sleep(1)
key(RIGHT); key(DOWN); time.sleep(2)
key(6); time.sleep(5)                     # 5: Airplane
menu(); time.sleep(2)
key(6); time.sleep(5)                     # back on
exit_app()
