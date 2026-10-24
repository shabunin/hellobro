#!/usr/bin/env python3
# Shows images the kitty way (graphics protocol), for the gfx feed: a PNG
# at its own size, in chunks; the same asked into 30×6 cells; a zlib'd
# RGB one; and the cell size, as programs ask for it.
import base64, math, struct, sys, zlib, os, termios, tty, select

def png(w, h, px):
    raw = b''.join(b'\0' + bytes(px[y * w * 3:(y + 1) * w * 3]) for y in range(h))
    chunk = lambda t, d: struct.pack('>I', len(d)) + t + d + struct.pack('>I', zlib.crc32(t + d))
    return b'\x89PNG\r\n\x1a\n' + chunk(b'IHDR', struct.pack('>IIBBBBB', w, h, 8, 2, 0, 0, 0)) + \
        chunk(b'IDAT', zlib.compress(raw)) + chunk(b'IEND', b'')

def picture(w, h):
    px = []
    for y in range(h):
        for x in range(w):
            d = math.hypot(x - w / 2, y - h / 2) / (min(w, h) / 2)
            g = int(255 * x / w) if d > 1 else int(255 * (1 - d) * (0.5 + 0.5 * math.sin(y / 6)))
            px += [g, g // 2, 255 - g]
    return px

def send(ctrl, data):
    b = base64.standard_b64encode(data)
    while b:
        part, b = b[:4096], b[4096:]
        sys.stdout.write('\x1b_G%s;%s\x1b\\' % ((ctrl + ',m=%d' % bool(b)) if ctrl else 'm=%d' % bool(b), part.decode()))
        ctrl = ''
    sys.stdout.flush()

def ask(q):
    fd = sys.stdin.fileno()
    old = termios.tcgetattr(fd)
    tty.setraw(fd)
    try:
        os.write(1, q.encode())
        out = b''
        while select.select([fd], [], [], 1)[0]:
            out += os.read(fd, 100)
            if out.endswith(b't'): break
        return out
    finally:
        termios.tcsetattr(fd, termios.TCSADRAIN, old)

print('cell size:', repr(ask('\x1b[16t')), 'area:', repr(ask('\x1b[14t')))
w, h = 240, 160
px = picture(w, h)
print('natural size:')
send('a=T,f=100,q=2', png(w, h, px))
print()
print('30x6 cells:')
send('a=T,f=100,c=30,r=6,q=2', png(w, h, px))
print()
print('zlib rgb, 12 cells wide:')
send('a=T,f=24,s=%d,v=%d,o=z,c=12,q=2' % (w, h), zlib.compress(bytes(px)))
print()
print('done')
