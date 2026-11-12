# The reader's style panel (bookstyle.go): a book in fullscreen, s for
# the panel, the second font (↓ ↓ Enter: ink's focus), bigger (Size's +), more space between the lines, wider
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
keys("s"); time.sleep(4)                             # the panel
key(DOWN); key(DOWN); time.sleep(1); key(ENTER); time.sleep(4)   # the second font
n = int(os.environ.get("BOOK_FONTS", "3"))           # the fonts there are: on to Size's +
for _ in range(n): key(DOWN); time.sleep(0.4)
time.sleep(2)
key(ENTER); time.sleep(3); key(ENTER); time.sleep(3) # bigger
key(DOWN); key(DOWN); key(ENTER); time.sleep(3)      # line spacing up
key(DOWN); key(DOWN); key(ENTER); time.sleep(3)      # margins wider
key(DOWN); key(ENTER); time.sleep(3)                 # not justified
key(ESC); time.sleep(4)                              # the page, set so
exit_app()
