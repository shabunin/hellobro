# The bar's +: a new workspace, the first number not taken, gone to; a
# window opened in it (Mod+Enter), then + again: a third.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

PLUS = (int(os.environ.get('PLUS_X', 117)), 23)

start(); time.sleep(3)
tap(*PLUS); time.sleep(3)                     # workspace 2
mod(ENTER); time.sleep(3)                     # a window in it
tap(int(os.environ.get('PLUS2_X', 210)), 23); time.sleep(3)  # the + after 1 and 2: workspace 3
exit_app()
