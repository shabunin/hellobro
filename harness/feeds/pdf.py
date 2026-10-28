# PDFs in the reader (bookpdf.go): the shelf with a PDF or two, the
# second opened in fullscreen, pages turned, its style panel to fit the
# width, screenfuls turned, back to the whole page, the contents (its
# outline, if any), then the shelf and the third, its outline. Wants
# HELLOBRO_BOOKS with an EPUB and two PDFs, a manual (no outline) second
# and a paper (an outline) third, by name.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
gui("books"); time.sleep(4)
key(DOWN); key(ENTER); time.sleep(6)                 # the manual
fullscreen(); time.sleep(5)
for _ in range(2): key(SPACE); time.sleep(4)
keys("s"); time.sleep(3)                             # its style panel
key(ENTER); time.sleep(3)                            # fit the width
key(ESC); time.sleep(6)
for _ in range(3): key(SPACE); time.sleep(4)         # screenfuls
keys("s"); time.sleep(3); key(ENTER); time.sleep(2); key(ESC); time.sleep(5)  # the whole page again
keys("t"); time.sleep(3)                             # its contents
tap(90, 40); time.sleep(4)                           # ‹ books
key(DOWN); key(DOWN); key(ENTER); time.sleep(6)      # the paper
key(SPACE); time.sleep(4)
keys("t"); time.sleep(3)                             # its outline
key(DOWN); key(DOWN); key(ENTER); time.sleep(5)
exit_app()
