import subprocess, sys
import numpy as np


def load(p):
    raw = subprocess.check_output(['ffmpeg', '-v', 'error', '-i', p, '-ac', '1', '-ar', '16000', '-f', 's16le', '-'])
    return np.frombuffer(raw, np.int16).astype(np.float32) / 32768


for p in sys.argv[1:]:
    x = load(p)
    f0 = []
    for i in range(0, len(x) - 1024, 320):
        fr = x[i:i + 1024]
        if np.sqrt((fr ** 2).mean()) < 0.02:
            continue
        fr = fr - fr.mean()
        ac = np.correlate(fr, fr, 'full')[1023:]
        lo, hi = 16000 // 300, 16000 // 60
        k = lo + int(np.argmax(ac[lo:hi]))
        if ac[k] > 0.4 * ac[0]:
            f0.append(16000 / k)
    print(p.split('/')[-1], 'median F0', round(float(np.median(f0)), 1), 'Hz', 'n', len(f0))
