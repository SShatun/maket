package sprites

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"

	xdraw "golang.org/x/image/draw"

	"maket/internal/charkit"
)

type State struct {
	Frames int     `json:"frames"`
	FPS    float64 `json:"fps"`
	Loop   bool    `json:"loop"`
}

type Spec struct {
	Size   [2]int           `json:"size"`
	Anchor [2]int           `json:"anchor"`
	Top    [2]int           `json:"top"`
	Rest   string           `json:"rest"`
	Tint   *bool            `json:"tint"`
	States map[string]State `json:"states"`
}

type Step struct {
	T    float64
	V    string
	Hold bool
}

type key struct {
	sprite, state, role string
	i                   int
}

type Set struct {
	Dir          string
	Specs        map[string]Spec
	Scale        float64
	HoldOneShots bool
	colors       map[string]color.RGBA
	frames       map[key]*image.RGBA
}

func Load(dir string, colors map[string]color.RGBA, scale float64) (*Set, error) {
	b, err := os.ReadFile(filepath.Join(dir, "sprites.json"))
	if err != nil {
		return nil, err
	}
	var f struct {
		Sprites map[string]Spec `json:"sprites"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("sprites.json: %w", err)
	}
	return &Set{Dir: dir, Specs: f.Sprites, Scale: scale, colors: colors, frames: map[key]*image.RGBA{}}, nil
}

func (s *Set) Prepare(name string, roles map[string]bool, only map[string]bool) error {
	spec, ok := s.Specs[name]
	if !ok {
		return fmt.Errorf("no live character %q", name)
	}
	tint := spec.Tint == nil || *spec.Tint
	for state, st := range spec.States {
		if only != nil && !only[state] && state != spec.Rest {
			continue
		}
		for i := 0; i < st.Frames; i++ {
			src, err := charkit.ReadPNG(filepath.Join(s.Dir, name, fmt.Sprintf("%s_%d.png", state, i)))
			if err != nil {
				return err
			}
			for role := range roles {
				r := role
				if !tint {
					r = ""
				}
				k := key{name, state, r, i}
				if _, done := s.frames[k]; done {
					continue
				}
				im := src
				if tint {
					im = charkit.Tint(src, s.colors[role])
				}
				s.frames[k] = s.scaled(im)
			}
		}
	}
	return nil
}

func (s *Set) Frame(name string, steps []Step, t float64, role string) *image.RGBA {
	spec := s.Specs[name]
	ts, state, hold := 0.0, spec.Rest, false
	for _, x := range steps {
		if t >= x.T {
			ts, state, hold = x.T, x.V, x.Hold
		}
	}
	if s.HoldOneShots && state != "catch" && state != "flinch" {
		hold = true
	}
	st := spec.States[state]
	i := int((t - ts) * st.FPS)
	switch {
	case st.Loop:
		i %= st.Frames
	case i >= st.Frames && hold:
		i = st.Frames - 1
	case i >= st.Frames:
		state = spec.Rest
		rs := spec.States[state]
		i = int((t-ts)*rs.FPS) % rs.Frames
	}
	if spec.Tint != nil && !*spec.Tint {
		role = ""
	}
	return s.frames[key{name, state, role, i}]
}

func (s *Set) scaled(src image.Image) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, int(float64(b.Dx())*s.Scale), int(float64(b.Dy())*s.Scale)))
	xdraw.NearestNeighbor.Scale(dst, dst.Bounds(), src, b, xdraw.Src, nil)
	return dst
}
