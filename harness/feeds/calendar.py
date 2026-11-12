# The calendar: its views (keys 1-5), synced first with the stand-in
# CalDAV server in calendar.conf (APP_EXTRA), a task typed in the tasks
# and ticked, a new event.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start(); time.sleep(3)
gui("calendar"); fullscreen(); time.sleep(4)   # the week, synced
keys("3"); time.sleep(2)                       # the month
keys("4"); time.sleep(2)                       # the agenda
keys("5"); time.sleep(2)                       # the tasks
tap(450, 187); time.sleep(1)                   # a new one, typed
keys("water the plants"); key(ENTER); time.sleep(2)
key(ESC); time.sleep(1)
tap(42, 326); time.sleep(2)                    # buy milk: done
keys("1"); time.sleep(2)                       # the day
keys("2"); time.sleep(1)
keys("n"); time.sleep(2)                       # a new event
keys("lunch with Mira"); time.sleep(2)
key(ENTER); time.sleep(8)                      # saved, and sent
exit_app()
