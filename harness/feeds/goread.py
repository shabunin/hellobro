# goread (github.com/Ray-D-Song/goread, with the Kobo's patch: the
# book's bold and italics; o shows a picture by read-kitty) in the
# Kobo's sh (the PC's, here). GOREAD: a directory with PC builds of
# goread and of hellobro named read-kitty; GOREAD_BOOK: an EPUB with
# italics and pictures in its chapters.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

bin, book = os.environ["GOREAD"], os.environ["GOREAD_BOOK"]
home = os.path.join(os.environ.get("TMPDIR", "/tmp"), "goread-home")
start()
tap(tab_x(0), tabs_y); time.sleep(2)                 # workspace 1: the launcher
keys("2"); key(ENTER); time.sleep(3)                 # the Kobo's sh
keys("mkdir -p " + home + "; HOME=" + home + " PATH=" + bin + ":$PATH TERM=xterm-256color goread " + book + "\n"); time.sleep(6)
keys("n"); time.sleep(3)
keys("n"); time.sleep(3)
keys("n"); time.sleep(4)                             # a chapter, italics in it
keys("o"); time.sleep(2)                             # which picture
keys("0\n"); time.sleep(8)                           # read-kitty shows it
keys("q"); time.sleep(4)                             # back in goread
keys("q"); time.sleep(3)
keys("echo back\n"); time.sleep(2)
exit_app()
