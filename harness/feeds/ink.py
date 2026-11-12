# ink's gallery (cmd/gallery, ink/gallery): Tab rings the controls; a
# switch flipped; a dropdown opened and an option picked; the page of
# text dragged slowly under the finger (black and white as it moves,
# grays once it's let go), the map flicked; the list dragged; the pen and
# a finger drawing; a menu's action and its toast; the Calendar: a month,
# a day, a new event typed in; the Mail: a message read, a reply typed,
# sent; Esc quits. The page starts under the header (144 px down);
# places as ink lays them out at 1.42 px a Dp, portrait (ROTA 0).
# app: gallery
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

Y = 144                                                # the page's top
APPS = [408, 906, 1257]                                # Components Calendar Mail
TABS_X = [101, 271, 408, 529, 661, 796, 921, 1038, 1161, 1306]  # Controls … Popups
app = lambda k: tap(APPS[k], Y + 42)
tab = lambda k: tap(TABS_X[k], Y + 115)

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
tap(W - 60, Y + 247); time.sleep(2)                    # Wi-Fi's switch: on
tap(1074, Y + 439); time.sleep(2)                      # Page size: open
tap(1074, Y + 681); time.sleep(2)                      # an option
tab(4); time.sleep(2)                                  # Scroll
drag((700, Y + 700), (700, Y + 300)); time.sleep(1)    # the text up, slowly
stroke([(900 - 40 * i, Y + 1300 - 25 * i) for i in range(10)]); time.sleep(3)  # the map flicked: it coasts
tab(3); time.sleep(2)                                  # Lists: under the finger
drag((700, Y + 1300), (700, Y + 500)); time.sleep(1)
tab(8); time.sleep(2)                                  # Draw
stroke([(300 + 30 * i, Y + 600 + int(120 * ((i % 8) / 8))) for i in range(30)], pen=True,
       pressure=lambda t: int(400 + 3000 * t)); time.sleep(1)   # the pen, harder as it goes
tap(173, Y + 260); time.sleep(1.5)                     # a finger draws: on
stroke([(300 + 25 * i, Y + 1000 + 8 * i) for i in range(30)]); time.sleep(1.5)
tab(9); time.sleep(2)                                  # Popups
tap(96, Y + 258); time.sleep(2)                        # the menu
tap(140, Y + 440); time.sleep(5)                       # an action: a toast, gone
app(1); time.sleep(2)                                  # Calendar: a week
tap(796, Y + 187); time.sleep(2)                       # Month
tap(820, Y + 989); time.sleep(2)                       # the 20th: its day
tap(1336, Y + 117); time.sleep(2)                      # + new
keys("Post the letters"); time.sleep(1)
key(ENTER); time.sleep(2)                              # Enter: saved
app(2); time.sleep(2)                                  # Mail
tap(139, Y + 630); time.sleep(2)                       # a message read
tap(975, Y + 116); time.sleep(2)                       # reply
keys("Lovely, thank you."); time.sleep(1.5)
tap(1274, Y + 109); time.sleep(3)                      # send
key(ESC); time.sleep(3)                                # it quits
