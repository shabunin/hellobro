# A PDF's pages, small (bookpages.go): the first book, a PDF, in
# fullscreen; o: the pages, drawn one by one; → → Enter: the third read.
# The bar's ☰: the contents; the Pages tab; a finger swiped up: the
# next screenful; a tap on its second page reads it.
# HELLOBRO_BOOKS: a directory with a PDF first by name.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
gui("books"); time.sleep(4)
key(ENTER); time.sleep(6)
fullscreen(); time.sleep(5)
key(24); time.sleep(12)                            # o: the pages, small
key(RIGHT); time.sleep(2); key(RIGHT); time.sleep(2)
key(ENTER); time.sleep(6)                          # page 3
tap(*barbtn(0)); time.sleep(3)                    # ☰: the contents
tap(170 + (W - 450) * 5 // 6, 40); time.sleep(10)  # the Pages tab
stroke([(W // 2, H * 3 // 4 - i * 40) for i in range(12)]); time.sleep(10)  # swiped up: the next screenful
tap(W // 2, 250); time.sleep(6)                    # its first row's middle page
fullscreen(); time.sleep(3)
exit_app()
