# Grab and kitty's keyboard protocol: a raw cat -v in the Kobo's sh asks
# for every key the kitty way (CSI > 11 u), then Mod+F: without grab the
# window goes fullscreen (only Super reaches cat); grabbed, Mod+F reaches
# cat whole and the bar says [grab]; Mod+Shift+G gives the keys back.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

KEY_G = 34
start()
tap(tab_x(0), tabs_y); time.sleep(2)                 # workspace 1: the launcher
keys("2"); time.sleep(3)                             # the Kobo's sh
keys("stty raw -echo; printf '\\033[>11u'; cat -v\n"); time.sleep(2)
fullscreen(); time.sleep(3)                          # hellobro's: fullscreen
fullscreen(); time.sleep(3)
mod(KEY_G, shift=True); time.sleep(3)                # grab
fullscreen(); time.sleep(3)                          # cat's now
keys("hi"); time.sleep(3)
mod(KEY_G, shift=True); time.sleep(3)                # let go
fullscreen(); time.sleep(3)                          # hellobro's again
fullscreen(); time.sleep(2)
exit_app()
