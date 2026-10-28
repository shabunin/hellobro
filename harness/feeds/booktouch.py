# The reader by finger only, once a book is open: swipes across turn
# the pages, a tap at the page's top right corner bookmarks it, ☰ for
# the contents, a swipe up and down them, the Bookmarks tab, the
# bookmark opened, the corner again to take it away, then the contents'
# "‹ books" back to the shelf. HELLOBRO_BOOKS as the books feed's.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

def swipe(x0, y0, x1, y1):
    stroke([(x0 + (x1 - x0) * i / 12, y0 + (y1 - y0) * i / 12) for i in range(13)])
    time.sleep(2)

start()
gui("books"); time.sleep(3)
key(ENTER); time.sleep(4)                            # the first book
fullscreen(); time.sleep(3)
for _ in range(4): swipe(W * 0.7, H / 2, W * 0.3, H / 2)   # on
swipe(W * 0.3, H / 2, W * 0.7, H / 2)                      # back
tap(W - 70, 50); time.sleep(3)                       # the corner: bookmarked
swipe(W * 0.7, H / 2, W * 0.3, H / 2)
tap(W // 2 - 100, H - 75); time.sleep(3)             # ☰: the contents
swipe(W / 2, H * 0.7, W / 2, H * 0.3)                # a screenful on
swipe(W / 2, H * 0.3, W / 2, H * 0.7)                # and back
L, R = 182, W - 292
tap(L + 3 * (R - L) // 4, 40); time.sleep(3)         # the Bookmarks tab
tap(W // 2, 124); time.sleep(3)                      # the bookmark's page
tap(W - 70, 50); time.sleep(3)                       # the corner: taken away
tap(W // 2 - 100, H - 75); time.sleep(3)             # ☰
tap(90, 40); time.sleep(4)                           # ‹ books: the shelf
exit_app()
