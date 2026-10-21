# Connects to the server (server/, on :2222), walks its pages, chats, quits.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
tap(tab_x(0), tabs_y); time.sleep(2)                 # SSH tab: connect screen
keys("1"); time.sleep(5)                             # saved host 1: the wish server
key(DOWN); time.sleep(1); key(ENTER); time.sleep(1.5)   # Todo
key(DOWN); key(DOWN); key(57); time.sleep(1.5)       # toggle an item (space)
keys("a"); keys("buy milk"); key(ENTER); time.sleep(1.5)
key(ESC); key(DOWN); key(ENTER); time.sleep(1)       # Chat
keys("hello from the kobo"); key(ENTER); time.sleep(1.5)
key(ESC); key(ESC); key(DOWN); key(ENTER); time.sleep(2)  # Showcase
tap(W * 0.6, H * 0.5); time.sleep(1.5)               # a click, reported by the app
key(ESC); key(UP); key(UP); key(UP); key(ENTER); time.sleep(3)  # Dashboard
keys("q"); time.sleep(2)                             # quit the remote app: connection closes
exit_app()
