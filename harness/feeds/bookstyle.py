# The reader's style panel (bookstyle.go): a book in fullscreen, s for
# the panel, the next font, bigger, more space between the lines, wider
# margins, not justified, back to the page. HELLOBRO_BOOKS and
# HELLOBRO_FONTS as the books feed's.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
gui("books"); time.sleep(3)
key(ENTER); time.sleep(4)                            # the first book
fullscreen(); time.sleep(3)
for _ in range(3): key(SPACE); time.sleep(2)         # a page of text
keys("s"); time.sleep(4)                             # the panel, on the font in use
key(RIGHT); time.sleep(4)                            # the next font
n = int(os.environ.get("BOOK_FONTS", "3"))           # the fonts there are: down to Size
for _ in range(n - 1): key(DOWN); time.sleep(0.4)
time.sleep(2)
key(RIGHT); time.sleep(3); key(RIGHT); time.sleep(3) # bigger
key(DOWN); key(RIGHT); time.sleep(3)                 # line spacing up
key(DOWN); key(RIGHT); time.sleep(3)                 # margins wider
key(DOWN); key(ENTER); time.sleep(3)                 # not justified
key(ESC); time.sleep(4)                              # the page, set so
exit_app()
