# The Music tab (portrait): an album, a track, pause and resume, seeking,
# volume, the next track by hand and by the end of one, the header on
# another tab, and back to the albums. Wants MUSIC=a folder of albums.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

row = lambda i: (700, 746 + 26 + 52 * i)              # library rows
button = lambda i: (484 + 58 + 128 * i, 483)          # prev −10 play +10 next vol− vol+
bar = lambda f: (489 + f * 875, 414)

start()
tap(tab_x(2), tabs_y); time.sleep(4)                  # Music tab: the albums
tap(*row(1)); time.sleep(2)                           # the second album's tracks
tap(*row(0)); time.sleep(8)                           # play the first; its cover comes
key(SPACE); time.sleep(3)                             # pause
key(SPACE); time.sleep(3)                             # and on
key(RIGHT); key(RIGHT); time.sleep(3)                 # 20 s on
tap(*button(6)); tap(*button(6)); time.sleep(1)       # louder
tap(*button(4)); time.sleep(6)                        # next track
tap(*bar(0.985)); time.sleep(10)                      # near its end: then the next one by itself
tap(tab_x(0), tabs_y); time.sleep(3)                  # SSH tab: the header says what's on
tap(tab_x(2), tabs_y); time.sleep(2)
tap(1322, 705); time.sleep(2)                         # "albums"
exit_app()
