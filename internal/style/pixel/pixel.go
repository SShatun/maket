package pixel

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"

	"github.com/fogleman/gg"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"

	"maket/internal/audio"
	"maket/internal/charkit"
	"maket/internal/render"
	"maket/internal/scene"
	"maket/internal/style"
	"maket/internal/timeline"
)

var roleColor = map[string]color.RGBA{
	"external": {255, 255, 255, 255},
	"producer": {0, 135, 81, 255},
	"consumer": {131, 118, 156, 255},
	"buffer":   {255, 163, 0, 255},
	"storage":  {41, 173, 255, 255},
	"error":    {255, 0, 77, 255},
	"ok":       {0, 228, 54, 255},
}

var (
	dot   = color.RGBA{44, 60, 108, 255}
	floor = color.RGBA{12, 18, 40, 255}
	grey  = color.RGBA{194, 195, 199, 255}
	black = color.RGBA{0, 0, 0, 255}
)

type timedStr struct {
	t float64
	v string
}

type timedState struct {
	t      float64
	v      string
	hold   bool
	silent bool
}

type ent struct {
	id, kind, label string
	x, y            float64
	appear, gone    float64
	size            float64
	from            [2]float64
	splitDur        float64
	cut             bool
	walks           []walk
	roles           []timedStr
	states          []timedState
	status          []timedStr
	badges          []timedStr
	shakes          []float64
}

type walk struct {
	t, dur   float64
	from, to [2]float64
}

type shot struct {
	at, until float64
	place     scene.Place
}

type move struct {
	t, dur, until  float64
	from, to       [2]float64
	label          string
	fromEnt, toEnt *ent
}

type say struct {
	t, until float64
	who      *ent
	text     string
	dx       float64
}

type focus struct {
	t, until float64
	targets  map[string]bool
}

type Pixel struct {
	theme   render.Theme
	sprites *sprites
	bg      *image.RGBA
	ents    []*ent
	byID    map[string]*ent
	moves   []move
	says    []say
	focus   *focus
	focuses []*focus
	shots   []shot
	cues    []audio.Cue
	dur     float64
	roles   map[string]color.RGBA
	dot     color.RGBA
}

func Colors(dir string) (map[string]color.RGBA, map[string]color.RGBA, error) {
	lk, err := charkit.ReadLook(dir)
	if err != nil {
		return nil, nil, err
	}
	roles := make(map[string]color.RGBA, len(roleColor)+len(lk.Roles))
	for k, v := range roleColor {
		roles[k] = v
	}
	for k, v := range lk.Roles {
		roles[k] = v
	}
	return roles, lk.Theme, nil
}

func New(root, look string, fonts *render.Fonts) (*Pixel, error) {
	dir := charkit.Dir(root, look)
	roles, over, err := Colors(dir)
	if err != nil {
		return nil, err
	}
	badges := map[string]image.Image{}
	for _, b := range []string{"wait", "ok", "bad", "question"} {
		path := filepath.Join(dir, "badges", "badge-"+b+".png")
		if _, err := os.Stat(path); err != nil {
			path = filepath.Join(root, "assets", "pixel-static", "badge-"+b+".png")
		}
		im, err := charkit.ReadPNG(path)
		if err != nil {
			return nil, err
		}
		badges[b] = im
	}
	sp, err := loadSprites(dir, roles)
	if err != nil {
		return nil, err
	}
	d := dot
	if c, ok := over["dot"]; ok {
		d = c
	}
	return &Pixel{theme: render.PixelTheme(fonts, badges, over), sprites: sp, roles: roles, dot: d}, nil
}

func (p *Pixel) Theme() render.Theme { return p.theme }

func (p *Pixel) Sounds() []audio.Cue { return p.cues }

func (p *Pixel) Layout(sc *scene.Scene, tl *timeline.Timeline) error {
	p.dur = tl.Duration
	p.byID = map[string]*ent{}
	at := tl.MustResolve
	if len(sc.Shots) == 0 {
		for i, e := range sc.Entities {
			if err := p.entity(e, 1.4+0.1*float64(i), math.Inf(1), 1, "", false, tl); err != nil {
				return err
			}
		}
		for i, ev := range sc.Events {
			if err := p.event(i, ev, at(ev.At), math.Inf(1), tl); err != nil {
				return err
			}
		}
	} else {
		starts, err := tl.ShotStarts(sc)
		if err != nil {
			return err
		}
		rng := rand.New(rand.NewSource(int64(sc.Seed)))
		for i, s := range sc.Shots {
			end := tl.Duration
			if i+1 < len(starts) {
				end = starts[i+1]
			}
			p.shots = append(p.shots, shot{starts[i], end, s.Place})
			p.byID = map[string]*ent{}
			for _, c := range s.Cast {
				cast := []scene.Cast{c}
				if c.Crowd != nil {
					cast = crowd(c.Crowd, rng)
				}
				for _, c := range cast {
					size := c.Size
					if size == 0 {
						size = 1
					}
					if err := p.entity(c.Entity, starts[i], end, size, c.State, true, tl); err != nil {
						return fmt.Errorf("shot %d: %w", i+1, err)
					}
				}
			}
			events := append([]scene.Event{}, s.Do...)
			sort.SliceStable(events, func(i, j int) bool { return at(events[i].At) < at(events[j].At) })
			for j, ev := range events {
				if err := p.event(j, ev, at(ev.At), end, tl); err != nil {
					return fmt.Errorf("shot %d: %w", i+1, err)
				}
			}
		}
	}
	for _, x := range p.ents {
		sort.SliceStable(x.roles, func(i, j int) bool { return x.roles[i].t < x.roles[j].t })
		sort.SliceStable(x.states, func(i, j int) bool { return x.states[i].t < x.states[j].t })
		roles := map[string]bool{}
		for _, r := range x.roles {
			if _, ok := p.roles[r.v]; !ok {
				return fmt.Errorf("entity %s: unknown role %q", x.id, r.v)
			}
			roles[r.v] = true
		}
		if err := p.sprites.prepare(x.kind, roles); err != nil {
			return fmt.Errorf("entity %s: %w", x.id, err)
		}
		for _, b := range x.badges {
			if _, ok := p.theme.Badges[b.v]; !ok && b.v != "" {
				return fmt.Errorf("entity %s: unknown badge %q", x.id, b.v)
			}
		}
		for _, s := range x.states {
			if _, ok := p.sprites.specs[x.kind].States[s.v]; !ok {
				return fmt.Errorf("entity %s: %s has no state %q", x.id, x.kind, s.v)
			}
		}
	}
	p.bg = background(p.theme.BG, p.dot)
	p.cues = p.sounds(sc, tl)
	return nil
}

func (p *Pixel) entity(e scene.Entity, appear, gone, size float64, state string, cut bool, tl *timeline.Timeline) error {
	at := tl.MustResolve
	if len(e.At) != 2 {
		return fmt.Errorf("entity %s: pixel style wants at: [x, y] in px", e.ID)
	}
	x := &ent{id: e.ID, kind: e.Kind, label: e.Label, x: e.At[0], y: e.At[1], appear: appear, gone: gone, size: size, cut: cut}
	if cut {
		if state == "" {
			state = p.sprites.specs[e.Kind].Rest
		}
		x.states = append(x.states, timedState{t: appear, v: state, silent: true})
	}
	if e.Appear != nil {
		x.appear = at(*e.Appear)
	}
	role := e.Role
	if role == "" {
		role = "external"
	}
	x.roles = append(x.roles, timedStr{0, role})
	for _, r := range e.Roles {
		x.roles = append(x.roles, timedStr{at(r.At), r.V})
	}
	for _, s := range e.States {
		x.states = append(x.states, timedState{t: at(s.At), v: s.V, hold: s.Hold})
	}
	for _, s := range e.Status {
		x.status = append(x.status, timedStr{at(s.At), s.V})
	}
	for _, s := range e.Badges {
		x.badges = append(x.badges, timedStr{at(s.At), s.V})
	}
	p.ents = append(p.ents, x)
	if e.ID != "" {
		p.byID[e.ID] = x
	}
	return nil
}

func (p *Pixel) event(i int, ev scene.Event, t, end float64, tl *timeline.Timeline) error {
	target := func() (*ent, error) {
		if ev.Target == nil || p.byID[ev.Target.ID] == nil {
			return nil, fmt.Errorf("event %d: %s needs a target entity", i+1, ev.Do)
		}
		return p.byID[ev.Target.ID], nil
	}
	until := func(def float64) float64 {
		if ev.Until != nil {
			return math.Min(end, tl.MustResolve(*ev.Until))
		}
		return math.Min(end, def)
	}
	switch ev.Do {
	case "move":
		if ev.From == nil || ev.To == nil {
			return fmt.Errorf("event %d: move needs from and to", i+1)
		}
		dur := ev.Dur
		if dur == 0 {
			dur = 0.6
		}
		from, err := p.point(*ev.From, t)
		if err != nil {
			return fmt.Errorf("event %d: %w", i+1, err)
		}
		to, err := p.point(*ev.To, t+dur)
		if err != nil {
			return fmt.Errorf("event %d: %w", i+1, err)
		}
		label := ev.Label
		if label == "" {
			label = "git"
		}
		p.moves = append(p.moves, move{t: t, dur: dur, until: until(t + dur), from: from, to: to, label: label, fromEnt: p.byID[ev.From.ID], toEnt: p.byID[ev.To.ID]})
	case "walk":
		if len(p.shots) == 0 {
			return fmt.Errorf("event %d: walk needs shots", i+1)
		}
		x, err := target()
		if err != nil {
			return err
		}
		px, py := x.position(t)
		x.walks = append(x.walks, walk{t: t, dur: ev.Dur, from: [2]float64{px, py}, to: [2]float64{ev.To.Coords[0], ev.To.Coords[1]}})
	case "zoom":
		if len(p.shots) == 0 {
			return fmt.Errorf("event %d: zoom needs shots", i+1)
		}
	case "say":
		who, err := target()
		if err != nil {
			return err
		}
		p.says = append(p.says, say{t: t, until: until(t + 2), who: who, text: ev.Text, dx: ev.Dx})
	case "state":
		x, err := target()
		if err != nil {
			return err
		}
		x.states = append(x.states, timedState{t: t, v: ev.State, hold: ev.Hold})
	case "role":
		x, err := target()
		if err != nil {
			return err
		}
		x.roles = append(x.roles, timedStr{t, ev.Role})
		if ev.Until != nil {
			x.roles = append(x.roles, timedStr{tl.MustResolve(*ev.Until), x.roles[0].v})
		}
	case "shake":
		x, err := target()
		if err != nil {
			return err
		}
		x.shakes = append(x.shakes, t)
	case "split":
		src, err := target()
		if err != nil {
			return err
		}
		if len(ev.Into) != 2 {
			return fmt.Errorf("event %d: split needs two halves", i+1)
		}
		dur := ev.Dur
		if dur == 0 {
			dur = 0.6
		}
		src.shakes = append(src.shakes, t)
		gone := t + 0.45
		if src.cut {
			gone = math.Min(src.gone, gone)
		}
		src.gone = gone
		for _, id := range ev.Into {
			h := p.byID[id]
			if h == nil {
				return fmt.Errorf("event %d: unknown entity %q", i+1, id)
			}
			h.appear = src.gone
			if src.cut {
				h.size *= 0.75
				px, py := src.position(src.gone)
				h.from = [2]float64{px, py}
			} else {
				h.size = 0.75
				h.from = [2]float64{src.x, src.y}
			}
			h.splitDur = dur
		}
	case "focus":
		f := &focus{t: t, until: until(end), targets: map[string]bool{}}
		for _, pt := range ev.Targets {
			f.targets[pt.ID] = true
		}
		if ev.Target != nil {
			f.targets[ev.Target.ID] = true
		}
		if len(p.shots) > 0 {
			p.focuses = append(p.focuses, f)
		} else {
			p.focus = f
		}
	default:
		if !style.Common(ev.Do) {
			return fmt.Errorf("event %d: pixel style does not know %q", i+1, ev.Do)
		}
	}
	return nil
}

func (p *Pixel) point(pt scene.Point, t float64) ([2]float64, error) {
	if pt.ID == "" {
		if len(pt.Coords) != 2 {
			return [2]float64{}, fmt.Errorf("point wants [x, y] in px")
		}
		return [2]float64{pt.Coords[0], pt.Coords[1]}, nil
	}
	x, ok := p.byID[pt.ID]
	if !ok {
		return [2]float64{}, fmt.Errorf("unknown entity %q", pt.ID)
	}
	if !x.cut {
		return [2]float64{x.x, x.y - 180*scale}, nil
	}
	px, py := x.position(t)
	return [2]float64{px, py - 180*scale*x.size}, nil
}

func crowd(c *scene.Crowd, rng *rand.Rand) []scene.Cast {
	var out []scene.Cast
	for row := 0; row < c.Rows; row++ {
		n := c.Count / c.Rows
		if row < c.Count%c.Rows {
			n++
		}
		depth := float64(row) / float64(max(1, c.Rows-1))
		y := c.Box[1] + (c.Box[3]-c.Box[1])*depth
		size := c.Size[0] + (c.Size[1]-c.Size[0])*depth
		cell := (c.Box[2] - c.Box[0]) / float64(n)
		for col := 0; col < n; col++ {
			x := c.Box[0] + (float64(col)+0.5+(rng.Float64()-0.5)*0.2)*cell
			out = append(out, scene.Cast{Entity: scene.Entity{Kind: c.Kind, At: []float64{x, y}}, Size: size, State: c.State})
		}
	}
	return out
}

func (x *ent) position(t float64) (float64, float64) {
	px, py := x.x, x.y
	if x.splitDur > 0 {
		u := smooth(clamp((t - x.appear) / x.splitDur))
		px = x.from[0] + (x.x-x.from[0])*u
		py = x.from[1] + (x.y-x.from[1])*u
	}
	for _, w := range x.walks {
		if t < w.t {
			break
		}
		u := clamp((t - w.t) / w.dur)
		px = w.from[0] + (w.to[0]-w.from[0])*u
		py = w.from[1] + (w.to[1]-w.from[1])*u
		if u < 1 {
			py += 2 * math.Sin((t-w.t)*math.Pi*12) * math.Sin(u*math.Pi)
		}
	}
	return px, py
}

func background(bg, dot color.RGBA) *image.RGBA {
	im := image.NewRGBA(image.Rect(0, 0, render.W, render.H))
	draw.Draw(im, im.Bounds(), image.NewUniform(bg), image.Point{}, draw.Src)
	for y := 12; y < render.H; y += 36 {
		for x := 12; x < render.W; x += 36 {
			draw.Draw(im, image.Rect(x, y, x+4, y+4), image.NewUniform(dot), image.Point{}, draw.Src)
		}
	}
	return im
}

func (p *Pixel) Draw(dst *image.RGBA, t float64) {
	draw.Draw(dst, dst.Bounds(), p.bg, image.Point{}, draw.Src)
	for _, s := range p.shots {
		if t >= s.at && t < s.until {
			c := s.place.Wall
			draw.Draw(dst, dst.Bounds(), image.NewUniform(color.RGBA{c[0], c[1], c[2], 255}), image.Point{}, draw.Src)
			if s.place.Floor != nil {
				c = s.place.Ground
				draw.Draw(dst, image.Rect(0, int(*s.place.Floor), render.W, render.H), image.NewUniform(color.RGBA{c[0], c[1], c[2], 255}), image.Point{}, draw.Src)
			}
			break
		}
	}
	dc := gg.NewContextForRGBA(dst)
	sway := snap(-8 * math.Sin(t/p.dur*math.Pi))
	ents := p.ents
	if len(p.shots) > 0 {
		sway = 0
		ents = nil
		for _, x := range p.ents {
			if t >= x.appear && t < x.gone {
				ents = append(ents, x)
			}
		}
		sort.SliceStable(ents, func(i, j int) bool {
			_, yi := ents[i].position(t)
			_, yj := ents[j].position(t)
			return yi < yj
		})
	}
	label := render.Face(p.theme.Fonts.Pixel, 38*0.62)
	status := render.Face(p.theme.Fonts.Mono, 22)
	for _, x := range ents {
		p.drawEnt(dst, dc, x, t, sway, label, status)
	}
	p.drawMoves(dc, t, sway)
	p.drawSays(dc, t, sway)
	f := p.focus
	for _, current := range p.focuses {
		if t >= current.t && t < current.until {
			f = current
		}
	}
	if f != nil && t >= f.t {
		k := clamp((t - f.t) / 0.6)
		c := p.theme.BG
		dc.SetRGBA255(int(c.R), int(c.G), int(c.B), int(255*0.6*k))
		if len(p.shots) > 0 {
			dc.DrawRectangle(0, 0, render.W, render.H)
		} else {
			dc.DrawRectangle(0, 140, render.W, render.StageBottom-140)
		}
		dc.Fill()
		for _, x := range ents {
			if f.targets[x.id] {
				p.drawEnt(dst, dc, x, t, sway, label, status)
			}
		}
	}
}

func (p *Pixel) drawEnt(dst *image.RGBA, dc *gg.Context, x *ent, t, sway float64, label, status font.Face) {
	if t < x.appear || t >= x.gone {
		return
	}
	k := render.Pop(true, t-x.appear)
	if x.cut && x.splitDur == 0 {
		k = 1
	}
	if k <= 0 {
		return
	}
	spec := p.sprites.specs[x.kind]
	im := p.sprites.frame(x.kind, x.states, t, lastStr(x.roles, t))
	shx := 0.0
	for _, s := range x.shakes {
		if t >= s && t < s+0.45 {
			shx = snap(9 * math.Sin((t-s)*60) * (1 - (t-s)/0.45))
		}
	}
	px, py := x.position(t)
	w, h := float64(im.Bounds().Dx()), float64(im.Bounds().Dy())
	ay := py + sway
	foot := float64(spec.Size[1]-spec.Anchor[1]) * scale * x.size
	dc.SetColor(floor)
	dc.DrawRectangle(px-w*x.size*0.42, ay-6, w*x.size*0.84, 12)
	dc.Fill()
	sw, sh := w*k*x.size, h*k*x.size
	r := image.Rect(int(px-sw/2+shx), int(ay+foot-sh), int(px+sw/2+shx), int(ay+foot))
	if k == 1 && x.size == 1 {
		draw.Draw(dst, r, im, image.Point{}, draw.Over)
	} else {
		xdraw.NearestNeighbor.Scale(dst, r, im, im.Bounds(), xdraw.Over, nil)
	}
	if k != 1 {
		return
	}
	labelScale := 1.0
	if x.cut {
		labelScale = x.size
	}
	dc.SetFontFace(label)
	dc.SetColor(p.theme.Ink)
	dc.DrawStringAnchored(x.label, px, ay+40*labelScale, 0.5, 0.5)
	if s := lastStr(x.status, t); s != "" {
		dc.SetFontFace(status)
		dc.SetColor(grey)
		dc.DrawStringAnchored(s, px, ay+72*labelScale, 0.5, 0.5)
	}
	if b := p.theme.Badges[lastStr(x.badges, t)]; b != nil {
		ty := ay - float64(spec.Anchor[1]-spec.Top[1])*scale*x.size
		bx := int(px + w*x.size*0.3 + shx)
		if x.cut && x.size != 1 {
			by := int(ty + 6*x.size)
			bw, bh := int(float64(b.Bounds().Dx())*x.size), int(float64(b.Bounds().Dy())*x.size)
			r := image.Rect(bx-bw/2, by-bh/2, bx-bw/2+bw, by-bh/2+bh)
			xdraw.NearestNeighbor.Scale(dst, r, b, b.Bounds(), xdraw.Over, nil)
		} else {
			dc.DrawImageAnchored(b, bx, int(ty+6), 0.5, 0.5)
		}
	}
}

func (p *Pixel) drawMoves(dc *gg.Context, t, sway float64) {
	for _, m := range p.moves {
		if t < m.t || t >= m.until {
			continue
		}
		k := smooth(clamp((t - m.t) / m.dur))
		hop := -60 * math.Sin(k*math.Pi)
		from, to := m.from, m.to
		if m.fromEnt != nil && m.fromEnt.cut {
			px, py := m.fromEnt.position(m.t)
			from = [2]float64{px, py - 180*scale*m.fromEnt.size}
		}
		if m.toEnt != nil && m.toEnt.cut {
			px, py := m.toEnt.position(t)
			to = [2]float64{px, py - 180*scale*m.toEnt.size}
		}
		x := snap(from[0] + (to[0]-from[0])*k)
		y := snap(from[1]+(to[1]-from[1])*k+hop) + sway
		dc.SetColor(black)
		dc.DrawRectangle(x-30, y-30, 60, 60)
		dc.Fill()
		dc.SetColor(p.theme.Accent)
		dc.DrawRectangle(x-24, y-24, 48, 48)
		dc.Fill()
		size := 13.0
		if len([]rune(m.label)) > 3 {
			size = 10
		}
		dc.SetFontFace(render.Face(p.theme.Fonts.Pixel, size))
		dc.SetColor(black)
		dc.DrawStringAnchored(m.label, x, y, 0.5, 0.5)
	}
}

func (p *Pixel) drawSays(dc *gg.Context, t, sway float64) {
	for _, s := range p.says {
		if t < s.t || t >= s.until {
			continue
		}
		if s.who.cut && (t < s.who.appear || t >= s.who.gone) {
			continue
		}
		k := render.Pop(true, t-s.t)
		spec := p.sprites.specs[s.who.kind]
		px, py := s.who.x, s.who.y
		size := 1.0
		if s.who.cut {
			px, py = s.who.position(t)
			size = s.who.size
		}
		cx := px + s.dx
		by := py + sway - float64(spec.Anchor[1]-spec.Top[1])*scale*size - 40
		face := render.Face(p.theme.Fonts.Pixel, 28*k)
		dc.SetFontFace(face)
		tw, _ := dc.MeasureString(s.text)
		bw, bh := tw+56, 92*k
		x0, y0 := snap(cx-bw/2), snap(by-bh)
		render.FrameBox(dc, x0, y0, x0+snap(bw), by, p.theme.Ink, p.theme.Card, 6)
		dc.SetColor(p.theme.Ink)
		for i := 0; i < 4; i++ {
			dc.DrawRectangle(snap(cx)-18+float64(i)*6, by-6, 12, 18-float64(i)*6)
		}
		dc.Fill()
		if k == 1 {
			dc.SetColor(p.theme.Accent)
			dc.DrawStringAnchored(s.text, cx, y0+bh/2, 0.5, 0.5)
		}
	}
}

func lastStr(l []timedStr, t float64) string {
	v := ""
	for _, x := range l {
		if t >= x.t {
			v = x.v
		}
	}
	return v
}

func snap(v float64) float64 { return math.Round(v/3) * 3 }

func clamp(x float64) float64 { return math.Max(0, math.Min(1, x)) }

func smooth(x float64) float64 { return x * x * (3 - 2*x) }
