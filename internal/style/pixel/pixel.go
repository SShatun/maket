package pixel

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
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
	t    float64
	v    string
	hold bool
}

type ent struct {
	id, kind, label string
	x, y            float64
	appear          float64
	roles           []timedStr
	states          []timedState
	status          []timedStr
	badges          []timedStr
	shakes          []float64
}

type move struct {
	t, dur, until float64
	from, to      [2]float64
	label         string
}

type say struct {
	t, until float64
	who      *ent
	text     string
	dx       float64
}

type focus struct {
	t       float64
	targets map[string]bool
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
	for i, e := range sc.Entities {
		if len(e.At) != 2 {
			return fmt.Errorf("entity %s: pixel style wants at: [x, y] in px", e.ID)
		}
		x := &ent{id: e.ID, kind: e.Kind, label: e.Label, x: e.At[0], y: e.At[1], appear: 1.4 + 0.1*float64(i)}
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
			x.states = append(x.states, timedState{at(s.At), s.V, s.Hold})
		}
		for _, s := range e.Status {
			x.status = append(x.status, timedStr{at(s.At), s.V})
		}
		for _, s := range e.Badges {
			x.badges = append(x.badges, timedStr{at(s.At), s.V})
		}
		p.ents = append(p.ents, x)
		p.byID[e.ID] = x
	}
	for i, ev := range sc.Events {
		t := at(ev.At)
		if err := p.event(i, ev, t, tl); err != nil {
			return err
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

func (p *Pixel) event(i int, ev scene.Event, t float64, tl *timeline.Timeline) error {
	target := func() (*ent, error) {
		if ev.Target == nil || p.byID[ev.Target.ID] == nil {
			return nil, fmt.Errorf("event %d: %s needs a target entity", i+1, ev.Do)
		}
		return p.byID[ev.Target.ID], nil
	}
	until := func(def float64) float64 {
		if ev.Until != nil {
			return tl.MustResolve(*ev.Until)
		}
		return def
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
		from, err := p.point(*ev.From)
		if err != nil {
			return fmt.Errorf("event %d: %w", i+1, err)
		}
		to, err := p.point(*ev.To)
		if err != nil {
			return fmt.Errorf("event %d: %w", i+1, err)
		}
		label := ev.Label
		if label == "" {
			label = "git"
		}
		p.moves = append(p.moves, move{t: t, dur: dur, until: until(t + dur), from: from, to: to, label: label})
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
		x.states = append(x.states, timedState{t, ev.State, ev.Hold})
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
	case "focus":
		f := &focus{t: t, targets: map[string]bool{}}
		for _, pt := range ev.Targets {
			f.targets[pt.ID] = true
		}
		if ev.Target != nil {
			f.targets[ev.Target.ID] = true
		}
		p.focus = f
	default:
		if !style.Common(ev.Do) {
			return fmt.Errorf("event %d: pixel style does not know %q", i+1, ev.Do)
		}
	}
	return nil
}

func (p *Pixel) point(pt scene.Point) ([2]float64, error) {
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
	return [2]float64{x.x, x.y - 135}, nil
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
	dc := gg.NewContextForRGBA(dst)
	sway := snap(-8 * math.Sin(t/p.dur*math.Pi))
	label := render.Face(p.theme.Fonts.Pixel, 38*0.62)
	status := render.Face(p.theme.Fonts.Mono, 22)
	for _, x := range p.ents {
		p.drawEnt(dst, dc, x, t, sway, label, status)
	}
	p.drawMoves(dc, t, sway)
	p.drawSays(dc, t, sway)
	if f := p.focus; f != nil && t >= f.t {
		k := clamp((t - f.t) / 0.6)
		c := p.theme.BG
		dc.SetRGBA255(int(c.R), int(c.G), int(c.B), int(255*0.6*k))
		dc.DrawRectangle(0, 140, render.W, 960)
		dc.Fill()
		for _, x := range p.ents {
			if f.targets[x.id] {
				p.drawEnt(dst, dc, x, t, sway, label, status)
			}
		}
	}
}

func (p *Pixel) drawEnt(dst *image.RGBA, dc *gg.Context, x *ent, t, sway float64, label, status font.Face) {
	k := render.Pop(true, t-x.appear)
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
	w, h := float64(im.Bounds().Dx()), float64(im.Bounds().Dy())
	ay := x.y + sway
	foot := float64(spec.Size[1]-spec.Anchor[1]) * scale
	dc.SetColor(floor)
	dc.DrawRectangle(x.x-w*0.42, ay-6, w*0.84, 12)
	dc.Fill()
	sw, sh := w*k, h*k
	r := image.Rect(int(x.x-sw/2+shx), int(ay+foot-sh), int(x.x+sw/2+shx), int(ay+foot))
	if k == 1 {
		draw.Draw(dst, r, im, image.Point{}, draw.Over)
	} else {
		xdraw.NearestNeighbor.Scale(dst, r, im, im.Bounds(), xdraw.Over, nil)
	}
	if k != 1 {
		return
	}
	dc.SetFontFace(label)
	dc.SetColor(p.theme.Ink)
	dc.DrawStringAnchored(x.label, x.x, ay+40, 0.5, 0.5)
	if s := lastStr(x.status, t); s != "" {
		dc.SetFontFace(status)
		dc.SetColor(grey)
		dc.DrawStringAnchored(s, x.x, ay+72, 0.5, 0.5)
	}
	if b := p.theme.Badges[lastStr(x.badges, t)]; b != nil {
		ty := ay - float64(spec.Anchor[1]-spec.Top[1])*scale
		dc.DrawImageAnchored(b, int(x.x+w*0.3+shx), int(ty+6), 0.5, 0.5)
	}
}

func (p *Pixel) drawMoves(dc *gg.Context, t, sway float64) {
	for _, m := range p.moves {
		if t < m.t || t >= m.until {
			continue
		}
		k := smooth(clamp((t - m.t) / m.dur))
		hop := -60 * math.Sin(k*math.Pi)
		x := snap(m.from[0] + (m.to[0]-m.from[0])*k)
		y := snap(m.from[1]+(m.to[1]-m.from[1])*k+hop) + sway
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
		k := render.Pop(true, t-s.t)
		spec := p.sprites.specs[s.who.kind]
		cx := s.who.x + s.dx
		by := s.who.y + sway - float64(spec.Anchor[1]-spec.Top[1])*scale - 40
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
