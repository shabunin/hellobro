# ink's gallery (cmd/gallery, ink/gallery): Tab rings the controls; a
# switch flipped; a dropdown opened and an option picked; the long one
# (AI model) opened, its options' page swiped, Esc; a menu's action
# picked and its toast; the list's page swiped; Esc quits. The page
# starts under the header and tabs (144 px down); places as ink lays
# them out at 1.42 px a Dp.
# app: gallery
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

Y = 144                                                # the page's top
tab = lambda k: tap(W * (2 * k + 1) // 12, Y + 105)    # Controls Text Lists Tiles Popups About
start()
key(TAB); time.sleep(1.5)                              # the quit button, ringed
tap(W - 60, Y + 237); time.sleep(2)                    # Wi-Fi's switch: on
tap(W - 150, Y + 429); time.sleep(2)                   # Page size: open
tap(W - 300, Y + 681); time.sleep(2)                   # A5
tap(W - 150, Y + 573); time.sleep(2)                   # AI model: open
stroke([(W - 250, Y + 1000 - i * 50) for i in range(10)]); time.sleep(2.5)  # its options, the next page (if paged)
key(ESC); time.sleep(2)
tab(4); time.sleep(2)                                  # Popups
tap(117, Y + 249); time.sleep(2)                       # the menu
tap(162, Y + 438); time.sleep(1.5)                     # export as PDF: a toast
time.sleep(4)                                          # the toast goes
tab(2); time.sleep(2)                                  # Lists
stroke([(W // 2, H * 3 // 4 - i * 60) for i in range(12)]); time.sleep(3)  # the next page
key(ESC); time.sleep(3)                                # it quits
