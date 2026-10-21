# The Settings tab's radios, the stand-ins (HELLOBRO_FAKE_RADIO, which
# run.sh sets): a network with a wrong password and then the right one,
# a Bluetooth scan, pairing with a code to confirm, Wi-Fi off and on; then
# the switcher (Ctrl+Shift+Tab, typed into) and Ctrl+Alt+N tabs; then
# sleep (Ctrl+Alt+S, pretended: HELLOBRO_FAKE_SLEEP) and waking.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

K1, K4, KS = 2, 5, 31

start()
key(K4, ctrl=True, alt=True); time.sleep(5)           # the tab: status, then the scan
key(DOWN); time.sleep(1)                              # BroLab
key(ENTER); time.sleep(1)                             # its password, asked
keys("wrongpass"); key(TAB); time.sleep(1)            # typed, shown
key(ENTER); time.sleep(4)                             # turned down
key(ENTER); time.sleep(1)
keys("bropassword"); key(ENTER); time.sleep(5)        # joined
key(TAB); time.sleep(1)                               # the Bluetooth panel
keys("s"); time.sleep(5)                              # a scan: JBL turns up
key(DOWN); key(DOWN); time.sleep(1)
key(ENTER); time.sleep(3)                             # pair: confirm the code
keys("y"); time.sleep(4)                              # paired, connected
key(DOWN); key(ENTER); time.sleep(3)                  # the FiiO (paired, below the connected ones): connect
key(TAB); key(TAB); keys("o"); time.sleep(4)          # past the settings to Wi-Fi: off
keys("o"); time.sleep(6)                              # and on
key(TAB, ctrl=True, shift=True); time.sleep(2)        # the switcher
keys("mus"); time.sleep(1)
key(ENTER); time.sleep(3)                             # the Music tab
key(K1, ctrl=True, alt=True); time.sleep(3)           # the SSH tab
key(F11); time.sleep(2)
key(TAB, ctrl=True, shift=True); time.sleep(2)        # the switcher, fullscreen
key(ESC); time.sleep(2)
key(K4, ctrl=True, alt=True); time.sleep(3)           # out of fullscreen, to Settings
key(KS, ctrl=True, alt=True); time.sleep(1)           # sleep: the screen, the radios off
keys("o"); time.sleep(8)                              # ignored while asleep; awake, radios back on
exit_app()
