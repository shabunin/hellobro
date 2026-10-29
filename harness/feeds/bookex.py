# A book's ":" commands (HELLOBRO_BOOKS: pooh.epub alone): ":" and "pa",
# the commands it could be; :3 the chapter's third page; :/honey looks
# for it; :/ the next; :size 30; :ch 2; :zz, not a command; :help, the
# commands in a menu: "size" Enter puts :size on the line, 28; :q closes.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

def ex(s, wait=4):
    keys(":"); time.sleep(1.5); keys(s); time.sleep(2); key(ENTER); time.sleep(wait)

start(); time.sleep(3)
gui("books"); time.sleep(5)
key(ENTER); time.sleep(8)                              # the book
keys(":pa"); time.sleep(3); key(BACKSPACE); key(BACKSPACE); key(BACKSPACE); time.sleep(2)
ex("3")
ex("/honey", 8)
ex("/", 8)
ex("size 30", 6)
ex("ch 2", 6)
ex("zz", 3)
ex("help", 4)                                          # the commands, in a menu
keys("size"); time.sleep(2); key(ENTER); time.sleep(3) # :size on the line
keys("28"); time.sleep(1); key(ENTER); time.sleep(5)
ex("q", 4)
exit_app()
