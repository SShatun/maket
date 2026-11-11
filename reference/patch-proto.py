import sys

p = sys.argv[1] + '/maket/engine.py'
s = open(p, encoding='utf-8').read()


def rep(old, new):
    global s
    assert s.count(old) == 1, old
    s = s.replace(old, new)


rep("e['rise_t'] = 0.05 + i * T.INTRO_STAGGER",
    "e['rise_t'] = e.get('appear', 0.05 + i * T.INTRO_STAGGER)")

rep("self.focus = next((ev for ev in self.events if ev['do'] == 'focus'), None)",
    "self.focus = next((ev for ev in self.events if ev['do'] == 'focus'), None)\n"
    "        self.shots = [ev for ev in self.events if ev['do'] == 'shot']\n"
    "        self.titles = [ev for ev in self.events if ev['do'] == 'title']")

rep("""    def set_cam(self, t):
        z = 1.0 + T.CAM_DRIFT_ZOOM * ease(t / self.duration)
        if self.focus: z += T.CAM_FOCUS_ZOOM * ease((t - self.focus['t']) / 2.5)
        self.zoom, self.sway = z, -T.CAM_SWAY_PX * math.sin(t / self.duration * math.pi)

    def pt(self, x, y, z):
        X, Y = self._raw(x, y, z, self.U, self.ox, self.oy)
        return (self.fx + (X - self.fx) * self.zoom, self.fy + (Y - self.fy) * self.zoom + self.sway)
""", """    def set_cam(self, t):
        z = 1.0 + T.CAM_DRIFT_ZOOM * ease(t / self.duration)
        if self.focus: z += T.CAM_FOCUS_ZOOM * ease((t - self.focus['t']) / 2.5)
        self.piv, self.out, k, zs = (self.fx, self.fy), (self.fx, self.fy), 0.0, 1.0
        for ev in self.shots:
            d = ev.get('dur', 0.6)
            kk = prog(t, ev['t'], ev['t'] + d) * (1 - prog(t, ev['until'], ev['until'] + d))
            if kk > k:
                a = self.anchor(ev['target']); x, y = self._xy(a[0], a[1])
                P = self._raw(x, y, a[2], self.U, self.ox, self.oy); C = (self.cx, self.cy)
                k, zs = kk, ev.get('zoom', 1.3)
                self.piv = (lerp(self.fx, P[0], k), lerp(self.fy, P[1], k))
                self.out = (lerp(self.fx, C[0], k), lerp(self.fy, C[1], k))
        z *= 1 + (zs - 1) * k
        self.zoom, self.sway = z, -T.CAM_SWAY_PX * math.sin(t / self.duration * math.pi)

    def pt(self, x, y, z):
        X, Y = self._raw(x, y, z, self.U, self.ox, self.oy)
        return (self.out[0] + (X - self.piv[0]) * self.zoom, self.out[1] + (Y - self.piv[1]) * self.zoom + self.sway)
""")

rep("""        self.oy = (top + bottom) / 2 - (ymin + ymax) / 2 * U
""", """        self.oy = (top + bottom) / 2 - (ymin + ymax) / 2 * U
        self.cx, self.cy = (left + right) / 2, (top + bottom) / 2
""")

rep("""        # caption
""", """        # progress
        c.rr(70, 120, 1010, 128, 4, fill=T.GRID)
        c.rr(70, 120, 70 + 940 * clamp(t / sc.duration), 128, 4, fill=T.INK)

        # titles
        for ev in sc.titles:
            if not (ev['t'] <= t < ev['until'] + .25): continue
            k = ease_out_back((t - ev['t']) / .35) * (1 - prog(t, ev['until'], ev['until'] + .25))
            if k <= .02: continue
            f = font(('Inter-Bold.otf', 84), max(.3, k)); y = ev.get('y', 860)
            tw = c.tw(ev['text'], f); h = 150 * max(.3, k)
            c.rr(540 - tw / 2 - 48, y - h / 2, 540 + tw / 2 + 48, y + h / 2, h / 2, fill=T.ROLES[ev.get('role', 'error')])
            c.text(540, y, ev['text'], f, T.CARD)

        # caption
""")

open(p, 'w', encoding='utf-8').write(s)

q = sys.argv[1] + '/maket/sfx.py'
s = open(q, encoding='utf-8').read()
rep("""        elif ev['do'] == 'focus':""", """        elif ev['do'] == 'title':
            m.add(ev['t'], thud(m), g['thud'] * .8); m.add(ev['t'] + .05, pop(m), g['pop'])
        elif ev['do'] == 'shot':
            m.add(ev['t'], whoosh(m, .5), g['whoosh'] * 1.4)
        elif ev['do'] == 'focus':""")
open(q, 'w', encoding='utf-8').write(s)
print('ok')
