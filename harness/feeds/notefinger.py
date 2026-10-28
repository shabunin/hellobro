# The notebook with no pen ever seen: a finger (a stylus without a
# battery, to the panel) writes a wave and a circle.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start(); time.sleep(3)
gui("notebook"); fullscreen(); time.sleep(4)
stroke([(200 + (W - 400) * i / 60, H // 3 + 60 * math.sin(i / 60 * 4 * math.pi)) for i in range(61)]); time.sleep(3)
stroke([(W // 2 + 150 * math.cos(i / 48 * 2 * math.pi), H // 2 + 150 * math.sin(i / 48 * 2 * math.pi)) for i in range(49)]); time.sleep(4)
exit_app()
