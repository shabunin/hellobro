# The quick menu as tiles: the swipe puts it up, the arrows move along
# and down it; 5 is Wi-Fi and 6 Bluetooth, each turned off and on again
# with the menu staying up, its tile saying how it is (the log says
# "radio: turning Wi-Fi off…" and so on); 4, the light, off and on
# ("light: 0%"). Then Airplane by its command
# in the switcher: the radios off, and back on.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start(); time.sleep(3)
menu(); time.sleep(3)
key(RIGHT); key(DOWN); time.sleep(2)
key(6); time.sleep(6)                     # 5: Wi-Fi off
key(7); time.sleep(6)                     # 6: Bluetooth off
key(6); time.sleep(6)                     # Wi-Fi on
key(7); time.sleep(6)                     # Bluetooth on
key(5); time.sleep(4)                     # 4: the light off
key(5); time.sleep(4)                     # and on, as it was
key(ESC); time.sleep(2)
gui("airplane"); time.sleep(5)            # the radios off
gui("airplane"); time.sleep(5)            # back on
exit_app()
