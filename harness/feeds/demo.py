# A tour of the tabs: drawing, the network probes, sound, typing, light swipes.
# app: brotools
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
tap(tab_x(1), tabs_y); time.sleep(1)                 # Finger tab
stroke([(cx0 + cw * i / 40, cy0 + ch * (0.5 + 0.4 * math.sin(i / 40 * 2 * math.pi))) for i in range(41)])
stroke([(cx0 + cw / 2 + ch * 0.3 * math.cos(a / 30 * 2 * math.pi), cy0 + ch / 2 + ch * 0.3 * math.sin(a / 30 * 2 * math.pi)) for a in range(31)])
tap(cx0 + cw * 0.2, cy0 + ch * 0.2)                  # a dot
stroke([(cx0 + 50 + i * 10, cy0 + 50) for i in range(30)], pen=True, pressure=lambda t: 500)  # pen: ignored here
time.sleep(1.5)
tap(tab_x(2), tabs_y); time.sleep(1)                 # Stylus tab
spiral = [(cx0 + cw / 2 + (20 + i * 4) * math.cos(i / 8), cy0 + ch / 2 + (20 + i * 4) * math.sin(i / 8)) for i in range(90)]
stroke(spiral, pen=True, pressure=lambda t: int(4095 * (0.1 + 0.9 * t)))
stroke([(cx0 + 40, cy0 + ch - 60), (cx0 + cw - 40, cy0 + ch - 60)], slot=1)   # a palm: ignored
stroke([(cx0 + 40 + i * (cw - 80) / 60, cy0 + 80 + 30 * math.sin(i / 4)) for i in range(61)], pen=True, pressure=lambda t: int(4095 * math.sin(t * math.pi)))
time.sleep(1.5)
tap(tab_x(3), tabs_y); time.sleep(3)                 # Network tab: Wi-Fi status runs by itself
if land:
    scan_btn, mdns_btn, get_btn = (W - 75, 186), (W - 75, 785), (928 - 60, 725)
else:
    scan_btn, mdns_btn, get_btn = (W - 75, 590), (W - 75, 1019), (W - 75, 1448)
tap(*scan_btn); time.sleep(12)
tap(*mdns_btn); time.sleep(9)
tap(*get_btn); time.sleep(6)
tap(tab_x(4), tabs_y); time.sleep(4)                 # Sound tab
tap(tab_x(5), tabs_y); time.sleep(8)                 # System tab
tap(tab_x(6), tabs_y); time.sleep(2)                 # Info tab
tap(tab_x(0), tabs_y); time.sleep(1)                 # Keyboard tab
keys("hello bro\nthis is the keyboard tab, typed on a fake NuPhy =)\n")
time.sleep(2)
stroke([(50, H * 0.7 - i * 12) for i in range(50)])   # light swipe up on the Keyboard tab
time.sleep(1)
stroke([(50, H * 0.5 + i * 12) for i in range(30)])   # and down
time.sleep(1)
tap(tab_x(1), tabs_y); time.sleep(1)                 # Finger tab: the edge of the canvas draws
stroke([(60, cy0 + 100 + i * 10) for i in range(40)])
time.sleep(1.5)
exit_app()
