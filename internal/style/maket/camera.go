package maket

import "math"

type camera struct {
	u, ox, oy      float64
	px, py, cx, cy float64
	zoom, sway     float64
}

func (s *Style) camera(t float64) camera {
	c := camera{u: s.u, ox: s.ox, oy: s.oy, px: s.fx, py: s.fy, cx: s.fx, cy: s.fy}
	c.zoom = 1 + driftZoom*ease(t/s.duration)
	if s.focus != nil {
		c.zoom += focusZoom * ease((t-s.focus.at)/2.5)
	}
	k, zoom := 0.0, 1.0
	for _, ev := range s.events {
		if ev.Do != "shot" {
			continue
		}
		kk := prog(t, ev.at, ev.at+ev.Dur) * (1 - prog(t, ev.until, ev.until+ev.Dur))
		if kk > k {
			p := ev.target
			x, y := s.ox+2*p.x*0.866*s.u, s.oy+(p.y-p.z)*s.u
			k, zoom = kk, ev.Zoom
			c.px, c.py = lerp(s.fx, x, k), lerp(s.fy, y, k)
			c.cx, c.cy = lerp(s.fx, s.cx, k), lerp(s.fy, s.cy, k)
		}
	}
	c.zoom *= 1 + (zoom-1)*k
	c.sway = -swayPX * math.Sin(t/s.duration*math.Pi)
	return c
}

func (c camera) pt(p point) (float64, float64) {
	x, y := c.ox+(p.x-p.y)*0.866*c.u, c.oy+(p.x+p.y)*0.5*c.u-p.z*c.u
	return c.cx + (x-c.px)*c.zoom, c.cy + (y-c.py)*c.zoom + c.sway
}

func (c camera) pab(p point) (float64, float64) {
	return c.pt(point{p.y + p.x, p.y - p.x, p.z})
}
