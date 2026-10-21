# Records what a terminal program prints, to replay it into the Terminal
# tab's drawing code (termrender_test.go):
#   harness/record.py OUT COLS ROWS SECONDS cmd...
# runs cmd in a pty of that size for SECONDS and saves its output to OUT.
# KEYS (with \x escapes) is typed one key per 0.3 s, starting after 3 s:
#   KEYS='\x04\x04' harness/record.py hx.raw 113 33 7 hx main.go
import os, pty, sys, time, fcntl, termios, struct, select, signal
out, cols, rows, secs, cmd = sys.argv[1], int(sys.argv[2]), int(sys.argv[3]), float(sys.argv[4]), sys.argv[5:]
keys = os.environ.get('KEYS', '').encode().decode('unicode_escape').encode()  # typed one per 0.3 s after 3 s
pid, fd = pty.fork()
if pid == 0:
    os.environ['TERM'] = 'xterm-256color'
    os.execvp(cmd[0], cmd)
fcntl.ioctl(fd, termios.TIOCSWINSZ, struct.pack('HHHH', rows, cols, 0, 0))
os.kill(pid, signal.SIGWINCH)
data = b''
start = time.time(); end = start + secs; sent = 0
while time.time() < end:
    if sent < len(keys) and time.time() > start + 3 + sent * 0.3:
        os.write(fd, keys[sent:sent+1]); sent += 1
    r, _, _ = select.select([fd], [], [], 0.1)
    if r:
        try: data += os.read(fd, 65536)
        except OSError: break
open(out, 'wb').write(data)
os.kill(pid, signal.SIGTERM)
