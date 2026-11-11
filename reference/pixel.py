import json, math, os, subprocess, sys, wave
import numpy as np
from PIL import Image, ImageDraw, ImageFont

HERE = os.path.dirname(os.path.abspath(__file__))
FONTS = os.path.join(HERE, 'fonts')
SPR = os.path.join(HERE, 'sprites')
MONO = os.path.join(HERE, '..', 'maket-src', 'maket', 'maket', 'fonts', 'jbm500.ttf')
W, H, FPS = 1080, 1920, 30

BG = (29, 43, 83)
DOT = (44, 60, 108)
PANEL = (16, 24, 50)
WHITE = (255, 241, 232)
YEL = (255, 236, 39)
RED = (255, 0, 77)
GRN = (0, 228, 54)
DGRN = (0, 135, 81)
ORA = (255, 163, 0)
LAV = (131, 118, 156)
GREY = (194, 195, 199)
DIM = (120, 132, 170)
ROLE = {'cardboard': (232, 170, 110), 'external': (255, 255, 255), 'producer': DGRN, 'storage': (41, 173, 255), 'lock': ORA,
        'error': RED, 'ok': GRN}
SPRITES = json.load(open(os.path.join(HERE, '..', 'artifact-files', 'e7a5d8b6-d08b-492d-8d17-7431b90e0ef5', 'project',
                                      'assets', 'Objects-Pixel', 'sprites.json')))['sprites']
SCALE = 0.75

_fc = {}


def font(name, size):
    k = (name, size)
    if k not in _fc:
        path = {'pix': os.path.join(FONTS, 'PixelifySans.ttf'), 'ps2p': os.path.join(FONTS, 'PressStart2P-Regular.ttf'),
                'mono': MONO, 'inter': os.path.join(os.path.dirname(MONO), 'Inter-Bold.otf')}[name]
        _fc[k] = ImageFont.truetype(path, size)
    return _fc[k]


def runs(text, size):
    if os.environ.get('PIXFONT', 'ps2p') == 'ps2p':
        f = font('ps2p', int(size * 0.62))
        return [(ch, f, int(size * 0.2)) for ch in text]
    out = []
    for ch in text:
        if ch == 'О':
            out.append(('O', font('pix', size), 0))
        elif ch == 'П':
            s = int(size * 0.62)
            out.append((ch, font('ps2p', s), int(size * 0.2)))
        else:
            out.append((ch, font('pix', size), 0))
    return out


def pix_width(text, size):
    return sum(f.getlength(c) for c, f, _ in runs(text, size))


def pix_text(d, cx, cy, text, size, fill):
    x = cx - pix_width(text, size) / 2
    base = font('pix', size)
    asc = base.getmetrics()[0]
    y0 = cy - asc * 0.62
    for c, f, dy in runs(text, size):
        if f is base or f.path.endswith('PixelifySans.ttf'):
            d.text((x, y0), c, font=f, fill=fill)
        else:
            d.text((x, y0 + dy + 2), c, font=f, fill=fill)
        x += f.getlength(c)


def frame_box(d, x0, y0, x1, y1, border, fill, bw=6):
    d.rectangle((x0, y0, x1, y1), fill=border)
    d.rectangle((x0 + bw, y0 + bw, x1 - bw, y1 - bw), fill=fill)


_sc = {}


def sprite(name, role):
    k = (name, role)
    if k not in _sc:
        im = Image.open(os.path.join(SPR, name + '.png')).convert('RGBA')
        a = np.asarray(im).astype(np.float32)
        r, g, b = a[..., 0], a[..., 1], a[..., 2]
        grey = (r == g) & (g == b) & (a[..., 3] > 0)
        col = np.array(ROLE[role], np.float32)
        for i in range(3):
            a[..., i] = np.where(grey, a[..., i] * col[i] / 255, a[..., i])
        im = Image.fromarray(a.clip(0, 255).astype(np.uint8), 'RGBA')
        im = im.resize((int(im.width * SCALE), int(im.height * SCALE)), Image.NEAREST)
        _sc[k] = im
    return _sc[k]


LIVE = os.path.join(HERE, '..', 'artifact-files', 'e7a5d8b6-d08b-492d-8d17-7431b90e0ef5', 'project', 'live')
LIVE_SPEC = json.load(open(os.path.join(LIVE, 'sprites.json')))['sprites']
_lc = {}


def tint(im, role):
    a = np.asarray(im.convert('RGBA')).astype(np.float32)
    r, g, b = a[..., 0], a[..., 1], a[..., 2]
    grey = (r == g) & (g == b) & (a[..., 3] > 0)
    col = np.array(ROLE[role], np.float32)
    for i in range(3):
        a[..., i] = np.where(grey, a[..., i] * col[i] / 255, a[..., i])
    return Image.fromarray(a.clip(0, 255).astype(np.uint8), 'RGBA')


def live_frame(name, states, t, role):
    spec = LIVE_SPEC[name]
    ts, st, hold = 0.0, spec['rest'], False
    for e in states:
        if t >= e[0]:
            ts, st, hold = e[0], e[1], len(e) > 2 and e[2] == 'hold'
    ss = spec['states'][st]
    i = int((t - ts) * ss['fps'])
    if ss['loop']:
        i %= ss['frames']
    elif i >= ss['frames']:
        if hold:
            i = ss['frames'] - 1
        else:
            st = spec['rest']
            rs = spec['states'][st]
            i = int((t - ts) * rs['fps']) % rs['frames']
    k = (name, st, i, role)
    if k not in _lc:
        im = Image.open(os.path.join(LIVE, name, f'{st}_{i}.png'))
        im = tint(im, role)
        _lc[k] = im.resize((im.width * 3 // 4, im.height * 3 // 4), Image.NEAREST)
    return _lc[k], spec


_bc = {}


def badge(name):
    if name not in _bc:
        im = Image.open(os.path.join(SPR, 'badge-' + name + '.png')).convert('RGBA')
        _bc[name] = im
    return _bc[name]


def clamp(x, a=0.0, b=1.0):
    return max(a, min(b, x))


def prog(t, a, b):
    return clamp((t - a) / (b - a)) if b > a else float(t >= a)


def ease(x):
    x = clamp(x)
    return x * x * (3 - 2 * x)


def step_pop(t, t0):
    k = t - t0
    if k < 0:
        return 0
    if k < 1 / FPS:
        return 0.55
    if k < 2 / FPS:
        return 1.12
    if k < 3 / FPS:
        return 0.96
    return 1.0


def at(series, t, default=None):
    v = default
    for ts, val in series:
        if t >= ts:
            v = val
    return v


def snap(v, g=3):
    return int(round(v / g) * g)


class Scene:
    def __init__(self, s):
        self.s = s
        self.ent = {e['id']: e for e in s['entities']}

    def anchor(self, eid):
        e = self.ent[eid]
        return e['at']

    def top(self, eid):
        e = self.ent[eid]
        sp = LIVE_SPEC[e['live']] if 'live' in e else SPRITES[e['sprite']]
        if 'live' in e:
            ax, ay = e['at']
            return ax + (sp['top'][0] - sp['anchor'][0]) * 0.75, ay + (sp['top'][1] - sp['anchor'][1]) * 0.75
        ax, ay = e['at']
        return ax + (sp['top'][0] - sp['anchor'][0]) * SCALE, ay + (sp['top'][1] - sp['anchor'][1]) * SCALE

    def frame(self, t):
        s = self.s
        im = Image.new('RGB', (W, H), BG)
        d = ImageDraw.Draw(im)
        for y in range(12, H, 36):
            for x in range(12, W, 36):
                d.rectangle((x, y, x + 3, y + 3), fill=DOT)

        sway = int(round(-8 * math.sin(t / s['duration'] * math.pi) / 3) * 3)
        stage = Image.new('RGBA', (W, H), (0, 0, 0, 0))
        sd = ImageDraw.Draw(stage)

        for e in s['entities']:
            k = step_pop(t, e['appear'])
            if k <= 0:
                continue
            role = at(e.get('roles', []), t, e['role'])
            if 'live' in e:
                sp, spec = live_frame(e['live'], e.get('states', []), t, role)
                foot = (spec['size'][1] - spec['anchor'][1]) * 3 // 4
            else:
                sp, foot = sprite(e['sprite'], role), 0
            ax, ay = e['at']
            
            shx = 0
            for ev in s['events']:
                if ev['do'] == 'shake' and ev['target'] == e['id'] and ev['t'] <= t < ev['t'] + 0.45:
                    shx = snap(9 * math.sin((t - ev['t']) * 60) * (1 - (t - ev['t']) / 0.45))
            w, h = sp.size
            if k != 1.0:
                sp2 = sp.resize((max(1, int(w * k)), max(1, int(h * k))), Image.NEAREST)
            else:
                sp2 = sp
            sd.rectangle((ax - w * 0.42, ay - 6, ax + w * 0.42, ay + 6), fill=(12, 18, 40, 255))
            stage.alpha_composite(sp2, (int(ax - sp2.width / 2 + shx), int(ay + foot - sp2.height)))
            if k == 1.0:
                pix_text(sd, ax, ay + 40, e['label'], 38, WHITE)
                st = at(e.get('status', []), t, '')
                if st:
                    sf = font('mono', 22)
                    sd.text((ax - sf.getlength(st) / 2, ay + 66), st, font=sf, fill=GREY)
                b = at(e.get('badges', []), t, None)
                if b:
                    bi = badge(b)
                    tx, ty = self.top(e['id'])
                    stage.alpha_composite(bi, (int(tx + w * 0.30 - bi.width / 2 + shx), int(ty - bi.height / 2 + 6)))

        for ev in s['events']:
            if ev['do'] != 'move' or t < ev['t']:
                continue
            if ev.get('until') is not None and t >= ev['until']:
                continue
            p = ease(prog(t, ev['t'], ev['t'] + ev['dur']))
            (x0, y0), (x1, y1) = ev['from'], ev['to']
            hop = -60 * math.sin(p * math.pi)
            x, y = snap(x0 + (x1 - x0) * p), snap(y0 + (y1 - y0) * p + hop)
            if ev.get('bounce') and t >= ev['t'] + ev['dur']:
                bk = prog(t, ev['t'] + ev['dur'], ev['t'] + ev['dur'] + 0.25)
                y = snap(y1 - 24 * math.sin(bk * math.pi))
            sd.rectangle((x - 30, y - 30, x + 30, y + 30), fill=(0, 0, 0, 255))
            sd.rectangle((x - 24, y - 24, x + 24, y + 24), fill=YEL + (255,))
            lab = ev.get('label', 'git')
            f = font('ps2p', 13 if len(lab) <= 3 else 10)
            sd.text((x - f.getlength(lab) / 2, y - 6), lab, font=f, fill=(0, 0, 0))

        for ev in s['events']:
            if ev['do'] != 'say' or not (ev['t'] <= t < ev['until']):
                continue
            k = step_pop(t, ev['t'])
            tx, ty = self.top(ev['who'])
            size = int(46 * k)
            if size < 8:
                continue
            tw = pix_width(ev['text'], size)
            bw, bh = tw + 56, int(92 * k)
            cx, by = tx + ev.get('dx', 0), ty - 40
            x0, y0 = snap(cx - bw / 2), snap(by - bh)
            frame_box(sd, x0, y0, x0 + snap(bw), by, WHITE + (255,), PANEL + (255,))
            for i in range(4):
                sd.rectangle((snap(cx) - 18 + i * 6, by - 6, snap(cx) - 6 + i * 6 - 0, by + 12 - i * 6), fill=WHITE + (255,))
            pix_text(sd, cx, y0 + bh / 2 + 2, ev['text'], size, YEL)

        dim = 0.0
        for ev in s['events']:
            if ev['do'] == 'guess':
                dim = max(dim, 0.65 * (1 - prog(t, ev['until'], ev['until'] + 0.2)) * (t >= ev['t']))
            if ev['do'] == 'answer' and t >= ev['t']:
                dim = max(dim, 0.65 * prog(t, ev['t'], ev['t'] + 0.3))
            if ev['do'] == 'number' and ev['t'] <= t < ev['until'] + 0.2:
                dim = max(dim, 0.7 * (1 - prog(t, ev['until'], ev['until'] + 0.2)))
        if dim > 0:
            a = np.asarray(stage).copy()
            a[..., 3] = (a[..., 3] * (1 - dim)).astype(np.uint8)
            stage = Image.fromarray(a, 'RGBA')
        im.paste(stage, (0, sway), stage)
        d = ImageDraw.Draw(im)

        for ev in s['events']:
            if ev['do'] == 'number' and ev['t'] <= t < ev['until']:
                k = step_pop(t, ev['t'])
                if k > 0:
                    hh = int(400 * k)
                    frame_box(d, 140, 620 - hh // 2, 940, 620 + hh // 2, WHITE, BG, bw=8)
                    f = font('ps2p', max(8, int(200 * k)))
                    d.text((540 - f.getlength(ev['text']) / 2, 460), ev['text'], font=f, fill=YEL)
                    if k == 1.0:
                        f2 = font('ps2p', 30)
                        d.text((540 - f2.getlength(ev['sub']) / 2, 720), ev['sub'], font=f2, fill=WHITE)
            if ev['do'] == 'guess' and ev['t'] <= t < ev['until'] + 0.2:
                self.guess_card(d, t, ev['text'], 560, struck=0)
            if ev['do'] == 'answer' and t >= ev['t']:
                sk = prog(t, ev['t'] + 0.25, ev['t'] + 0.55)
                self.guess_card(d, t, ev['guess'], 470, struck=sk)
                ct = ev.get('card_t', ev['t'] + 0.7)
                if t >= ct:
                    k = step_pop(t, ct)
                    y0 = 740
                    hh = int(230 * k)
                    frame_box(d, 140, y0, 940, y0 + hh, GRN, DGRN, bw=8)
                    if k == 1.0:
                        pix_text(d, 540, y0 + 52, 'на самом деле', 28, WHITE)
                        pix_text(d, 540, y0 + 140, ev['text'], ev.get('size', 70), WHITE)
                        bi = badge(ev.get('badge', 'question'))
                        im.paste(bi, (940 - bi.width // 2, y0 - bi.height // 2), bi)

        self.code(d, t)
        self.captions(d, t)
        p = clamp(t / s['duration'])
        frame_box(d, 70, 112, 1010, 136, WHITE, BG, bw=4)
        if p > 0:
            d.rectangle((76, 118, 76 + snap(928 * p, 6), 130), fill=YEL)
        return im

    def guess_card(self, d, t, text, y, struck):
        frame_box(d, 140, y, 940, y + 230, WHITE, BG, bw=8)
        pix_text(d, 540, y + 58, 'догадка', 28, GREY)
        pix_text(d, 540, y + 140, text, 76, LAV if struck > 0 else YEL)
        if struck > 0:
            tw = pix_width(text, 76)
            x0 = 540 - tw / 2 - 20
            d.rectangle((x0, y + 136, x0 + (tw + 40) * struck, y + 146), fill=RED)

    def code(self, d, t):
        c = self.s.get('code')
        if not c:
            return
        lines = [l for l in c['lines'] if t >= l.get('appear', 0)]
        frame_box(d, 70, 1120, 950, 1390, WHITE, PANEL, bw=6)
        f = font('mono', 25)
        hl = {}
        for ts, i, role in c['highlight']:
            if t >= ts:
                hl[i] = role
        hl = {i: r for i, r in hl.items() if r != 'off'}
        for i, l in enumerate(lines):
            y = 1144 + i * 38
            idx = c['lines'].index(l)
            role = hl.get(idx)
            col = DIM
            if role:
                bg = {'yel': (70, 64, 30), 'error': (90, 20, 50), 'ok': (10, 80, 40), 'lav': (60, 54, 90)}[role]
                d.rectangle((82, y - 4, 938, y + 32), fill=bg)
                col = {'yel': YEL, 'error': (255, 120, 150), 'ok': GRN, 'lav': WHITE}[role]
            d.text((96, y), l['text'], font=f, fill=col)

    def captions(self, d, t):
        for a, b, text in self.s['captions']:
            if a <= t < b:
                f = font('inter', 44)
                tw = f.getlength(text)
                x0 = snap(510 - tw / 2 - 30)
                frame_box(d, x0, 1416, snap(510 + tw / 2 + 30), 1500, WHITE, BG, bw=6)
                d.text((510 - tw / 2, 1458), text, font=f, fill=YEL, anchor='lm')


def sq(f, dur, sr=48000, vol=0.25, f2=None):
    n = int(sr * dur)
    tt = np.arange(n) / sr
    fr = np.linspace(f, f2 or f, n)
    ph = np.cumsum(fr) / sr
    w = np.sign(np.sin(2 * np.pi * ph))
    env = np.minimum(1, (n - np.arange(n)) / (sr * 0.02))
    return w * vol * env


def noise(dur, sr=48000, vol=0.15):
    n = int(sr * dur)
    return (np.random.RandomState(1).rand(n) * 2 - 1) * vol * np.linspace(1, 0, n)


def sfx(scene, path, sr=48000):
    out = np.zeros(int(sr * (scene['duration'] + 1)))

    def put(t, a):
        i = int(t * sr)
        out[i:i + len(a)] += a[:len(out) - i]

    for ev in scene['sfx']:
        t, k = ev
        if k == 'title':
            put(t, sq(523, .08)); put(t + .09, sq(784, .12))
        elif k == 'appear':
            put(t, sq(660, .05, vol=.12))
        elif k == 'say':
            put(t, sq(880, .05, vol=.15)); put(t + .06, sq(1175, .06, vol=.15))
        elif k == 'move':
            put(t, sq(300, .18, vol=.12, f2=900))
        elif k == 'block':
            put(t, sq(110, .3, vol=.3)); put(t, noise(.25))
        elif k == 'question':
            put(t, sq(440, .07, vol=.15)); put(t + .08, sq(660, .1, vol=.15))
        elif k == 'tick':
            put(t, sq(1400, .03, vol=.1))
        elif k == 'ok':
            for i, f in enumerate((523, 659, 784, 1047)):
                put(t + i * .07, sq(f, .09, vol=.18))
        elif k == 'strike':
            put(t, noise(.2, vol=.2)); put(t, sq(200, .15, vol=.15, f2=90))
    out = np.clip(out, -1, 1)
    with wave.open(path, 'wb') as w:
        w.setnchannels(1); w.setsampwidth(2); w.setframerate(sr)
        w.writeframes((out * 32767).astype(np.int16).tobytes())


def main():
    scene = json.load(open(sys.argv[2], encoding='utf-8'))
    sc = Scene(scene)
    if sys.argv[1] == 'stills':
        os.makedirs(sys.argv[3], exist_ok=True)
        for x in sys.argv[4:]:
            p = os.path.join(sys.argv[3], f'still_{float(x):06.2f}.png')
            sc.frame(float(x)).save(p)
            print(p)
        return
    out = sys.argv[3]
    vid = out + '.video.mp4'
    p = subprocess.Popen(['ffmpeg', '-v', 'error', '-y', '-f', 'rawvideo', '-pix_fmt', 'rgb24', '-s', f'{W}x{H}', '-r',
                          str(FPS), '-i', '-', '-c:v', 'libx264', '-preset', 'veryfast', '-crf', '18', '-pix_fmt',
                          'yuv420p', vid], stdin=subprocess.PIPE)
    for i in range(int(round(scene['duration'] * FPS))):
        p.stdin.write(sc.frame(i / FPS).tobytes())
    p.stdin.close()
    p.wait()
    sfx(scene, out + '.sfx.wav')
    print(vid, out + '.sfx.wav')


if __name__ == '__main__':
    main()
