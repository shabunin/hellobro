# fancy-cat (github.com/freref/fancy-cat), a PDF viewer on kitty's
# graphics, in the Kobo's sh (the PC's, here). FANCY_CAT: an armv7 build
# (it runs under qemu-arm); READ_DOCS: a directory with test.pdf.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

fc, docs = os.environ["FANCY_CAT"], os.environ["READ_DOCS"]
start()
tap(tab_x(0), tabs_y); time.sleep(2)                 # workspace 1: the launcher
keys("2"); key(ENTER); time.sleep(3)                 # the Kobo's sh
keys("HOME=" + docs + " " + fc + " " + docs + "/test.pdf\n"); time.sleep(10)
keys("n"); time.sleep(5)                             # page 2
keys("n"); time.sleep(5)                             # page 3
keys("p"); time.sleep(5)                             # back to 2
keys("w"); time.sleep(5)                             # the page's width
keys("j"); time.sleep(4)                             # down
keys("ii"); time.sleep(5)                            # zoomed in
key(46, ctrl=True); time.sleep(3)                    # Ctrl+C: quit
keys("echo back\n"); time.sleep(2)
exit_app()
