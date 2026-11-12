# ink's gallery (cmd/gallery, ink/gallery): Tab rings the controls; a
# switch flipped; a dropdown opened and an option picked; the pickers'
# calendar opened, a month on, a day; the page of text dragged slowly
# under the finger (black and white as it moves, grays once it's let
# go); the list made live and dragged; the table sorted by year; a
# folder of the tree opened; a menu's action and its toast; Esc quits.
# The page starts under the header (144 px down); places as ink lays
# them out at 1.42 px a Dp, portrait (ROTA 0).
# app: gallery
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

Y = 144                                                # the page's top
tab = lambda k: tap(W * (2 * k + 1) // 20, Y + 105)    # Controls Pickers Text Lists Scroll Table Tree Grid Popups About

def drag(a, b, steps=30, pause=0.05):
    """A finger held at a, moved slowly to b, lifted."""
    tid[0] += 1
    frame(0, ID=tid[0], TOOL=0, **at(*a), P=30); time.sleep(0.1)
    for i in range(1, steps + 1):
        frame(0, **at(a[0] + (b[0] - a[0]) * i // steps, a[1] + (b[1] - a[1]) * i // steps), P=30)
        time.sleep(pause)
    time.sleep(0.3)
    frame(0, ID=-1); time.sleep(1.5)

start()
key(TAB); time.sleep(1.5)                              # the quit button, ringed
tap(W - 60, Y + 237); time.sleep(2)                    # Wi-Fi's switch: on
tap(W - 150, Y + 429); time.sleep(2)                   # Page size: open
tap(W - 300, Y + 681); time.sleep(2)                   # A5
tab(1); time.sleep(2)                                  # Pickers
tap(1029, Y + 245); time.sleep(2)                      # the calendar
tap(1362, Y + 318); time.sleep(2)                      # December
tap(1238, Y + 599); time.sleep(2)                      # a day
tab(4); time.sleep(2)                                  # Scroll
drag((700, Y + 1000), (700, Y + 400)); time.sleep(1)   # the text up, slowly
drag((700, Y + 500), (700, Y + 800), steps=10); time.sleep(1)  # back a bit, quicker
tab(3); time.sleep(2)                                  # Lists
tap(W - 60, Y + 184); time.sleep(2)                    # under the finger: on
drag((700, Y + 1300), (700, Y + 500)); time.sleep(1)
tab(5); time.sleep(2)                                  # Table
tap(1179, Y + 311); time.sleep(2)                      # by year
tab(6); time.sleep(2)                                  # Tree
tap(144, Y + 301); time.sleep(2)                       # Diary opens
tab(8); time.sleep(2)                                  # Popups
tap(117, Y + 249); time.sleep(2)                       # the menu
tap(162, Y + 438); time.sleep(5)                       # export as PDF: a toast, gone
key(ESC); time.sleep(3)                                # it quits
