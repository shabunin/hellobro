# A finger held on the menu's Wi-Fi tile opens Wi-Fi's window, held on
# Bluetooth's, Bluetooth's; a tap on Wi-Fi's still only turns it off
# (the log says "held on a tile" twice, then "radio: turning Wi-Fi
# off…"); 7 opens Settings. Where the tiles are: WIFI_XY and BT_XY,
# as "x,y" (portrait's by default; landscape's 936,472 and 1292,472).
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

wifi = tuple(int(v) for v in os.environ.get("WIFI_XY", "700,552").split(","))
bt = tuple(int(v) for v in os.environ.get("BT_XY", "1057,552").split(","))
start(); time.sleep(3)
menu(); time.sleep(3)
hold(*wifi); time.sleep(4)                # Wi-Fi's window
menu(); time.sleep(3)
hold(*bt); time.sleep(4)                  # Bluetooth's
menu(); time.sleep(3)
tap(*wifi); time.sleep(5)                 # a tap: Wi-Fi off, the menu up
key(ESC); time.sleep(2)
menu(); time.sleep(3)
key(8); time.sleep(4)                     # 7: Settings' window
exit_app()
