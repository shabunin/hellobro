# All Books: rename. The shelf (HELLOBRO_BOOKS: a.pdf, pooh.epub);
# Folders off, All: the first book; F2, its name typed over, Enter. The
# second by its row's rename, then cancel. The book opened: its title.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start(); time.sleep(3)
gui("books"); time.sleep(5)
key(TAB); time.sleep(3)                                # All
key(60); time.sleep(3)                                 # F2
for _ in range(30): key(14)
keys("Notes on a PDF"); time.sleep(2); key(ENTER); time.sleep(4)
key(108); time.sleep(1); key(60); time.sleep(3)        # the next: F2
keys(" bis"); time.sleep(2); key(ESC); time.sleep(3)   # not
key(103); time.sleep(1); key(ENTER); time.sleep(8)     # the renamed one, open
exit_app()
