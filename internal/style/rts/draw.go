package rts

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/fogleman/gg"
	"golang.org/x/image/font"

	"maket/internal/render"
	spr "maket/internal/style/sprites"
)

type item struct {
	key    float64
	a      *actor
	x, y   int
	im     *image.RGBA
	flip   bool
	moving bool
}

func hash(a, b, c, d uint64) uint64 {
	x := a*0x9E3779B97F4A7C15 ^ b*0xBF58476D1CE4E5B9 ^ c*0x94D049BB133111EB ^ d*0x2545F4914F6CDD1D
	x ^= x >> 30
	x *= 0xBF58476D1CE4E5B9
	x ^= x >> 27
	x *= 0x94D049BB133111EB
	x ^= x >> 31
	return x
}

func last(l []timedStr, t float64) (string, float64) {
	v, at := "", 0.0
	for _, x := range l {
		if t >= x.t {
			v, at = x.v, x.t
		}
	}
	return v, at
}

func clamp(x float64) float64 { return math.Max(0, math.Min(1, x)) }

func floorTo(v int) int { return int(math.Floor(float64(v)/px)) * px }

func (s *Style) Draw(dst *image.RGBA, t float64) {
	sh := s.shotAt(t)
	draw.Draw(dst, dst.Bounds(), sh.bg, image.Point{}, draw.Src)
	ox, oy := sh.offset(t)
	if ox != 0 || oy != 0 {
		fill(dst, mapRect, black)
		d := image.Pt(ox*px, oy*px)
		r := mapRect.Add(d).Intersect(mapRect)
		draw.Draw(dst, r, sh.bg, r.Min.Sub(d), draw.Src)
	}
	world := dst.SubImage(mapRect).(*image.RGBA)
	items := s.items(sh, t)
	wallDone := sh.wall == nil
	for _, it := range items {
		if !wallDone && it.key >= float64(sh.wall.y) {
			s.wallDraw(world, sh, ox, oy)
			wallDone = true
		}
		s.blit(world, it, ox, oy)
	}
	if !wallDone {
		s.wallDraw(world, sh, ox, oy)
	}
	s.selection(world, sh, t, items, ox, oy)
	s.marks(world, sh, t, items, ox, oy)
	s.plates(world, sh, t, items, ox, oy)
	for _, f := range sh.focuses {
		if t < f.t || t >= f.until {
			continue
		}
		a := uint8(255 * 0.6 * clamp((t-f.t)/0.6))
		draw.Draw(world, mapRect, image.NewUniform(color.RGBA{0, 0, 0, a}), image.Point{}, draw.Over)
		for _, it := range items {
			if f.targets[it.a] {
				s.blit(world, it, ox, oy)
			}
		}
	}
	fog := s.art.fog
	for _, r := range s.art.fogRuns {
		copy(dst.Pix[dst.PixOffset(r[1], mapTop+r[0]):dst.PixOffset(r[2], mapTop+r[0])], fog.Pix[fog.PixOffset(r[1], r[0]):fog.PixOffset(r[2], r[0])])
	}
	s.elements(world, sh, t)
	dc := gg.NewContextForRGBA(dst)
	faces := map[float64]font.Face{}
	face := func(size float64) font.Face {
		if f, ok := faces[size]; ok {
			return f
		}
		f := render.Face(s.fonts.Pixel, size)
		faces[size] = f
		return f
	}
	s.message(dc, face, sh, t)
	s.hud(dst, dc, face, sh, t, items)
	render.Caption(dst, s.sc, s.tl, s.theme, t)
	s.flash(dst, sh, t)
}

func (s *Style) items(sh *shot, t float64) []item {
	out := make([]item, 0, len(sh.actors))
	var one [1]spr.Step
	for _, a := range sh.actors {
		if t < a.appear || t >= a.gone {
			continue
		}
		x, y, st, since, flip, moving := a.pose(t)
		if a.jitter {
			n := uint64(int(t*30) / 3)
			h := hash(sh.seed, a.salt, n, 3)
			x += float64(int(h%3) - 1)
			y += float64(int((h/3)%3) - 1)
		}
		role, _ := last(a.roles, t)
		var im *image.RGBA
		if st != "" {
			one[0] = spr.Step{T: since, V: st}
			im = s.set.Frame(a.kind, one[:], t, role)
		} else {
			im = s.set.Frame(a.kind, a.steps, t, role)
		}
		if im == nil {
			continue
		}
		xi, yi := int(math.Floor(x+0.5)), int(math.Floor(y+0.5))
		out = append(out, item{key: float64(yi), a: a, x: xi, y: yi, im: im, flip: flip, moving: moving})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].key < out[j].key })
	return out
}

func (s *Style) anchor(it item) (int, int, *image.RGBA) {
	spec := s.set.Specs[it.a.kind]
	ax, ay := spec.Anchor[0]/2, spec.Anchor[1]/2
	im := it.im
	if it.flip {
		if f := s.flips[im]; f != nil {
			im = f
			ax = im.Bounds().Dx() - ax
		}
	}
	return ax, ay, im
}

func (s *Style) blit(world *image.RGBA, it item, ox, oy int) {
	ax, ay, im := s.anchor(it)
	x := (it.x+ox)*px - ax
	y := mapTop + (it.y+oy)*px - ay
	draw.Draw(world, image.Rect(x, y, x+im.Bounds().Dx(), y+im.Bounds().Dy()), im, image.Point{}, draw.Over)
}

func (s *Style) wallDraw(world *image.RGBA, sh *shot, ox, oy int) {
	w := sh.wall.im
	y := mapTop + (sh.wall.y-13+oy)*px
	x := ox * px
	draw.Draw(world, image.Rect(x, y, x+w.Bounds().Dx(), y+w.Bounds().Dy()), w, image.Point{}, draw.Over)
}

func (sh *shot) offset(t float64) (int, int) {
	amp := 0
	for _, x := range sh.shakes {
		if t >= x.t && t < x.until {
			amp = max(amp, x.px)
		}
	}
	for _, x := range sh.shocks {
		if d := t - x.t; d >= 0 && d < 0.7 {
			amp = max(amp, int(math.Ceil(float64(x.px)*(1-d/0.7))))
		}
	}
	if amp == 0 {
		return 0, 0
	}
	h := hash(sh.seed, uint64(int(math.Round(t*30))), 7, 11)
	span := uint64(2*amp + 1)
	ox, oy := int(h%span)-amp, int((h/span)%span)-amp
	if ox == 0 && oy == 0 {
		ox = amp
		if h>>40&1 == 0 {
			ox = -amp
		}
	}
	return ox, oy
}

func (sh *shot) pickAt(t float64) pick {
	p := sh.picks[0]
	for _, x := range sh.picks {
		if t >= x.t {
			p = x
		}
	}
	return p
}

func (s *Style) selection(world *image.RGBA, sh *shot, t float64, items []item, ox, oy int) {
	p := sh.pickAt(t)
	if p.who == nil {
		return
	}
	for _, it := range items {
		if it.a != p.who && !p.group[it.a] {
			continue
		}
		ax, ay, im := s.anchor(it)
		b := s.boxes[it.a.kind]
		if it.flip {
			w := im.Bounds().Dx()
			b = image.Rect(w-b.Max.X, b.Min.Y, w-b.Min.X, b.Max.Y)
		}
		x0 := (it.x+ox)*px - ax
		y0 := mapTop + (it.y+oy)*px - ay
		b = b.Add(image.Pt(x0, y0))
		sel := s.art.sel
		sw, shh := sel.Bounds().Dx(), sel.Bounds().Dy()
		var top, cx int
		if b.Dx() <= sw+12 && b.Dy() <= shh+12 {
			cx = (b.Min.X + b.Max.X) / 2
			cy := (b.Min.Y + b.Max.Y) / 2
			x := floorTo(cx - sw/2)
			y := floorTo(cy - shh/2)
			draw.Draw(world, image.Rect(x, y, x+sw, y+shh), sel, image.Point{}, draw.Over)
			top = y
		} else {
			r := b.Inset(-px)
			l := 3 * px
			ink := s.art.selInk
			for _, c := range [][2]int{{r.Min.X, r.Min.Y}, {r.Max.X - l, r.Min.Y}, {r.Min.X, r.Max.Y - px}, {r.Max.X - l, r.Max.Y - px}} {
				fill(world, image.Rect(c[0], c[1], c[0]+l, c[1]+px), ink)
			}
			for _, c := range [][2]int{{r.Min.X, r.Min.Y}, {r.Max.X - px, r.Min.Y}, {r.Min.X, r.Max.Y - l}, {r.Max.X - px, r.Max.Y - l}} {
				fill(world, image.Rect(c[0], c[1], c[0]+px, c[1]+l), ink)
			}
			cx, top = (r.Min.X+r.Max.X)/2, r.Min.Y
		}
		if p.group != nil {
			continue
		}
		bx := cx - 5*px
		by := top - 2*px
		fill(world, image.Rect(bx, by, bx+10*px, by+px), hpOff)
		fill(world, image.Rect(bx, by, bx+int(math.Round(p.hp*10))*px, by+px), hpOn)
		return
	}
}

func (s *Style) marks(world *image.RGBA, sh *shot, t float64, items []item, ox, oy int) {
	for _, it := range items {
		if it.moving || len(it.a.marks) == 0 || sh.tallyAt(it.a, t) != nil {
			continue
		}
		m, at := last(it.a.marks, t)
		frames := s.art.marks[m]
		if len(frames) == 0 {
			continue
		}
		im := frames[int((t-at)*4+it.a.anim*4)%len(frames)]
		spec := s.set.Specs[it.a.kind]
		dx := (spec.Top[0] - spec.Anchor[0]) / 12
		if it.flip {
			dx = -dx
		}
		dy := (spec.Anchor[1] - spec.Top[1]) / 12
		w, h := im.Bounds().Dx(), im.Bounds().Dy()
		x := (it.x+dx+ox)*px - w/2
		y := mapTop + (it.y-dy-2+oy)*px - h
		draw.Draw(world, image.Rect(x, y, x+w, y+h), im, image.Point{}, draw.Over)
	}
}

func text(dc *gg.Context, face font.Face, s string, x, y float64, c color.RGBA, shadow bool) {
	dc.SetFontFace(face)
	if shadow {
		dc.SetColor(black)
		dc.DrawString(s, x+4, y+4)
	}
	dc.SetColor(c)
	dc.DrawString(s, x, y)
}

func fit(face func(float64) font.Face, width int, lines ...string) font.Face {
	for _, size := range []float64{32, 24, 16} {
		f := face(size)
		ok := true
		for _, l := range lines {
			ok = ok && font.MeasureString(f, l).Ceil() <= width
		}
		if ok {
			return f
		}
	}
	return face(16)
}

func (s *Style) message(dc *gg.Context, face func(float64) font.Face, sh *shot, t float64) {
	var m *message
	for i := range sh.msgs {
		if t >= sh.msgs[i].t && t < sh.msgs[i].until {
			m = &sh.msgs[i]
		}
	}
	if m == nil {
		return
	}
	d := t - m.t
	if d < 0.6 && int(d*8)%2 == 1 {
		return
	}
	f := fit(face, render.W-80, m.lines...)
	asc := float64(f.Metrics().Ascent.Ceil())
	for i, l := range m.lines {
		text(dc, f, l, 40, 92+asc+float64(i*44), s.palette["msg"], true)
	}
}

func (s *Style) resText(key string, t float64) (string, bool, string) {
	var cur *resStep
	steps := s.res[key]
	for i := range steps {
		if t >= steps[i].t {
			cur = &steps[i]
		}
	}
	if cur == nil {
		return "", false, ""
	}
	if cur.count == nil {
		return cur.text, cur.alert, cur.icon
	}
	k := 1.0
	if cur.count[2] > 0 {
		k = clamp((t - cur.t) / cur.count[2])
	}
	n := int(math.Round(cur.count[0] + (cur.count[1]-cur.count[0])*k))
	return strings.ReplaceAll(cur.text, "{n}", strconv.Itoa(n)), cur.alert, cur.icon
}

func (s *Style) hud(dst *image.RGBA, dc *gg.Context, face func(float64) font.Face, sh *shot, t float64, items []item) {
	if n := len(sh.keys); n > 0 {
		w := (render.W - 32) / n
		for i, k := range sh.keys {
			x := 16 + i*w
			txt, alert, icon := s.resText(k, t)
			if txt == "" {
				continue
			}
			ic := s.art.glyphs[icon]
			ix, iy := x+(iconW-ic.Bounds().Dx())/2, 6+(iconW-ic.Bounds().Dy())/2
			draw.Draw(dst, image.Rect(ix, iy, ix+ic.Bounds().Dx(), iy+ic.Bounds().Dy()), ic, image.Point{}, draw.Over)
			col := s.palette["res_ink"]
			if alert {
				col = s.palette["res_bad"]
			}
			text(dc, fit(face, w-56, txt), txt, float64(x+44), 38, col, false)
		}
	}
	p := sh.pickAt(t)
	if p.who != nil {
		fr := s.art.frame
		draw.Draw(dst, image.Rect(328, 1472, 328+fr.Bounds().Dx(), 1472+fr.Bounds().Dy()), fr, image.Point{}, draw.Src)
		role, _ := last(p.who.roles, t)
		if im := s.looks[p.who.kind+"/"+role]; im != nil {
			w, h := im.Bounds().Dx(), im.Bounds().Dy()
			x, y := 388-w/2, 1532-h/2
			draw.Draw(dst, image.Rect(x, y, x+w, y+h), im, image.Point{}, draw.Over)
		}
		if n := len(p.group); n > 0 {
			c := "×" + strconv.Itoa(n)
			f := face(16)
			text(dc, f, c, float64(328+fr.Bounds().Dx()-12-font.MeasureString(f, c).Ceil()), float64(1472+fr.Bounds().Dy()-12), white, true)
		}
		fill(dst, image.Rect(342, 1604, 434, 1620), frameInk[0])
		fill(dst, image.Rect(346, 1608, 430, 1616), hpOff)
		fill(dst, image.Rect(346, 1608, 346+int(math.Round(p.hp*21))*4, 1616), hpOn)
		status := p.status
		if !p.fixed {
			status, _ = last(p.src, t)
		}
		f := fit(face, 1040-472, p.name, status)
		text(dc, f, p.name, 472, 1520, white, false)
		text(dc, f, status, 472, 1584, toneInk[p.tone], false)
	}
	for i, b := range p.buttons {
		x, y := 312+i*124, 1672
		bg, ic, off := s.art.button, s.art.icons[b], 0
		if p.pressed[b] {
			bg, ic, off = s.art.pressed, s.art.lit[b], 4
		}
		draw.Draw(dst, image.Rect(x, y, x+bg.Bounds().Dx(), y+bg.Bounds().Dy()), bg, image.Point{}, draw.Src)
		ix, iy := x+(bg.Bounds().Dx()-ic.Bounds().Dx())/2+off, y+(bg.Bounds().Dy()-ic.Bounds().Dy())/2+off
		draw.Draw(dst, image.Rect(ix, iy, ix+ic.Bounds().Dx(), iy+ic.Bounds().Dy()), ic, image.Point{}, draw.Over)
	}
	mw := int(math.Round(float64(mapW) * 62 / mapH))
	x0 := minimapIn.Min.X + (minimapIn.Dx()-mw*4)/2
	clip := image.Rect(x0+4, minimapIn.Min.Y+4, x0+mw*4-4, minimapIn.Max.Y-4)
	for _, it := range items {
		mx := x0 + int(float64(it.x)*float64(mw)/mapW)*4
		my := minimapIn.Min.Y + int(float64(it.y)*62/mapH)*4
		sz := 4
		if !it.a.unit {
			sz = 8
			mx, my = mx-4, my-4
		}
		role, _ := last(it.a.roles, t)
		fill(dst, image.Rect(mx, my, mx+sz, my+sz).Intersect(clip), s.roles[role])
	}
}

func (s *Style) flash(dst *image.RGBA, sh *shot, t float64) {
	k, col := 0.0, white
	for i, x := range s.shots {
		if x.in.color == "none" {
			continue
		}
		d, h := math.Abs(t-x.at), x.in.dur/2
		if i == 0 {
			d, h = t-x.at, x.in.dur
			if d < 0 {
				continue
			}
		}
		if d < h && 1-d/h > k {
			k = 1 - d/h
			col = white
			if x.in.color == "black" {
				col = black
			}
		}
	}
	for _, x := range sh.shocks {
		if d := t - x.t; d >= 0 && d < 0.12 && 1-d/0.12 > k {
			k, col = 1-d/0.12, white
		}
	}
	if k <= 0 {
		return
	}
	c := color.RGBA{uint8(float64(col.R) * k), uint8(float64(col.G) * k), uint8(float64(col.B) * k), uint8(255 * k)}
	draw.Draw(dst, dst.Bounds(), image.NewUniform(c), image.Point{}, draw.Over)
}
