package rts

import (
	"errors"
	"image"
	"path/filepath"

	"maket/internal/audio"
	"maket/internal/render"
	"maket/internal/scene"
	"maket/internal/timeline"
)

type Style struct {
	dir   string
	theme render.Theme
}

func New(root string, fonts *render.Fonts) (*Style, error) {
	return &Style{dir: filepath.Join(root, "assets", "looks", "rts"), theme: render.PixelTheme(fonts, nil, nil)}, nil
}

func (s *Style) Layout(sc *scene.Scene, tl *timeline.Timeline) error {
	return errors.New("style rts is not ready yet")
}

func (s *Style) Draw(dst *image.RGBA, t float64) {}

func (s *Style) Sounds() []audio.Cue { return nil }

func (s *Style) Theme() render.Theme { return s.theme }
