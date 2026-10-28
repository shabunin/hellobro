# bookokrat (github.com/bugzmanov/bookokrat), a reader of EPUB, PDF and
# DjVu, in the Kobo's sh (the PC's, here): an EPUB with a picture (shown
# by Unicode placeholders), then a PDF (kitty's images by id).
# BOOKOKRAT: the command that runs a build, with no TMUX, TERM_PROGRAM
# or the like from the PC's environment (bookokrat goes by them): an
# armv7 one as `podman run --rm -it -e TERM=xterm-256color -v DOCS:/docs
# -v OUT:/out IMAGE /out/bookokrat` (an Alpine with fontconfig and
# libgcc); the books in its /docs: pic.epub, test.pdf. It finds kitty's
# graphics by asking (and no shared memory: the images go in the stream).
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

cmd = os.environ["BOOKOKRAT"]
start()
tap(tab_x(0), tabs_y); time.sleep(2)                 # workspace 1: the launcher
keys("2"); key(ENTER); time.sleep(3)                 # the Kobo's sh
keys(cmd + " /docs/pic.epub\n"); time.sleep(25)      # qemu: slow to start
for _ in range(4): keys("j"); time.sleep(2)          # down the text
key(32, ctrl=True); time.sleep(5)                    # Ctrl+D: half a page
key(22, ctrl=True); time.sleep(5)                    # Ctrl+U: back up
key(ESC); time.sleep(2)                              # the note it starts with
keys("q"); time.sleep(4)
keys(cmd + " /docs/test.pdf\n"); time.sleep(30)
keys("j"); time.sleep(6)
key(ESC); time.sleep(2)
keys("q"); time.sleep(4)
keys("echo back\n"); time.sleep(2)
exit_app()
