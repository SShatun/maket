"""Maket: isometric explainer video engine. Scene JSON in, 1080x1920 frames out."""
import math, os
from PIL import Image, ImageDraw, ImageFont
from . import tokens as T

HERE = os.path.dirname(__file__)
S = T.SUPERSAMPLE
W, H = T.WIDTH, T.HEIGHT


def font(spec, scale=1.0):
    name, size = spec
    return ImageFont.truetype(os.path.join(HERE, 'fonts', name), int(size * S * scale))


# ---------- maths
def clamp(x, a=0.0, b=1.0): return max(a, min(b, x))
def ease(x):
    x = clamp(x); return 4 * x ** 3 if x < .5 else 1 - (-2 * x + 2) ** 3 / 2
def ease_out_back(x):
    x = clamp(x); c1 = 1.5; return 1 + (c1 + 1) * (x - 1) ** 3 + c1 * (x - 1) ** 2
def prog(t, a, b): return ease((t - a) / (b - a)) if b > a else float(t >= a)
def lerp(a, b, k): return a + (b - a) * k
def mix(c, k, base=T.GROUND): return tuple(int(base[i] + (c[i] - base[i]) * k) for i in range(3))
def shade(c, k): return tuple(int(v * k) for v in c)
def lighten(c, k): return tuple(int(v + (255 - v) * k) for v in c)


class Scene:
    def __init__(self, spec):
        self.spec = spec
        self.duration = float(spec['duration'])
        self.ents = {e['id']: dict(e) for e in spec['entities']}
        self.order = [e['id'] for e in spec['entities']]
        for i, eid in enumerate(self.order):
            e = self.ents[eid]
            e.setdefault('role', 'producer'); e.setdefault('kind', 'block')
            e.setdefault('label_pos', 'below' if e['kind'] in ('slots', 'cylinder') or e['role'] == 'consumer' else 'above')
            e['rise_t'] = 0.05 + i * T.INTRO_STAGGER
            if e['kind'] == 'slots':
                e.setdefault('count', 3); e.setdefault('spacing', 2.5)
        self.code = spec.get('code')
        self.events = sorted(spec.get('events', []), key=lambda ev: ev['t'])
        self.blocks = [ev for ev in self.events if ev['do'] == 'block']
        self.focus = next((ev for ev in self.events if ev['do'] == 'focus'), None)
        self.flashes = [(ev['t'], ev['target']) for ev in self.events if ev['do'] == 'flash']
        self.recolors = [ev for ev in self.events if ev['do'] == 'state']
        self.moves = {}
        for ev in self.events:
            if ev['do'] != 'send': continue
            ev.setdefault('dur', T.SEND_DUR)
            self.moves.setdefault(str(ev['token']), []).append(ev)
            if ev.get('consume'): self.flashes.append((ev['t'] + ev['dur'], self._ent_of(ev['to'])))
        self._words()
        self._fit()

    # ----- anchors
    def _ent_of(self, anchor):
        if isinstance(anchor, str): return anchor.split('[')[0]
        return None

    def slot_ab(self, e, i):
        n, sp = e['count'], e['spacing']
        return (e['at'][0] + (i - (n - 1) / 2) * sp, e['at'][1])

    def anchor(self, a):
        """-> (a, b, z) world point where a token sits."""
        if isinstance(a, (list, tuple)): return tuple(a) if len(a) == 3 else (a[0], a[1], 0.0)
        if '[' in a:
            eid, idx = a[:-1].split('['); e = self.ents[eid]
            x, y = self.slot_ab(e, int(idx)); return (x, y, T.SLOT_H)
        e = self.ents[a]
        top = {'block': T.BLOCK_H, 'cylinder': T.CYL_H, 'slots': T.SLOT_H}[e['kind']]
        return (e['at'][0], e['at'][1], top)

    # ----- projection & camera
    def _raw(self, x, y, z, U, ox, oy): return (ox + (x - y) * 0.866 * U, oy + (x + y) * 0.5 * U - z * U)

    def _fit(self):
        pts = []
        for e in self.ents.values():
            fps = [self.slot_ab(e, i) for i in range(e['count'])] if e['kind'] == 'slots' else [tuple(e['at'])]
            r = {'block': T.BLOCK, 'slots': T.SLOT, 'cylinder': 2 * T.CYL_R}[e['kind']] / 2
            h = {'block': T.BLOCK_H, 'slots': T.SLOT_H + T.TOKEN, 'cylinder': T.CYL_H}[e['kind']]
            for a, b in fps:
                for da, db in ((-r, 0), (r, 0), (0, -r), (0, r)):
                    x, y = b + db + a + da, b + db - a - da
                    pts.append((self._raw(x, y, 0, 1, 0, 0), self._raw(x, y, h, 1, 0, 0)))
        flat = [p for pair in pts for p in pair]
        xmin, xmax = min(p[0] for p in flat), max(p[0] for p in flat)
        ymin, ymax = min(p[1] for p in flat), max(p[1] for p in flat)
        top = T.STAGE_TOP + 92
        bottom = (T.STAGE_BOTTOM_WITH_CODE if self.code else T.STAGE_BOTTOM_NO_CODE) - 92
        left, right = T.SAFE_LEFT_X + 60, T.SAFE_RIGHT_X - 60
        U = min((right - left) / max(1e-6, xmax - xmin), (bottom - top) / max(1e-6, ymax - ymin), 66) / (1 + (T.CAM_DRIFT_ZOOM + T.CAM_FOCUS_ZOOM) * .6)
        self.U = U
        self.ox = (left + right) / 2 - (xmin + xmax) / 2 * U
        self.oy = (top + bottom) / 2 - (ymin + ymax) / 2 * U
        if self.focus:
            fa = self.anchor(self.focus['targets'][0]); x, y = self._xy(fa[0], fa[1])
            self.fx, self.fy = self._raw(x, y, 0, U, self.ox, self.oy)
        else:
            self.fx, self.fy = (left + right) / 2, (top + bottom) / 2

    def _xy(self, a, b): return (b + a, b - a)

    def set_cam(self, t):
        z = 1.0 + T.CAM_DRIFT_ZOOM * ease(t / self.duration)
        if self.focus: z += T.CAM_FOCUS_ZOOM * ease((t - self.focus['t']) / 2.5)
        self.zoom, self.sway = z, -T.CAM_SWAY_PX * math.sin(t / self.duration * math.pi)

    def pt(self, x, y, z):
        X, Y = self._raw(x, y, z, self.U, self.ox, self.oy)
        return (self.fx + (X - self.fx) * self.zoom, self.fy + (Y - self.fy) * self.zoom + self.sway)

    def pab(self, a, b, z): x, y = self._xy(a, b); return self.pt(x, y, z)

    # ----- state queries
    def blocked(self, eid, t):
        return any(ev['target'] == eid and ev['t'] <= t < ev.get('until', 1e9) for ev in self.blocks)

    def block_start(self, eid, t):
        for ev in self.blocks:
            if ev['target'] == eid and ev['t'] <= t < ev.get('until', 1e9): return ev['t']
        return None

    def role_at(self, eid, t):
        role = self.ents[eid]['role']
        for ev in self.recolors:
            if ev['target'] == eid and ev['t'] <= t < ev.get('until', 1e9): role = ev['role']
        if self.blocked(eid, t): role = 'error'
        return role

    def dim(self, eid, t):
        if not self.focus or eid in [self._ent_of(x) for x in self.focus['targets']]: return 1.0
        return 1 - (1 - T.FOCUS_DIM) * prog(t, self.focus['t'], self.focus['t'] + T.FOCUS_DUR)

    def timeline(self, items, t, default=''):
        cur = default
        for row in items or []:
            if row[0] <= t: cur = row[1]
        return cur

    def token(self, tid, t):
        """-> None or dict(pos=(a,b,z), at=anchor or None, waiting=bool)"""
        mv = self.moves[tid]
        if t < mv[0]['t']: return None
        last = None
        for ev in mv:
            if t < ev['t']: break
            p0, p1 = self.anchor(ev['from']), self.anchor(ev['to'])
            end = ev['t'] + ev['dur']
            if t <= end:
                k = prog(t, ev['t'], end)
                same = self._ent_of(ev['from']) == self._ent_of(ev['to'])
                arc = ev.get('arc', T.SHIFT_ARC if same else T.SEND_ARC)
                return dict(pos=(lerp(p0[0], p1[0], k), lerp(p0[1], p1[1], k), lerp(p0[2], p1[2], k) + math.sin(math.pi * k) * arc),
                            at=None, waiting=False, moving=True)
            last = ev
        if last.get('consume'): return None
        return dict(pos=self.anchor(last['to']), at=last['to'], waiting=bool(last.get('waiting')), moving=False)

    def occupancy(self, eid, t):
        n = 0
        for tid in self.moves:
            s = self.token(tid, t)
            if s and isinstance(s['at'], str) and s['at'].startswith(eid + '['): n += 1
        return n

    # ----- captions
    def _words(self):
        self.words = []; cid = 0
        for a, b, txt in self.spec.get('captions', []):
            ws = txt.split(); tot = sum(len(w) + 2 for w in ws); t = a
            n = len(ws); k = -(-n // T.CAPTION_MAX_WORDS); per = -(-n // k)
            for i, w in enumerate(ws):
                dt = (b - a) * (len(w) + 2) / tot
                if i % per == 0: cid += 1
                self.words.append((t, t + dt, w, cid)); t += dt


class Canvas:
    def __init__(self, sc):
        self.sc = sc
        self.im = Image.new('RGB', (W * S, H * S), T.GROUND)
        self.d = ImageDraw.Draw(self.im)
    def P(self, pts): return [(x * S, y * S) for x, y in pts]
    def poly(self, pts, fill): self.d.polygon(self.P(pts), fill=fill)
    def line(self, pts, fill, w=1.5): self.d.line(self.P(pts), fill=fill, width=max(1, int(w * S)))
    def rr(self, x0, y0, x1, y1, r, fill=None): self.d.rounded_rectangle([x0 * S, y0 * S, x1 * S, y1 * S], r * S, fill=fill)
    def text(self, x, y, s, f, fill, a='mm'): self.d.text((x * S, y * S), s, font=f, fill=fill, anchor=a)
    def tw(self, s, f): return self.d.textlength(s, font=f) / S

    def cube(self, a, b, z, w, h, col, dx=0.0):
        sc = self.sc; x, y = sc._xy(a, b); x -= w / 2; y -= w / 2
        q = lambda X, Y, Z: (sc.pt(X, Y, Z)[0] + dx, sc.pt(X, Y, Z)[1])
        top = [q(x, y, z + h), q(x + w, y, z + h), q(x + w, y + w, z + h), q(x, y + w, z + h)]
        fx = [q(x + w, y, z), q(x + w, y + w, z), q(x + w, y + w, z + h), q(x + w, y, z + h)]
        fy = [q(x, y + w, z), q(x + w, y + w, z), q(x + w, y + w, z + h), q(x, y + w, z + h)]
        self.poly(fx, shade(col, T.FACE_RIGHT)); self.poly(fy, shade(col, T.FACE_LEFT)); self.poly(top, col)
        c = sc.pab(a, b, z + h); return (c[0] + dx, c[1])

    def cylinder(self, a, b, r, h, col):
        sc = self.sc
        g = sc.pab(a, b, 0); tp = sc.pab(a, b, h)
        rx, ry = 1.2247 * r * sc.U * sc.zoom, 0.7071 * r * sc.U * sc.zoom
        L, R = shade(col, T.FACE_LEFT), shade(col, T.FACE_RIGHT)
        bx = lambda c: [(c[0] - rx) * S, (c[1] - ry) * S, (c[0] + rx) * S, (c[1] + ry) * S]
        self.d.pieslice(bx(g), 90, 180, fill=L); self.d.pieslice(bx(g), 0, 90, fill=R)
        self.d.rectangle([(g[0] - rx) * S, tp[1] * S, g[0] * S, g[1] * S], fill=L)
        self.d.rectangle([g[0] * S, tp[1] * S, (g[0] + rx) * S, g[1] * S], fill=R)
        for k in (1 / 3, 2 / 3):
            c = sc.pab(a, b, h * k); self.d.arc(bx(c), 0, 180, fill=shade(col, .55), width=int(2 * S))
        self.d.ellipse(bx(tp), fill=col)
        return tp

    def shadow(self, a, b, w, k):
        sc = self.sc; x, y = sc._xy(a, b); x -= w / 2; y -= w / 2; o = .3
        self.poly([sc.pt(x + o, y + o, 0), sc.pt(x + w + o, y + o, 0), sc.pt(x + w + o, y + w + o, 0), sc.pt(x + o, y + w + o, 0)], mix(T.SHADOW, k))


class Renderer:
    def __init__(self, spec):
        self.sc = Scene(spec)
        self.f_label, self.f_status = font(T.FONT_LABEL), font(T.FONT_STATUS)
        self.f_tok, self.f_tok_s = font(T.FONT_TOKEN), font(T.FONT_TOKEN_SMALL)
        self.f_code, self.f_cap = font(T.FONT_CODE), font(T.FONT_CAPTION)

    def frame(self, t):
        sc = self.sc; sc.set_cam(t); c = Canvas(sc)
        for i in range(-20, 32):
            c.line([sc.pt(i, -20, 0), sc.pt(i, 32, 0)], T.GRID); c.line([sc.pt(-20, i, 0), sc.pt(32, i, 0)], T.GRID)
        rise = lambda e: ease_out_back((t - e['rise_t']) / T.INTRO_RISE)
        items, labels = [], []

        for eid in sc.order:
            e = sc.ents[eid]; r = rise(e); k = clamp(r) * sc.dim(eid, t)
            if r <= 0: continue
            col = mix(T.ROLES[sc.role_at(eid, t)], sc.dim(eid, t))
            fl = max([1 - abs(t - (ft + .1)) / .25 for ft, fe in sc.flashes if fe == eid and ft - .15 < t < ft + .35] or [0])
            if fl > 0: col = lighten(col, .35 * fl)
            bs = sc.block_start(eid, t)
            dx = math.sin(t * 60) * T.SHAKE_PX * (1 - prog(t, bs, bs + T.SHAKE_DUR)) if bs is not None and t < bs + T.SHAKE_DUR else 0
            a, b = e['at']
            if e['kind'] == 'block':
                c.shadow(a, b, T.BLOCK, k)
                items.append((2 * b, lambda a=a, b=b, col=col, r=r, dx=dx: c.cube(a, b, 0, T.BLOCK, T.BLOCK_H * max(0, r), col, dx)))
                half = T.BLOCK / 2 * sc.U * sc.zoom
                top = sc.pab(a, b, T.BLOCK_H)[1] - half; bot = sc.pab(a, b, 0)[1] + half
                labels.append((e, sc.pab(a, b, 0)[0] + dx, top, bot, k))
            elif e['kind'] == 'slots':
                for i in range(e['count']):
                    sa, sb = sc.slot_ab(e, i); c.shadow(sa, sb, T.SLOT, k)
                    items.append((2 * sb, lambda sa=sa, sb=sb, col=col, r=r: c.cube(sa, sb, 0, T.SLOT, T.SLOT_H * max(0, r), col)))
                half = T.SLOT / 2 * sc.U * sc.zoom
                labels.append((e, sc.pab(a, b, 0)[0], sc.pab(a, b, T.SLOT_H)[1] - half, sc.pab(a, b, 0)[1] + half, k))
            elif e['kind'] == 'cylinder':
                c.shadow(a, b, 2 * T.CYL_R, k)
                items.append((2 * b, lambda a=a, b=b, col=col, r=r: c.cylinder(a, b, T.CYL_R, T.CYL_H * max(0.01, r), col)))
                ry = 0.7071 * T.CYL_R * sc.U * sc.zoom
                labels.append((e, sc.pab(a, b, 0)[0], sc.pab(a, b, T.CYL_H)[1] - ry, sc.pab(a, b, 0)[1] + ry, k))

        for tid in sc.moves:
            s = sc.token(tid, t)
            if not s: continue
            a, b, z = s['pos']
            host = sc._ent_of(s['at']) if s['at'] else sc._ent_of(sc.moves[tid][0]['from'])
            d = sc.dim(host, t) if (not s['moving']) else 1.0
            mv = [ev for ev in sc.moves[tid] if ev['t'] <= t][-1]
            col = T.CARD if mv.get('role') is None else T.ROLES[mv['role']]
            ink = T.TOKEN_INK if mv.get('role') is None else T.CARD
            size = T.TOKEN
            if s['waiting']:
                col = mix(T.ROLES['error'], .75 + .25 * math.sin(t * 2 * math.pi * T.BLOCK_PULSE_HZ)); ink = T.CARD; size = .9
            col = mix(col, d); ink = mix(ink, d, col)
            def draw(a=a, b=b, z=z, col=col, ink=ink, size=size, tid=tid):
                tp = c.cube(a, b, z, size, size, col)
                lab = str(sc.moves[tid][0].get('label', tid))
                c.text(tp[0], tp[1] + 2, lab, self.f_tok if len(lab) <= 3 else self.f_tok_s, ink)
            items.append((2 * b + .75 * z + .01, draw))

        items.sort(key=lambda x: x[0])
        for _, fn in items: fn()

        # labels
        for e, x, top, bot, k in labels:
            eid = e['id']; blocked = sc.blocked(eid, t)
            name_col = mix(T.ERROR_TEXT if blocked else T.INK, k)
            st = sc.timeline(e.get('status'), t)
            if e['kind'] == 'slots':
                n = sc.occupancy(eid, t); full = n >= e['count']
                st2 = f"{n}/{e['count']}" + (' забит' if full and e.get('full_word', True) else '')
                st_col = mix(T.ERROR_TEXT if full else T.MUTED, k)
                st = st or st2
            else:
                st_col = mix(T.ERROR_TEXT if blocked else T.MUTED, k)
            if e['label_pos'] in ('right', 'left'):
                half = {'block': T.BLOCK, 'slots': T.SLOT, 'cylinder': 2 * T.CYL_R}[e['kind']] * 0.866 * sc.U * sc.zoom
                if e['kind'] == 'slots': half += (e['count'] - 1) / 2 * e['spacing'] * 2 * 0.866 * sc.U * sc.zoom
                sgn = 1 if e['label_pos'] == 'right' else -1; anc = 'lm' if sgn > 0 else 'rm'
                xx = x + sgn * (half + 24); yy = (top + bot) / 2 - 8
                c.text(xx, yy - T.LABEL_GAP / 2, e['label'], self.f_label, name_col, anc)
                if st: c.text(xx, yy + T.LABEL_GAP / 2, st, self.f_status, st_col, anc)
            elif e['label_pos'] == 'above':
                c.text(x, top - 40 - T.LABEL_GAP, e['label'], self.f_label, name_col)
                if st: c.text(x, top - 40, st, self.f_status, st_col)
            else:
                c.text(x, bot + 30, e['label'], self.f_label, name_col)
                prog_rows = e.get('progress')
                if prog_rows:
                    p, lab, show = 0.0, st, False
                    for a0, a1, lb in prog_rows:
                        if t >= a0: p, lab, show = clamp((t - a0) / (a1 - a0)), lb, t < a1 + 0.6 or e.get('keep_progress', a1 > sc.duration)
                    if not show:
                        if st: c.text(x, bot + 30 + T.LABEL_GAP, st, self.f_status, st_col)
                        continue
                    c.text(x - 8, bot + 30 + T.LABEL_GAP, lab, self.f_status, st_col, 'rm')
                    c.rr(x + 8, bot + 23 + T.LABEL_GAP, x + 132, bot + 37 + T.LABEL_GAP, 7, fill=mix(T.GRID, k))
                    if p > .02: c.rr(x + 8, bot + 23 + T.LABEL_GAP, x + 8 + 124 * p, bot + 37 + T.LABEL_GAP, 7, fill=mix(T.ROLES[e['role']], k))
                elif st:
                    c.text(x, bot + 30 + T.LABEL_GAP, st, self.f_status, st_col)

        # code card
        if sc.code:
            k = prog(t, .5, 1.0); x0, y0, x1, y1 = T.CODE_BOX
            c.rr(x0, y0, x1, y1, 22, fill=mix(T.CARD, k))
            li, role = 0, 'producer'
            for row in sc.code.get('highlight', []):
                if row[0] <= t: li, role = row[1], (row[2] if len(row) > 2 else 'producer')
            lines = sc.code['lines'][:7]
            lh = min(T.CODE_LINE_H, (y1 - y0 - 56) / max(1, len(lines)))
            fc = self.f_code if lh >= T.CODE_LINE_H else font(T.FONT_CODE, max(.82, lh / T.CODE_LINE_H))
            top0 = y0 + (y1 - y0 - lh * len(lines)) / 2 + 4
            ly = top0 + li * lh
            c.rr(x0 + 16, ly - 6, x1 - 16, ly + lh - 8, 8, fill=mix(T.CODE_TINT[role], k))
            for i, ln in enumerate(lines):
                c.text(x0 + 38, top0 + i * lh, ln, fc, mix(T.INK if i == li else T.CODE_DIM, k), 'la')

        # caption
        cur = [w for w in sc.words if w[0] <= t]
        if cur and t <= cur[-1][1] + .35:
            ws = [w for w in sc.words if w[3] == cur[-1][3]]
            wd = [c.tw(w[2], self.f_cap) for w in ws]; sp = c.tw(' ', self.f_cap)
            tot = sum(wd) + sp * (len(ws) - 1); x = T.CAPTION_CENTER_X - tot / 2
            c.rr(x - 30, T.CAPTION_Y - T.CAPTION_H / 2, x + tot + 30, T.CAPTION_Y + T.CAPTION_H / 2, T.CAPTION_H / 2, fill=T.INK)
            for w, dd in zip(ws, wd):
                c.text(x, T.CAPTION_Y, w[2], self.f_cap, T.CARD if w[0] <= t else (130, 142, 154), 'lm'); x += dd + sp
        return c.im.reduce(S)
