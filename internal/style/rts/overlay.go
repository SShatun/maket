package rts

import (
	"fmt"
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
	"maket/internal/scene"
)

const (
	grain  = 3
	border = 2 * grain
	pad    = 2 * grain
	iconW  = 36
)

var (
	shade  = color.RGBA{0, 0, 0, 150}
	dimInk = color.RGBA{214, 208, 194, 255}
	fills  = map[string]color.RGBA{"": {170, 162, 150, 255}, "card": {170, 162, 150, 255}, "accent": {214, 172, 80, 255}, "error": {178, 66, 56, 255}}
)

type tag struct {
	who   *actor
	at    image.Point
	below bool
	im    *image.RGBA
}

type tally struct {
	who      *actor
	t, until float64
	steps    []float64
	ims      []*image.RGBA
	bars     []image.Rectangle
	badge    string
}

type layer struct {
	t  float64
	im *image.RGBA
}

type element struct {
	t, until float64
	at       image.Point
	layers   []layer
	cells    []image.Point
	ink      color.RGBA
}

func (s *Style) face(size float64) font.Face {
	if f, ok := s.faces[size]; ok {
		return f
	}
	f := render.Face(s.fonts.Pixel, size)
	s.faces[size] = f
	return f
}

func (s *Style) width(size float64, txt string) int {
	return font.MeasureString(s.face(size), txt).Ceil()
}

func (s *Style) write(im *image.RGBA, size float64, txt string, x, y int, c color.RGBA) {
	dc := gg.NewContextForRGBA(im)
	dc.SetFontFace(s.face(size))
	sh := math.Max(grain, size/8)
	dc.SetColor(black)
	dc.DrawString(txt, float64(x)+sh, float64(y)+sh)
	dc.SetColor(c)
	dc.DrawString(txt, float64(x), float64(y))
}

func (s *Style) ink(tone string, def color.RGBA) color.RGBA {
	switch tone {
	case "accent":
		return s.palette["msg"]
	case "ok":
		return toneInk["ok"]
	case "error":
		return toneInk["bad"]
	case "ink":
		return white
	}
	return def
}

func (s *Style) slab(w, h int, tint color.RGBA) *image.RGBA {
	im := image.NewRGBA(image.Rect(0, 0, w+border, h+border))
	fill(im, image.Rect(border, border, w+border, h+border), shade)
	r := image.Rect(0, 0, w, h)
	tile(im, r, s.art.chip)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := im.PixOffset(x, y)
			im.Pix[i] = uint8(int(im.Pix[i]) * int(tint.R) / 255)
			im.Pix[i+1] = uint8(int(im.Pix[i+1]) * int(tint.G) / 255)
			im.Pix[i+2] = uint8(int(im.Pix[i+2]) * int(tint.B) / 255)
		}
	}
	outline(im, r, grain, frameInk[0])
	in := r.Inset(grain)
	fill(im, image.Rect(in.Min.X, in.Min.Y, in.Max.X, in.Min.Y+grain), bevelLight)
	fill(im, image.Rect(in.Min.X, in.Min.Y, in.Min.X+grain, in.Max.Y), bevelLight)
	fill(im, image.Rect(in.Min.X+grain, in.Max.Y-grain, in.Max.X, in.Max.Y), bevelDark)
	fill(im, image.Rect(in.Max.X-grain, in.Min.Y+grain, in.Max.X, in.Max.Y), bevelDark)
	return im
}

func (s *Style) labelPlate(txt string) *image.RGBA {
	size := 24.0
	if s.width(size, txt) > 504 {
		size = 16
	}
	tw := s.width(size, txt)
	im := s.slab(tw+2*(border+pad)+grain, int(size)+2*(border+pad)+grain, fills["card"])
	s.write(im, size, txt, border+pad, border+pad+int(size), white)
	return im
}

func labelPos(c scene.Cast) (bool, error) {
	switch c.LabelPos {
	case "", "above":
		return false, nil
	case "below":
		return true, nil
	}
	return false, fmt.Errorf("cast %s: label_pos wants above or below, got %q", c.ID, c.LabelPos)
}

func (s *Style) entityTag(sh *shot, a *actor, c scene.Cast) error {
	if c.LabelPos == "" {
		return nil
	}
	below, err := labelPos(c)
	if err != nil {
		return err
	}
	if c.Label == "" {
		return fmt.Errorf("cast %s: label_pos needs label", c.ID)
	}
	sh.tags = append(sh.tags, tag{who: a, below: below, im: s.labelPlate(c.Label)})
	return nil
}

func (s *Style) crowdTag(sh *shot, cr *crowd, c scene.Cast) error {
	if c.LabelPos == "" {
		return nil
	}
	below, err := labelPos(c)
	if err != nil {
		return err
	}
	if c.Label == "" {
		return fmt.Errorf("crowd %s: label_pos needs label", cr.title)
	}
	x0, y0, x1, y1 := cr.box()
	if math.IsInf(x0, 1) {
		return nil
	}
	spec := s.set.Specs[cr.kind]
	b := s.box(cr.kind)
	ay := spec.Anchor[1] / 2
	at := image.Pt(int(math.Round((x0+x1)/2))*px, mapTop+int(math.Round(y0))*px-(ay-b.Min.Y))
	if below {
		at.Y = mapTop + int(math.Round(y1))*px + (b.Max.Y - ay)
	} else {
		for _, u := range cr.units {
			if len(u.marks) > 0 {
				at.Y -= 72 + 2*px
				break
			}
		}
	}
	sh.tags = append(sh.tags, tag{at: at, below: below, im: s.labelPlate(c.Label)})
	return nil
}

func (s *Style) tallyPlate(txt, badge string) (*image.RGBA, image.Rectangle) {
	const size = 32
	tw := s.width(size, txt)
	x, ch := border+pad, size+4*grain
	if badge != "" {
		ic := s.art.small[badge][0].Bounds()
		x, ch = border+pad+ic.Dx()+3*grain, max(ch, ic.Dy())
	}
	im := s.slab(x+tw+grain+border+pad, ch+2*(border+pad), fills["card"])
	y := border + pad + (ch-size-4*grain)/2
	s.write(im, size, txt, x, y+size, white)
	return im, image.Rect(x, y+size+2*grain, x+tw, y+size+4*grain)
}

func (s *Style) tries(sh *shot, ev scene.Event, re rawEvent, t float64) error {
	if ev.Target == nil || sh.byID[ev.Target.ID] == nil {
		return fmt.Errorf("tries needs target: id of one cast entity in this shot")
	}
	a := sh.byID[ev.Target.ID]
	badge := "wait"
	if re.Badge != nil {
		badge = *re.Badge
	}
	if badge == "bad" {
		badge = "bang"
	}
	if badge != "" && len(s.art.small[badge]) == 0 {
		return fmt.Errorf("unknown badge %q, known: bang, question, wait, dots, zzz", badge)
	}
	text := ev.Text
	if text == "" {
		text = "{n}"
	}
	var vals []string
	var steps []float64
	if c := re.Count; c != nil {
		if len(c) != 3 || c[2] < 0 {
			return fmt.Errorf("tries: count wants [from, to, seconds]")
		}
		if !strings.Contains(text, "{n}") {
			text = "{n}" + text
		}
		from, to := int(math.Round(c[0])), int(math.Round(c[1]))
		k, dir := to-from, 1
		if k < 0 {
			k, dir = -k, -1
		}
		for i := 0; i <= k; i++ {
			ti := t
			if k > 0 {
				ti = t + float64(i)*c[2]/float64(k)
			}
			vals = append(vals, strings.ReplaceAll(text, "{n}", strconv.Itoa(from+i*dir)))
			steps = append(steps, ti)
		}
	} else if strings.Contains(text, "{n}") {
		return fmt.Errorf("tries: {n} in text needs count")
	} else {
		vals, steps = []string{text}, []float64{t}
	}
	until := sh.until
	if ev.Until != nil {
		u, err := s.when(*ev.Until)
		if err != nil {
			return err
		}
		until = math.Min(until, u)
	}
	x := tally{who: a, t: t, until: until, steps: steps, badge: badge}
	for _, v := range vals {
		im, bar := s.tallyPlate(v, badge)
		x.ims, x.bars = append(x.ims, im), append(x.bars, bar)
	}
	sh.tallies = append(sh.tallies, x)
	return nil
}

func inMap(x, y float64) bool { return x >= 0 && x <= mapW && y >= 0 && y <= mapH }

func snap(v float64) int { return int(math.Floor(v/grain)) * grain }

func (s *Style) element(sh *shot, ev scene.Event, t, until float64) error {
	e := element{t: t, until: until}
	switch ev.Do {
	case "text":
		if !inMap(ev.Pos[0], ev.Pos[1]) {
			return fmt.Errorf("text pos wants map px inside 180x229, got %v", ev.Pos)
		}
		size := ev.Size
		if size == 0 {
			size = 24
		}
		size = math.Max(8, math.Round(size/8)*8)
		tw, d := s.width(size, ev.Text), int(math.Max(grain, size/8))
		im := image.NewRGBA(image.Rect(0, 0, tw+d, int(size)+d))
		s.write(im, size, ev.Text, 0, int(size), s.ink(ev.Tone, white))
		e.at = image.Pt(snap(ev.Pos[0]*px-float64(tw)/2), snap(mapTop+ev.Pos[1]*px-size/2))
		e.layers = []layer{{t, im}}
	case "panel":
		b := ev.Box
		if !inMap(b[0], b[1]) || !inMap(b[2], b[3]) {
			return fmt.Errorf("panel box wants map px inside 180x229, got %v", b)
		}
		r := image.Rect(int(math.Round(b[0]))*px, mapTop+int(math.Round(b[1]))*px, int(math.Round(b[2]))*px, mapTop+int(math.Round(b[3]))*px)
		times := []float64{t}
		shows := make([]float64, len(ev.Lines))
		for i, l := range ev.Lines {
			shows[i] = t
			if l.Appear != nil {
				ta, err := s.when(*l.Appear)
				if err != nil {
					return err
				}
				shows[i] = math.Max(t, ta)
				times = append(times, shows[i])
			}
		}
		sort.Float64s(times)
		for i, ti := range times {
			if i > 0 && ti == times[i-1] {
				continue
			}
			var vis []scene.PanelLine
			for j, l := range ev.Lines {
				if shows[j] <= ti {
					vis = append(vis, l)
				}
			}
			e.layers = append(e.layers, layer{ti, s.panelImage(ev, r.Dx(), r.Dy(), vis)})
		}
		e.at = r.Min
	case "line":
		f, to := ev.From.Coords, ev.To.Coords
		if !inMap(f[0], f[1]) || !inMap(to[0], to[1]) {
			return fmt.Errorf("line from and to want map px inside 180x229")
		}
		e.cells = cells(f[0], f[1], to[0], to[1], ev.Kind == "dash")
		if ev.Kind == "arrow" {
			a := math.Atan2(to[1]-f[1], to[0]-f[0])
			for _, side := range []float64{-1, 1} {
				e.cells = append(e.cells, cells(to[0], to[1], to[0]-4*math.Cos(a+side*0.7), to[1]-4*math.Sin(a+side*0.7), false)...)
			}
		}
		e.ink = s.ink(ev.Tone, white)
	}
	sh.elems = append(sh.elems, e)
	return nil
}

func cells(x0, y0, x1, y1 float64, dash bool) []image.Point {
	d := math.Hypot(x1-x0, y1-y0)
	n := max(1, int(math.Ceil(d)))
	var out []image.Point
	for i := 0; i <= n; i++ {
		u := float64(i) / float64(n)
		if dash && int(d*u/3)%2 != 0 {
			continue
		}
		p := image.Pt(int(math.Round(x0+(x1-x0)*u)), int(math.Round(y0+(y1-y0)*u)))
		if len(out) == 0 || out[len(out)-1] != p {
			out = append(out, p)
		}
	}
	return out
}

func (s *Style) panelImage(ev scene.Event, w, h int, lines []scene.PanelLine) *image.RGBA {
	im := s.slab(w, h, fills[ev.Fill])
	if ev.Tone != "" {
		outline(im, image.Rect(0, 0, w, h), grain, s.ink(ev.Tone, white))
	}
	inner := w - 2*(border+pad) - grain
	ts := 24.0
	if ev.Title != "" && s.width(ts, ev.Title) > inner {
		ts = 16
	}
	ls := 24.0
	avail := float64(h - 2*(border+pad))
	if ev.Title != "" {
		avail -= ts + 12
	}
	for _, l := range ev.Lines {
		if s.width(ls, l.Text) > inner {
			ls = 16
		}
	}
	if n := float64(len(ev.Lines)); n > 0 && n*ls*1.5 > avail+ls/2 {
		ls = 16
	}
	y := border + pad
	if ev.Title != "" {
		if len(ev.Lines) == 0 {
			y = (h - int(ts)) / 2
		}
		tw := s.width(ts, ev.Title)
		s.write(im, ts, ev.Title, max(border+pad, (w-tw)/2), y+int(ts), s.ink(ev.Tone, white))
		y += int(ts) + 12
	}
	for _, l := range lines {
		s.write(im, ls, l.Text, border+pad, y+int(ls), s.ink(l.Tone, dimInk))
		y += int(ls * 1.5)
	}
	return im
}

func (sh *shot) tallyAt(a *actor, t float64) *tally {
	var cur *tally
	for i := range sh.tallies {
		x := &sh.tallies[i]
		if x.who == a && t >= x.t && t < x.until && (cur == nil || x.t >= cur.t) {
			cur = x
		}
	}
	return cur
}

func paste(dst *image.RGBA, im *image.RGBA, x, y int) {
	draw.Draw(dst, image.Rect(x, y, x+im.Bounds().Dx(), y+im.Bounds().Dy()), im, image.Point{}, draw.Over)
}

func (s *Style) plates(world *image.RGBA, sh *shot, t float64, items []item, ox, oy int) {
	if len(sh.tags) == 0 && len(sh.tallies) == 0 {
		return
	}
	tops := map[*actor]int{}
	for _, it := range items {
		x := sh.tallyAt(it.a, t)
		if x == nil {
			continue
		}
		spec := s.set.Specs[it.a.kind]
		dx := (spec.Top[0] - spec.Anchor[0]) / 12
		if it.flip {
			dx = -dx
		}
		dy := (spec.Anchor[1] - spec.Top[1]) / 12
		i := 0
		for i+1 < len(x.steps) && t >= x.steps[i+1] {
			i++
		}
		im := x.ims[i]
		w, h := im.Bounds().Dx()-border, im.Bounds().Dy()-border
		px0 := snap(float64((it.x+dx+ox)*px - w/2))
		py0 := mapTop + (it.y-dy-2+oy)*px - h
		paste(world, im, px0, py0)
		if x.badge != "" {
			frames := s.art.small[x.badge]
			ic := frames[int((t-x.t)*4+it.a.anim*4)%len(frames)]
			paste(world, ic, px0+border+pad, py0+(h-ic.Bounds().Dy())/2)
		}
		if x.badge == "wait" && i+1 < len(x.steps) {
			bar := x.bars[i].Add(image.Pt(px0, py0))
			k := 1 - clamp((t-x.steps[i])/(x.steps[i+1]-x.steps[i]))
			fill(world, bar, well)
			fill(world, image.Rect(bar.Min.X, bar.Min.Y, bar.Min.X+snap(k*float64(bar.Dx())), bar.Max.Y), s.palette["msg"])
		}
		tops[it.a] = py0
	}
	for _, tg := range sh.tags {
		w, h := tg.im.Bounds().Dx()-border, tg.im.Bounds().Dy()-border
		if tg.who == nil {
			x, y := tg.at.X+ox*px-w/2, tg.at.Y+oy*px-h-px
			if tg.below {
				y = tg.at.Y + oy*px + px
			}
			paste(world, tg.im, snap(float64(x)), y)
			continue
		}
		for _, it := range items {
			if it.a != tg.who {
				continue
			}
			ax, ay, im := s.anchor(it)
			b := s.boxes[it.a.kind]
			if it.flip {
				bw := im.Bounds().Dx()
				b = image.Rect(bw-b.Max.X, b.Min.Y, bw-b.Min.X, b.Max.Y)
			}
			b = b.Add(image.Pt((it.x+ox)*px-ax, mapTop+(it.y+oy)*px-ay))
			x := snap(float64((b.Min.X+b.Max.X)/2 - w/2))
			y := b.Max.Y + px
			if !tg.below {
				top := b.Min.Y
				if v, ok := tops[it.a]; ok {
					top = min(top, v)
				}
				y = top - px - h
			}
			paste(world, tg.im, x, y)
			break
		}
	}
}

func (s *Style) elements(world *image.RGBA, sh *shot, t float64) {
	for _, e := range sh.elems {
		if t < e.t || t >= e.until {
			continue
		}
		if e.cells != nil {
			for _, c := range e.cells {
				x, y := c.X*px+grain, mapTop+c.Y*px+grain
				fill(world, image.Rect(x, y, x+px, y+px), black)
			}
			for _, c := range e.cells {
				x, y := c.X*px, mapTop+c.Y*px
				fill(world, image.Rect(x, y, x+px, y+px), e.ink)
			}
			continue
		}
		im := e.layers[0].im
		for _, l := range e.layers {
			if t >= l.t {
				im = l.im
			}
		}
		paste(world, im, e.at.X, e.at.Y)
	}
}
