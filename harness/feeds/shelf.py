# All Books (bookwin.go, library.go): the shelf fullscreen, its Recent
# tab; All; Folders: Fiction, in it SF, Backspace up again, the book in
# Fiction opened. ☰: its contents, with All Books at the top left and
# back to the book at the top right; All Books: the shelf, back to the
# book at its top right now; back to the book. s: the style panel, the
# same header; back to the book.
# HELLOBRO_BOOKS: a directory with Docs/a.pdf, Fiction/pooh.epub,
# Fiction/SF/space.epub and manual.pdf.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

PAD = 16
TOP = 40                                               # the header's middle, fullscreen
ALL, BACK = (PAD + min(230, (W - 2 * PAD) // 5) // 2, TOP), (W - PAD - min(300, (W - 2 * PAD) // 4) // 2, TOP)
L, R = PAD + min(230, (W - 2 * PAD) // 5) + 16, W - PAD - min(300, (W - 2 * PAD) // 4) - 16
def tab(k, n=3): return (L + (2 * k + 1) * (R - L) // (2 * n), TOP)

start()
gui("books"); time.sleep(4)
fullscreen(); time.sleep(4)                            # Recent
tap(*tab(1)); time.sleep(3)                            # All
tap(*tab(2)); time.sleep(3)                            # Folders
key(108); time.sleep(1); key(ENTER); time.sleep(3)     # Fiction
key(108); time.sleep(1); key(ENTER); time.sleep(3)     # SF
key(14); time.sleep(3)                                 # Backspace: up, Fiction
key(108); time.sleep(1); key(108); time.sleep(1)
key(ENTER); time.sleep(8)                              # the book
tap(*barbtn(0)); time.sleep(4)                         # ☰: its contents
tap(*ALL); time.sleep(5)                               # All Books
tap(*BACK); time.sleep(6)                              # back to the book
key(31); time.sleep(4)                                 # s: the style panel
tap(*BACK); time.sleep(4)                              # back to the book
exit_app()
