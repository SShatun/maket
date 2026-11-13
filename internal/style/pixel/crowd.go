package pixel

import (
	"encoding/json"
	"image"
	"math"

	"github.com/fogleman/gg"
	"golang.org/x/image/font"

	"maket/internal/behavior"
	"maket/internal/render"
	"maket/internal/scene"
	spr "maket/internal/style/sprites"
	"maket/internal/timeline"
)

func (p *Pixel) crowdStage(sh scene.Shot, at, until float64, tl *timeline.Timeline, seed int) *behavior.Stage {
	needed := false
	for _, c := range sh.Cast {
		needed = needed || c.Crowd != nil && c.Crowd.Behavior != ""
	}
	for _, ev := range sh.Do {
		needed = needed || ev.Do == "admit" || ev.Do == "tries" || ev.Do == "badge"
	}
	if !needed {
		return nil
	}
	st := behavior.NewStage(behavior.Config{Seed: uint64(seed), At: at, Until: until, Width: render.W, Height: render.H, Unit: 12, Specs: p.sprites.specs, Resolve: tl.Resolve, GateOffsets: map[string][2]float64{"lock": {108, -192}}})
	return st
}

func (p *Pixel) syncCrowd(st *behavior.Stage) {
	for _, x := range p.ents {
		if x.stage != st {
			continue
		}
		a := x.actor
		x.appear, x.gone = a.Appear, a.Gone
		x.roles, x.states, x.badges, x.status = nil, nil, nil, nil
		for _, r := range a.Roles {
			x.roles = append(x.roles, timedStr{r.T, r.V})
		}
		for _, s := range a.Steps {
			x.states = append(x.states, timedState{t: s.T, v: s.V, hold: s.Hold, silent: true})
		}
		for _, b := range a.Badges {
			x.badges = append(x.badges, timedStr{b.T, b.V})
		}
		for _, s := range a.Status {
			x.status = append(x.status, timedStr{s.T, s.V})
		}
	}
}

func pixelBadge(s string) string {
	if s == "bang" {
		return "bad"
	}
	return s
}

func (p *Pixel) prepareFlips() {
	for _, x := range p.ents {
		if x.actor == nil {
			continue
		}
		spec := p.sprites.specs[x.kind]
		for _, r := range x.roles {
			for state, st := range spec.States {
				for i := 0; i < st.Frames; i++ {
					im := p.sprites.Frame(x.kind, []spr.Step{{V: state}}, (float64(i)+0.5)/st.FPS, r.v)
					if im == nil || p.flips[im] != nil {
						continue
					}
					dst := image.NewRGBA(im.Bounds())
					for y := 0; y < im.Bounds().Dy(); y++ {
						for x := 0; x < im.Bounds().Dx(); x++ {
							dst.SetRGBA(im.Bounds().Dx()-1-x, y, im.RGBAAt(x, y))
						}
					}
					p.flips[im] = dst
				}
			}
		}
	}
}

func (p *Pixel) crowdFrame(a *behavior.Actor, t float64) (*image.RGBA, bool) {
	_, _, state, since, flip, moving := a.Pose(t)
	steps := a.Steps
	if state != "" {
		steps = []spr.Step{{T: since, V: state}}
	} else {
		steps = append([]spr.Step(nil), steps...)
		for i := range steps {
			if steps[i].V != "catch" && steps[i].V != "flinch" {
				steps[i].Hold = true
			}
		}
	}
	role := ""
	for _, r := range a.Roles {
		if t >= r.T {
			role = r.V
		}
	}
	im := p.sprites.Frame(a.Kind, steps, t, role)
	if flip && p.flips[im] != nil {
		im = p.flips[im]
	}
	return im, moving
}

func (p *Pixel) drawTally(dc *gg.Context, x *ent, tally *behavior.Tally, t float64) {
	i := tally.Index(t)
	txt := tally.Values[i]
	face := render.Face(p.theme.Fonts.Pixel, 28)
	w := float64(font.MeasureString(face, txt).Ceil()) + 32
	badge := p.theme.Badges[pixelBadge(tally.Badge)]
	if badge != nil {
		w += 48
	}
	px, py := x.position(t)
	spec := p.sprites.specs[x.kind]
	y := py - float64(spec.Anchor[1]-spec.Top[1])*scale*x.size - 76
	render.FrameBox(dc, px-w/2, y, px+w/2, y+58, p.theme.Ink, p.theme.Card, 6)
	tx := px
	if badge != nil {
		dc.Push()
		dc.Translate(px-w/2+24, y+24)
		dc.Scale(0.5, 0.5)
		dc.DrawImageAnchored(badge, 0, 0, 0.5, 0.5)
		dc.Pop()
		tx += 24
	}
	dc.SetFontFace(face)
	dc.SetColor(p.theme.Ink)
	dc.DrawStringAnchored(txt, tx, y+28, 0.5, 0.5)
	if tally.Badge == "wait" && i+1 < len(tally.Steps) {
		dc.SetColor(p.theme.Accent)
		dc.DrawRectangle(px-w/2+8, y+48, math.Max(0, w-16)*tally.Remaining(t), 4)
		dc.Fill()
	}
}

func crowdGate(st *behavior.Stage, events []scene.Event) error {
	for _, ev := range events {
		if ev.Do != "admit" {
			continue
		}
		var v struct {
			Gate string `json:"gate"`
		}
		if err := json.Unmarshal(ev.Raw, &v); err != nil {
			return err
		}
		st.Gate = st.ByID[v.Gate]
		break
	}
	return nil
}
