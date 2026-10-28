# Bookmarks (bookmarks.go): a book in fullscreen, b on a page, the
# ribbon under another, the ☰ under the page for the contents, Tab to
# the bookmarks, the first one opened, then one taken away by d, and
# the Aa for the style panel. HELLOBRO_BOOKS as the books feed's.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
gui("books"); time.sleep(3)
key(ENTER); time.sleep(4)                            # the first book
fullscreen(); time.sleep(3)
for _ in range(3): key(SPACE); time.sleep(2)         # a page of text
keys("b"); time.sleep(3)                             # bookmarked: a ribbon at the top
for _ in range(4): key(SPACE); time.sleep(2)
tap(W // 2 + 100, H - 75); time.sleep(3)             # the ribbon under the page
key(SPACE); time.sleep(2)
tap(W // 2 - 100, H - 75); time.sleep(3)             # ☰: the contents
key(TAB); time.sleep(3)                              # the bookmarks
key(UP); key(ENTER); time.sleep(3)                   # the first one's page
keys("t"); time.sleep(2); key(TAB); time.sleep(3)    # the bookmarks again
keys("d"); time.sleep(3)                             # the first taken away
key(ESC); time.sleep(3)
tap(W // 2, H - 75); time.sleep(3)                   # Aa: the style panel
key(ESC); time.sleep(3)
exit_app()
