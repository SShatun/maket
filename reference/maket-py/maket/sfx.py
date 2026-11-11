"""Sound design derived from scene events. Every sound is synthesised, no samples."""
import wave
import numpy as np
from . import tokens as T

SR = 48000


class Mixer:
    def __init__(self, dur, seed=7):
        self.out = np.zeros(int(SR * dur)); self.rng = np.random.default_rng(seed)

    def add(self, t0, sig, gain):
        i = int(t0 * SR)
        if i >= len(self.out) or i < 0: return
        j = min(len(self.out), i + len(sig)); self.out[i:j] += sig[:j - i] * gain

    def save(self, path):
        d = int(T.SFX['room_delay'] * SR); wet = np.zeros_like(self.out); wet[d:] = self.out[:-d] * T.SFX['room_mix']
        m = self.out + wet; m = m / max(1e-9, np.abs(m).max()) * T.SFX['peak']
        pcm = (m * 32767).astype(np.int16); st = np.stack([pcm, pcm], axis=1)
        with wave.open(path, 'wb') as w:
            w.setnchannels(2); w.setsampwidth(2); w.setframerate(SR); w.writeframes(st.tobytes())


def _env(n, a, d):
    t = np.arange(n) / SR; return np.minimum(1, t / a) * np.exp(-t / d)
def _tone(f0, f1, dur, d, a=0.003):
    n = int(dur * SR); f = np.linspace(f0, f1, n); return np.sin(2 * np.pi * np.cumsum(f) / SR) * _env(n, a, d)
def _pad(x, n): y = np.zeros(n); y[:min(n, len(x))] = x[:n]; return y
def _lp(x, k):
    y = np.empty_like(x); acc = 0.0
    for i, v in enumerate(x): acc += k * (v - acc); y[i] = acc
    return y

def whoosh(m, dur):
    n = int(dur * SR); t = np.linspace(0, 1, n); return _lp(m.rng.standard_normal(n), .06) * np.sin(np.pi * t) ** 2 * 1.6
def land(m, pitch=1.0):
    n = int(.12 * SR)
    return (_tone(900 * pitch, 520 * pitch, .12, .035) + .5 * _pad(_tone(1800 * pitch, 1500 * pitch, .05, .012), n)
            + .3 * _pad(_lp(m.rng.standard_normal(int(.03 * SR)), .4) * _env(int(.03 * SR), .001, .008), n))
def rise(m, i): return _tone(300 + i * 40, 420 + i * 40, .18, .06) * .5
def thud(m):
    n = int(.45 * SR)
    return _tone(140, 55, .45, .16) * 1.3 + _pad(_lp(m.rng.standard_normal(int(.2 * SR)), .08) * _env(int(.2 * SR), .002, .05) * 1.2, n)
def buzz(m):
    n = int(.22 * SR); t = np.arange(n) / SR
    sq = np.sign(np.sin(2 * np.pi * 110 * t)) * .25 + np.sign(np.sin(2 * np.pi * 116 * t)) * .25
    return _lp(sq, .15) * _env(n, .005, .12)
def pop(m): return _tone(420, 980, .16, .06) * .9
def chime(m):
    n = int(1.6 * SR)
    return (_tone(660, 660, 1.6, .5, .01) + .6 * _tone(990, 990, 1.6, .4, .01) + .3 * _pad(_tone(1320, 1320, 1.2, .3, .01), n)) * .5


def build(scene, path):
    m = Mixer(scene.duration); g = T.SFX
    for i, eid in enumerate(scene.order):
        m.add(scene.ents[eid]['rise_t'] + .18, rise(m, i), g['rise'])
    land_n = 0
    for ev in scene.events:
        if ev.get('sound') is False: continue
        if ev['do'] == 'send':
            end = ev['t'] + ev['dur']
            m.add(ev['t'], whoosh(m, ev['dur']), g['whoosh'] * (.6 if scene._ent_of(ev['from']) == scene._ent_of(ev['to']) else 1))
            if ev.get('consume'): m.add(end - .02, pop(m), g['pop'])
            elif not ev.get('waiting'):
                m.add(end - .01, land(m, 1 + .06 * (land_n % 4)), g['land']); land_n += 1
        elif ev['do'] == 'block':
            m.add(ev['t'] - .02, thud(m), g['thud']); m.add(ev['t'] + .1, buzz(m), g['buzz']); m.add(ev['t'] + .4, buzz(m), g['buzz'] * .8)
        elif ev['do'] == 'flash':
            m.add(ev['t'], pop(m), g['pop'])
        elif ev['do'] == 'sfx':
            fn = {'whoosh': lambda: whoosh(m, ev.get('dur', .5)), 'land': lambda: land(m), 'thud': lambda: thud(m),
                  'buzz': lambda: buzz(m), 'pop': lambda: pop(m), 'chime': lambda: chime(m)}[ev['name']]
            m.add(ev['t'], fn(), g.get(ev['name'], .5) * ev.get('gain', 1.0))
        elif ev['do'] == 'focus':
            m.add(ev['t'], chime(m), g['chime'])
    m.save(path)
