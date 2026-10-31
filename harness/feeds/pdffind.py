# ":/" in a PDF (HELLOBRO_BOOKS: a.pdf first, Little CMS's tutorial):
# :/portability goes to the first page with it (8), :/ to the next; a
# word not in it says so. The log says "book: "portability" found true".
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

def ex(s, wait=4):
    keys(":"); time.sleep(1.5); keys(s); time.sleep(2); key(ENTER); time.sleep(wait)

start(); time.sleep(3)
gui("books"); time.sleep(4)
key(ENTER); time.sleep(8)                              # the PDF
ex("/portability", 10)
ex("/", 10)
ex("/zzqx", 10)
exit_app()
