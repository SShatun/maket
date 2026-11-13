package rts

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"path/filepath"
	"sort"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"

	"maket/internal/audio"
	"maket/internal/charkit"
	"maket/internal/render"
	"maket/internal/scene"
	"maket/internal/style/pixel"
	spr "maket/internal/style/sprites"
	"maket/internal/timeline"
)

const (
	mapW, mapH = 180, 229
	px         = 6
	mapTop     = 48
	mapBottom  = mapTop + mapH*px
	panelTop   = 1424
)

var mapRect = image.Rect(0, mapTop, render.W, mapBottom)

type Style struct {
	dir     string
	theme   render.Theme
	fonts   *render.Fonts
	roles   map[string]color.RGBA
	palette map[string]color.RGBA
	set     *spr.Set
	art     *art
	sc      *scene.Scene
	tl      *timeline.Timeline
	shots   []*shot
	flips   map[*image.RGBA]*image.RGBA
	boxes   map[string]image.Rectangle
	looks   map[string]*image.RGBA
	res     map[string][]resStep
	cues    []audio.Cue
	faces   map[float64]font.Face
}

func New(root string, fonts *render.Fonts) (*Style, error) {
	dir := filepath.Join(root, "assets", "looks", "rts")
	lk, err := charkit.ReadLook(dir)
	if err != nil {
		return nil, err
	}
	roles, _, err := pixel.Colors(dir)
	if err != nil {
		return nil, err
	}
	set, err := spr.Load(dir, roles, 0.5)
	if err != nil {
		return nil, err
	}
	set.HoldOneShots = true
	a, err := loadArt(dir)
	if err != nil {
		return nil, err
	}
	th := render.PixelTheme(fonts, nil, nil)
	th.Caption = render.CaptionStyle{
		Y: 1330, Size: 46, Stroke: 7,
		Ink: color.RGBA{255, 255, 255, 255}, Dim: color.RGBA{160, 160, 150, 255}, Edge: color.RGBA{10, 8, 6, 255},
		Karaoke: true,
	}
	th.OwnCaption = true
	th.NoProgress = true
	return &Style{dir: dir, theme: th, fonts: fonts, roles: roles, palette: lk.Palette, set: set, art: a, faces: map[float64]font.Face{}}, nil
}

func (s *Style) Theme() render.Theme { return s.theme }

func (s *Style) Sounds() []audio.Cue { return s.cues }

func (s *Style) Layout(sc *scene.Scene, tl *timeline.Timeline) error {
	if len(sc.Shots) == 0 {
		return errors.New("style rts needs shots: every shot has its own map, hud, cast and do")
	}
	starts, err := tl.ShotStarts(sc)
	if err != nil {
		return err
	}
	if err := s.captionStyle(sc.Caption); err != nil {
		return err
	}
	s.sc, s.tl = sc, tl
	s.shots = nil
	s.res = map[string][]resStep{}
	s.boxes = map[string]image.Rectangle{}
	var keys []string
	for i, raw := range sc.Shots {
		end := tl.Duration
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		sh, err := s.buildShot(i, raw, starts[i], end, keys, int64(sc.Seed))
		if err != nil {
			return fmt.Errorf("shot %d: %w", i+1, err)
		}
		keys = sh.keys
		s.shots = append(s.shots, sh)
	}
	for _, steps := range s.res {
		sort.SliceStable(steps, func(i, j int) bool { return steps[i].t < steps[j].t })
	}
	if err := s.prepare(); err != nil {
		return err
	}
	s.cues = s.sounds()
	return nil
}

func (s *Style) captionStyle(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	var c struct {
		Size   float64  `json:"size"`
		Y      float64  `json:"y"`
		Stroke *float64 `json:"stroke"`
		Plate  []uint8  `json:"plate"`
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return fmt.Errorf("caption: %w", err)
	}
	cs := &s.theme.Caption
	if c.Size > 0 {
		cs.Size = c.Size
	}
	if c.Y > 0 {
		cs.Y = c.Y
	}
	if c.Stroke != nil {
		cs.Stroke = *c.Stroke
	}
	switch len(c.Plate) {
	case 0:
	case 4:
		cs.Plate = color.RGBA{c.Plate[0], c.Plate[1], c.Plate[2], c.Plate[3]}
	default:
		return errors.New("caption: plate is [r, g, b, a]")
	}
	return nil
}

func (s *Style) prepare() error {
	type want struct {
		roles, states map[string]bool
		flip          bool
	}
	wants := map[string]*want{}
	var kinds []string
	for _, sh := range s.shots {
		for _, a := range sh.actors {
			w := wants[a.Kind]
			if w == nil {
				w = &want{roles: map[string]bool{}, states: map[string]bool{}}
				wants[a.Kind] = w
				kinds = append(kinds, a.Kind)
			}
			spec := s.set.Specs[a.Kind]
			for _, r := range a.Roles {
				if _, ok := s.roles[r.V]; !ok {
					return fmt.Errorf("%s: unknown role %q", a.Name(), r.V)
				}
				w.roles[r.V] = true
			}
			for _, st := range a.States() {
				if _, ok := spec.States[st]; !ok {
					return fmt.Errorf("%s: %s has no state %q", a.Name(), a.Kind, st)
				}
				w.states[st] = true
			}
			for _, m := range a.Badges {
				if _, ok := s.art.marks[m.V]; !ok && m.V != "" {
					return fmt.Errorf("%s: unknown mark %q, known: bang, question, wait, dots, zzz", a.Name(), m.V)
				}
			}
			w.flip = w.flip || a.Unit || a.Flip || len(a.Legs) > 0
		}
	}
	s.flips = map[*image.RGBA]*image.RGBA{}
	s.looks = map[string]*image.RGBA{}
	for _, kind := range kinds {
		w := wants[kind]
		if err := s.set.Prepare(kind, w.roles, w.states); err != nil {
			return err
		}
		s.box(kind)
		spec := s.set.Specs[kind]
		w.states[spec.Rest] = true
		if !w.flip {
			continue
		}
		for role := range w.roles {
			for st := range w.states {
				ss := spec.States[st]
				for i := 0; i < ss.Frames; i++ {
					im := s.set.Frame(kind, []spr.Step{{T: 0, V: st}}, (float64(i)+0.5)/ss.FPS, role)
					if im != nil && s.flips[im] == nil {
						s.flips[im] = mirror(im)
					}
				}
			}
		}
	}
	for _, sh := range s.shots {
		for _, p := range sh.picks {
			if p.who == nil {
				continue
			}
			for _, r := range p.who.Roles {
				k := p.who.Kind + "/" + r.V
				if s.looks[k] != nil {
					continue
				}
				im, err := s.portrait(p.who.Kind, r.V)
				if err != nil {
					return err
				}
				s.looks[k] = im
			}
		}
	}
	return nil
}

func (s *Style) box(kind string) image.Rectangle {
	if b, ok := s.boxes[kind]; ok {
		return b
	}
	spec := s.set.Specs[kind]
	src, err := charkit.ReadPNG(filepath.Join(s.dir, kind, spec.Rest+"_0.png"))
	b := image.Rectangle{}
	if err == nil {
		b = opaque(shrink(src, 2))
	}
	s.boxes[kind] = b
	return b
}

func (s *Style) portrait(kind, role string) (*image.RGBA, error) {
	spec := s.set.Specs[kind]
	src, err := charkit.ReadPNG(filepath.Join(s.dir, kind, "idle_0.png"))
	if err != nil {
		src, err = charkit.ReadPNG(filepath.Join(s.dir, kind, spec.Rest+"_0.png"))
		if err != nil {
			return nil, err
		}
	}
	if spec.Tint == nil || *spec.Tint {
		src = charkit.Tint(src, s.roles[role])
	}
	full := image.NewRGBA(src.Bounds())
	draw.Draw(full, full.Bounds(), src, src.Bounds().Min, draw.Src)
	b := opaque(full)
	w, h := (b.Dx()+11)/12, (b.Dy()+11)/12
	k := 2
	for _, c := range []int{6, 4, 3} {
		if w*c <= 96 && h*c <= 96 {
			k = c
			break
		}
	}
	big := image.NewRGBA(image.Rect(0, 0, b.Dx()*k/12, b.Dy()*k/12))
	xdraw.NearestNeighbor.Scale(big, big.Bounds(), full, b, xdraw.Src, nil)
	cw, ch := min(96, big.Bounds().Dx()), min(96, big.Bounds().Dy())
	x0, y0 := (big.Bounds().Dx()-cw)/2/k*k, (big.Bounds().Dy()-ch)/2/k*k
	out := image.NewRGBA(image.Rect(0, 0, cw, ch))
	draw.Draw(out, out.Bounds(), big, image.Pt(x0, y0), draw.Src)
	return out, nil
}

func (s *Style) shotAt(t float64) *shot {
	cur := s.shots[0]
	for _, sh := range s.shots {
		if t >= sh.at {
			cur = sh
		}
	}
	return cur
}
