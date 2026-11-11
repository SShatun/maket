package style

import (
	"fmt"
	"image"
	"image/draw"

	"maket/internal/audio"
	"maket/internal/render"
	"maket/internal/scene"
	"maket/internal/timeline"
)

type Style interface {
	Layout(sc *scene.Scene, tl *timeline.Timeline) error
	Draw(dst *image.RGBA, t float64)
	Sounds() []audio.Cue
	Theme() render.Theme
}

func Common(do string) bool {
	return do == "number" || do == "focus"
}

type None struct {
	theme render.Theme
}

func NewNone(fonts *render.Fonts) *None {
	return &None{theme: render.LightTheme(fonts)}
}

func (n *None) Layout(sc *scene.Scene, _ *timeline.Timeline) error {
	for i, ev := range sc.Events {
		if !Common(ev.Do) {
			return fmt.Errorf("event %d: style none does not know %q", i+1, ev.Do)
		}
	}
	return nil
}

func (n *None) Draw(dst *image.RGBA, _ float64) {
	draw.Draw(dst, dst.Bounds(), image.NewUniform(n.theme.BG), image.Point{}, draw.Src)
}

func (n *None) Sounds() []audio.Cue { return nil }

func (n *None) Theme() render.Theme { return n.theme }
