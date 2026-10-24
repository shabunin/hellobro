# read-kitty (readkitty.go): a PDF, then an EPUB, a page at a time, in
# the Kobo's sh (the PC's, here). READ_KITTY: a PC build of hellobro by
# that name; READ_DOCS: a directory with test.pdf, test.epub and pic.jpg.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

rk, docs = os.environ["READ_KITTY"], os.environ["READ_DOCS"]
start()
tap(tab_x(0), tabs_y); time.sleep(2)                 # workspace 1: the launcher
keys("2"); key(ENTER); time.sleep(3)                 # the Kobo's sh
keys("HOME=" + docs + " " + rk + " " + docs + "/test.pdf\n"); time.sleep(5)
keys(" "); time.sleep(3)                             # page 2
keys("3\n"); time.sleep(3)                           # page 3
keys("b"); time.sleep(3)                             # back to 2
keys("q"); time.sleep(2)
keys("HOME=" + docs + " " + rk + " " + docs + "/test.epub\n"); time.sleep(6)
keys("+++"); time.sleep(6)                           # bigger text
keys(" "); time.sleep(3)
keys("q"); time.sleep(2)
keys("HOME=" + docs + " " + rk + " " + docs + "/test.pdf\n"); time.sleep(5)   # back where it was: 2
keys("++"); time.sleep(6)                            # zoomed in: 225%
keys(" "); time.sleep(4)                             # down the page
keys("l"); time.sleep(4)                             # to the right
keys("w"); time.sleep(4)                             # the page's width
keys("q"); time.sleep(2)
keys("HOME=" + docs + " " + rk + " " + docs + "/pic.jpg\n"); time.sleep(5)    # a picture, fit
keys("+"); time.sleep(4)
keys("-"); time.sleep(4)
keys("q"); time.sleep(2)
exit_app()
