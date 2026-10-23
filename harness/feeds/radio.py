# The radios' windows, the stand-ins (HELLOBRO_FAKE_RADIO, which run.sh
# sets), Wi-Fi above Bluetooth (the launcher closed first): a network
# with a wrong password and then the right one, a Bluetooth scan, pairing
# with a code to confirm, Wi-Fi off and on; then the switcher (Meta+D,
# typed into) opening Battery, fullscreen, the switcher over it; then
# sleep (Shift+Meta+S, pretended: HELLOBRO_FAKE_SLEEP) and waking.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

KQ, KV, KD, KS = 16, 47, 32, 31

start()
mod(KQ, shift=True); time.sleep(2)                    # the launcher closed
gui("wi-fi"); mod(KV); gui("bluetooth")               # Bluetooth under Wi-Fi
mod(UP); time.sleep(3)                                # Wi-Fi: status, then the scan
key(DOWN); time.sleep(1)                              # BroLab
key(ENTER); time.sleep(1)                             # its password, asked
keys("wrongpass"); key(TAB); time.sleep(1)            # typed, shown
key(ENTER); time.sleep(4)                             # turned down
key(ENTER); time.sleep(1)
keys("bropassword"); key(ENTER); time.sleep(5)        # joined
mod(DOWN); time.sleep(1)                              # Bluetooth
keys("s"); time.sleep(5)                              # a scan: JBL turns up
key(DOWN); key(DOWN); time.sleep(1)
key(ENTER); time.sleep(3)                             # pair: confirm the code
keys("y"); time.sleep(4)                              # paired, connected
key(DOWN); key(ENTER); time.sleep(3)                  # the FiiO (paired, below the connected ones): connect
mod(UP); keys("o"); time.sleep(4)                     # Wi-Fi: off
keys("o"); time.sleep(6)                              # and on
mod(KD); time.sleep(2)                                # the switcher
keys("battery"); time.sleep(1)
key(ENTER); time.sleep(3)                             # Battery, beside Wi-Fi
fullscreen(); time.sleep(2)
mod(KD); time.sleep(2)                                # the switcher, fullscreen
key(ESC); time.sleep(2)
fullscreen(); time.sleep(3)                           # out of fullscreen
mod(KS, shift=True); time.sleep(1)                    # sleep: the screen, the radios off
keys("o"); time.sleep(8)                              # ignored while asleep; awake, radios back on
exit_app()
