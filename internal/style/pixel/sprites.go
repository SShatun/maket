package pixel

import (
	"image"
	"image/color"

	spr "maket/internal/style/sprites"
)

const scale = 1.0

type spriteSpec = spr.Spec

type sprites struct {
	*spr.Set
	dir   string
	specs map[string]spriteSpec
}

func loadSprites(dir string, colors map[string]color.RGBA) (*sprites, error) {
	s, err := spr.Load(dir, colors, scale)
	if err != nil {
		return nil, err
	}
	return &sprites{Set: s, dir: dir, specs: s.Specs}, nil
}

func (s *sprites) prepare(name string, roles map[string]bool) error {
	return s.Prepare(name, roles, nil)
}

func (s *sprites) frame(name string, states []timedState, t float64, role string) *image.RGBA {
	steps := make([]spr.Step, len(states))
	for i, x := range states {
		steps[i] = spr.Step{T: x.t, V: x.v, Hold: x.hold}
	}
	return s.Frame(name, steps, t, role)
}
