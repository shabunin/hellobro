#!/usr/bin/python3 -I
# Stand-in for fbink: "-e" reports an Elipsa-like state, "-g file=-,x=,y=" pastes
# a PGM from stdin onto $FAKE_SCREEN_DIR/screen.raw, snapshotting big updates.
import os, sys, struct, zlib
d = os.environ['FAKE_SCREEN_DIR']
rota = int(os.environ.get('FBINK_FORCE_ROTA', os.environ.get('FAKE_GYRO', '0')))
W, H = (1872, 1404) if rota in (1, 3) else (1404, 1872)
args = sys.argv[1:]
with open(os.path.join(d, 'calls.txt'), 'a') as f:
    f.write(' '.join(args) + (f'  [rota={rota}]' if 'FBINK_FORCE_ROTA' in os.environ else '  [gyro]') + '\n')
if '-e' in args:
    print(f"FBINK_VERSION='fake';viewWidth={W};viewHeight={H};screenWidth={W};screenHeight={H};"
          f"viewHoriOrigin=0;viewVertOrigin=0;deviceCodename='europa';rotationMap='{{ 0, 1, 2, 3 }}';"
          f"touchSwapAxes=1;touchMirrorX=1;touchMirrorY=0;currentRota={rota};", end='')
    sys.exit(0)
g = args[args.index('-g') + 1]
opts = dict(kv.split('=') for kv in g.split(','))
x, y = int(opts['x']), int(opts['y'])
data = sys.stdin.buffer.read()
parts = data.split(b'\n', 3)
assert parts[0] == b'P5', parts[0]
w, h = map(int, parts[1].split()); pix = parts[3]
path = os.path.join(d, 'screen.raw')
key = os.path.join(d, 'dims')
if not os.path.exists(path) or open(key).read() != f'{W}x{H}':
    open(path, 'wb').write(b'\x80' * (W * H)); open(key, 'w').write(f'{W}x{H}')
scr = bytearray(open(path, 'rb').read())
def png(name):
    raw = b''.join(b'\x00' + bytes(scr[r*W:(r+1)*W]) for r in range(H))
    chunk = lambda t, b: struct.pack('>I', len(b)) + t + b + struct.pack('>I', zlib.crc32(t + b) & 0xffffffff)
    open(os.path.join(d, name), 'wb').write(b'\x89PNG\r\n\x1a\n' + chunk(b'IHDR', struct.pack('>IIBBBBB', W, H, 8, 0, 0, 0, 0))
        + chunk(b'IDAT', zlib.compress(raw, 6)) + chunk(b'IEND', b''))
n = len(os.listdir(d))
big = w * h >= int(os.environ.get('FAKE_SNAP_AREA', W * H // 5))
if big:
    png(f'snap-{n:04d}-a-before.png')
for r in range(h):
    scr[(y + r) * W + x:(y + r) * W + x + w] = pix[r * w:(r + 1) * w]
open(path, 'wb').write(scr)
if big:
    png(f'snap-{n:04d}-b-after.png')
if big:
    png("final.png")
