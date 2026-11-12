package moe

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"sort"
	"strings"

	"github.com/fogleman/gg"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/math/f64"

	"maket/internal/render"
	"maket/internal/scene"
)

type walk struct {
	at, dur  float64
	from, to [2]float64
	flip     bool
}

type actorPose struct {
	x, y, ground, scaleY, hop float64
}

type movePoint struct {
	actor *actor
	xy    [2]float64
}

type move struct {
	at, dur, until, arc float64
	from, to            movePoint
	image, flash        *image.RGBA
}

func (a *actor) walkAt(t float64) *walk {
	i := sort.Search(len(a.walks), func(i int) bool { return a.walks[i].at > t }) - 1
	if i < 0 {
		return nil
	}
	return &a.walks[i]
}

func (a *actor) position(t float64) (float64, float64) {
	if w := a.walkAt(t); w != nil {
		k := clamp((t - w.at) / w.dur)
		k = k * k * (3 - 2*k)
		return lerp(w.from[0], w.to[0], k), lerp(w.from[1], w.to[1], k)
	}
	return a.x, a.y
}

func (a *actor) facing(t float64) bool {
	if w := a.walkAt(t); w != nil {
		return w.flip
	}
	return a.flip
}

func (a *actor) pose(start, t float64) actorPose {
	x, y := a.position(t)
	bob, breath, hop, squash := 0.0, 1.0, 0.0, 1.0
	dt := t - start
	if a.motion != "none" {
		bob = 3 * math.Sin(math.Pi*dt)
		breath = 1 + 0.01*math.Sin(math.Pi*dt)
	}
	if a.motion == "jump" {
		phase := math.Mod(math.Max(0, dt)*2, 1)
		hop = 45 * math.Sin(math.Pi*phase)
		squash = 1 - 0.06*math.Exp(-math.Pow(math.Min(phase, 1-phase)/0.08, 2))
	}
	if w := a.walkAt(t); w != nil && t < w.at+w.dur && w.from != w.to {
		bob, squash = 0, 1
		hop = 12 * math.Abs(math.Sin(2*math.Pi*(t-w.at))) * clamp((w.at+w.dur-t)/0.12)
	}
	return actorPose{x: x, y: y + bob - hop, ground: y, scaleY: a.size * breath * squash, hop: hop}
}

func point(p *scene.Point, actors map[string]*actor) (movePoint, error) {
	if p != nil {
		if p.ID != "" && p.Slot < 0 && actors[p.ID] != nil {
			return movePoint{actor: actors[p.ID]}, nil
		}
		if len(p.Coords) == 2 {
			return movePoint{xy: [2]float64{p.Coords[0], p.Coords[1]}}, nil
		}
	}
	return movePoint{}, fmt.Errorf("move needs from and to as cast ids or [x, y]")
}

func (p movePoint) position(t float64) [2]float64 {
	if p.actor != nil {
		x, y := p.actor.position(t)
		return [2]float64{x, y}
	}
	return p.xy
}

func (s *Style) hand(p movePoint, start, t, towardX float64) [2]float64 {
	if p.actor == nil {
		return p.xy
	}
	a := p.actor
	pose := a.pose(start, t)
	spec := s.sprites.Specs[a.kind]
	side := 1.0
	if towardX < pose.x {
		side = -1
	}
	return [2]float64{pose.x + side*float64(spec.Size[0])*a.size*0.18, pose.y - float64(spec.Anchor[1]-spec.Top[1])*pose.scaleY*0.45}
}

func (s *Style) makeMove(ev scene.Event, at, until float64, actors map[string]*actor) (move, error) {
	m := move{at: at, dur: ev.Dur, until: until, arc: 120}
	var err error
	if m.from, err = point(ev.From, actors); err != nil {
		return m, err
	}
	if m.to, err = point(ev.To, actors); err != nil {
		return m, err
	}
	if m.dur == 0 {
		m.dur = 0.6
	}
	if ev.Arc != nil {
		m.arc = *ev.Arc
	}
	if m.dur < 0 || m.arc < 0 {
		return m, fmt.Errorf("move needs positive dur and nonnegative arc")
	}
	if ev.Until == nil {
		m.until = math.Min(until, at+m.dur)
	} else if until < at+m.dur {
		return m, fmt.Errorf("move until must not precede arrival")
	}
	role := ev.Role
	if role == "" {
		role = "storage"
	}
	col, ok := s.roles[role]
	if !ok {
		return m, fmt.Errorf("move: unknown role %q", role)
	}
	if m.image, err = s.makeTag(ev.Label, col); err != nil {
		return m, err
	}
	if ev.Consume {
		m.flash = image.NewRGBA(image.Rect(-96, -96, 96, 96))
		for y := -96; y < 96; y++ {
			for x := -96; x < 96; x++ {
				r := math.Hypot(float64(x), float64(y))
				v := uint8(255 * math.Pow(clamp(1-r/96), 2))
				m.flash.SetRGBA(x, y, color.RGBA{v, v, v, 255})
			}
		}
	}
	return m, nil
}

func (s *Style) makeTag(label string, col color.RGBA) (*image.RGBA, error) {
	if strings.TrimSpace(label) == "" || strings.ContainsAny(label, "\n\r") {
		return nil, fmt.Errorf("move label must be one nonempty line")
	}
	const k = 1.75
	face := render.Face(s.theme.Fonts.Bold, 40*k)
	defer face.Close()
	w := math.Max(144*k, float64(font.MeasureString(face, label).Ceil())+48*k)
	if w > render.W-48 {
		return nil, fmt.Errorf("move label is too wide; shorten it")
	}
	b := image.Rect(-int(w/2+20*k), -int(12*k), int(math.Ceil(w/2+28*k)), int(132*k))
	im := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	dc := gg.NewContextForRGBA(im)
	dc.Translate(float64(-b.Min.X), float64(-b.Min.Y))
	dc.SetRGBA255(0, 0, 0, 28)
	dc.DrawRoundedRectangle(-w/2+2*k, 26*k, w+8*k, 94*k, 20*k)
	dc.Fill()
	dc.SetRGBA255(0, 0, 0, 40)
	dc.DrawRoundedRectangle(-w/2+6*k, 29*k, w, 88*k, 16*k)
	dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(-w/2, 20*k, w, 88*k, 16*k)
	dc.FillPreserve()
	dc.SetColor(col)
	dc.SetLineWidth(3 * k)
	dc.Stroke()
	dc.MoveTo(0, 0)
	dc.CubicTo(-20*k, 4*k, 12*k, 18*k, 0, 32*k)
	dc.Stroke()
	dc.DrawCircle(0, 32*k, 4*k)
	dc.Stroke()
	dc.SetFontFace(face)
	dc.SetColor(s.theme.Caption.Edge)
	dc.DrawStringAnchored(label, 0, 70*k, 0.5, 0.35)
	im.Rect = b
	return im, nil
}

func (s *Style) drawMoves(dst *image.RGBA, sh *shot, cam f64.Aff3, t float64) {
	for _, m := range sh.moves {
		if t < m.at || t >= m.until+0.16 || t >= m.until && m.flash == nil {
			continue
		}
		fromCenter, toCenter := m.from.position(m.at), m.to.position(m.at+m.dur)
		from := s.hand(m.from, sh.at, m.at, toCenter[0])
		to := s.hand(m.to, sh.at, t, fromCenter[0])
		if t >= m.until {
			dt := (t - m.until) / 0.16
			z := 0.6 + dt
			addImage(dst, m.flash, compose(cam, f64.Aff3{z, 0, to[0], 0, z, to[1] + 64}), 1-dt)
			continue
		}
		k := clamp((t - m.at) / m.dur)
		k = k * k * (3 - 2*k)
		x, y := lerp(from[0], to[0], k), lerp(from[1], to[1], k)-m.arc*4*k*(1-k)
		angle := 0.06 * math.Sin(2*math.Pi*1.2*(t-m.at))
		sin, cos := math.Sincos(angle)
		matrix := f64.Aff3{cam[0] * cos, -cam[0] * sin, cam[0]*x + cam[2], cam[4] * sin, cam[4] * cos, cam[4]*y + cam[5]}
		xdraw.ApproxBiLinear.Transform(dst, matrix, m.image, m.image.Bounds(), draw.Over, nil)
	}
}
