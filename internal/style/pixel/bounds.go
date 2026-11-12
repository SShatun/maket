package pixel

import (
	"fmt"
	"math"
	"os"
	"strings"

	"maket/internal/render"
	"maket/internal/scene"
	"maket/internal/timeline"
)

func (p *Pixel) warnBounds(shot int, events []scene.Event, end float64, tl *timeline.Timeline) {
	for i, ev := range events {
		var box [4]float64
		var bubble *say
		label := ev.Text
		start, until := tl.MustResolve(ev.At), end
		switch ev.Do {
		case "panel":
			copy(box[:], ev.Box)
			label = ev.Title
		case "text":
			box = render.TextBounds(p.theme, ev)
		case "say":
			bubble = &say{t: start, who: p.byID[ev.Target.ID], text: ev.Text, dx: ev.Dx}
			until = math.Min(until, start+2)
		default:
			continue
		}
		if ev.Until != nil {
			until = math.Min(end, tl.MustResolve(*ev.Until))
		}
		if bubble != nil {
			start, until = math.Max(start, bubble.who.appear), math.Min(until, bubble.who.gone)
		}
		if until <= start {
			continue
		}
		times := []float64{start, math.Nextafter(until, start)}
		for frame := int(math.Ceil(start * render.FPS)); float64(frame)/render.FPS < until; frame++ {
			times = append(times, float64(frame)/render.FPS)
		}
		var overflow [4]float64
		for _, t := range times {
			if bubble != nil {
				box, _, _ = p.sayBox(*bubble, t, 0)
				box[3] += 12
			}
			v := render.Camera(events, tl, t)
			x0, y0 := box[0]*v[0]+v[1], box[1]*v[0]+v[2]
			x1, y1 := box[2]*v[0]+v[1], box[3]*v[0]+v[2]
			for side, d := range [4]float64{-x0, 140 - y0, x1 - render.W, y1 - 1640} {
				overflow[side] = math.Max(overflow[side], d)
			}
		}
		var sides []string
		for side, name := range []string{"left", "top", "right", "bottom"} {
			if overflow[side] > 0 {
				sides = append(sides, fmt.Sprintf("%s %.0f px", name, math.Ceil(overflow[side])))
			}
		}
		if len(sides) > 0 {
			fmt.Fprintf(os.Stderr, "maket: shot %d event %d %s %q outside frame: %s\n", shot+1, i+1, ev.Do, label, strings.Join(sides, ", "))
		}
	}
}
