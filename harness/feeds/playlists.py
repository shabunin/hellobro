# The Music tab's library (portrait). By keyboard: Folders, a folder played
# whole and shuffled, the next track, everything added to a new playlist,
# then Playlists: it opened, an entry moved and one taken off, repeat
# round, played from a track. By touch: the Chill folder opened and played
# shuffled with the bar, the track added to the playlist with the chip,
# shuffle off, the playlist played in order, its last entry moved up.
# Wants MUSIC=a folder with Music/<folders>/<albums>.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

KEY_A, KEY_R, KEY_N, KEY_X, HOME, END = 30, 19, 49, 45, 102, 107
seg = lambda i: (165 + 272 * i, 774)                  # Albums Folders Playlists
row = lambda i: (700, 840 + 52 * i)
bar = lambda i: ((110, 272, 434, 569, 677)[i], 1757)  # play shuffle +playlist|remove ↑ ↓
chip = lambda i: ((549, 692, 834)[i], 559)            # shuffle, repeat, + playlist

start()
tap(tab_x(2), tabs_y); time.sleep(4)                  # Music tab: the albums
key(TAB); time.sleep(2)                               # Folders: Chill, Heroes
key(DOWN); key(ENTER); time.sleep(2)                  # into Heroes: HoMM2, HoMM3
key(BACKSPACE); time.sleep(2)                         # back up, on Heroes
key(ENTER, shift=True); time.sleep(6)                 # Heroes played whole
key(KEY_R); time.sleep(2)                             # shuffled
key(KEY_N); time.sleep(5)                             # the next one
key(KEY_A, shift=True); time.sleep(2)                 # add what's open (the top of Folders: all music)…
keys("Road trip"); time.sleep(1)
key(ENTER); time.sleep(2)                             # …to a new playlist
key(TAB); time.sleep(2)                               # Playlists
key(ENTER); time.sleep(2)                             # open Road trip
key(HOME); key(DOWN, shift=True); time.sleep(2)       # the first one down a place
key(KEY_X); time.sleep(2)                             # and off
key(KEY_T); time.sleep(1)                             # repeat all
key(ENTER); time.sleep(6)                             # play the playlist from the one selected

tap(*seg(1)); time.sleep(2)                           # Folders, by touch
tap(*row(0)); time.sleep(2)                           # into Chill: Night
tap(*bar(1)); time.sleep(6)                           # Chill, shuffled
tap(*chip(2)); time.sleep(2)                          # the track to a playlist…
keys("road"); key(ENTER); time.sleep(2)               # …Road trip, found by typing
tap(*chip(0)); time.sleep(2)                          # shuffle off
tap(*seg(2)); time.sleep(2)                           # Playlists: still in Road trip
tap(*bar(0)); time.sleep(5)                           # played in order
key(END); tap(*bar(3)); time.sleep(2)                 # the last one up
tap(1334, 705); time.sleep(2)                         # back: the playlists
exit_app()
