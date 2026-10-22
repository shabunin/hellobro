# Five minutes of typing and menu hopping in a session (TIMEOUT=400).
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
import random
random.seed(1)
tap(tab_x(0), tabs_y); time.sleep(2)                 # SSH tab
keys("1"); key(ENTER); time.sleep(5)                             # connect
end = time.time() + 330
n = 0
while time.time() < end:                             # rounds: chat typing, then menu navigation
    key(DOWN); key(DOWN); key(ENTER); time.sleep(0.5)  # Chat (from Dashboard)
    for i in range(60):
        key(random.randint(16, 25)); time.sleep(0.05)  # letters, ~5 per second with key()'s pause
        n += 1
    key(ENTER); time.sleep(0.3)
    key(ESC); key(ESC)
    for i in range(10):
        key(DOWN); key(ENTER); time.sleep(0.2); key(ESC)
    key(ESC); keys("1"); time.sleep(0.3)             # back to Dashboard
print("typed", n, file=sys.stderr)
keys("q"); time.sleep(3)
exit_app()
