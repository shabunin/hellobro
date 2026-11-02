# Two fingers on a notebook whose fingers turn pages (run it with
# settings note-finger=turns in APP_EXTRA), full screen: a workspace
# swipe either way, no page turned; one finger still turns it.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *
start(); time.sleep(3)
mod(3); time.sleep(2); gui("notebook"); time.sleep(3)
fullscreen(); time.sleep(3)
pair((W // 2, H // 3), (W // 2, H // 2), W // 3, 0); time.sleep(3)   # right: workspace 1
pair((W // 2, H // 3), (W // 2, H // 2), -W // 3, 0); time.sleep(3)  # left: back to the notebook
stroke([(W * 3 // 4 - i * 30, H // 2) for i in range(20)]); time.sleep(3)  # one finger: turns the page
exit_app()
