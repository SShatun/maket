package render

import (
	"image/color"
	"math"

	"github.com/fogleman/gg"
	"golang.org/x/image/font"

	"maket/internal/scene"
	"maket/internal/timeline"
)

func ShotElements(dc *gg.Context, events []scene.Event, tl *timeline.Timeline, th Theme, t, end float64) {
	o := overlay{dc: dc, tl: tl, th: th, t: t}
	for _, ev := range events {
		until := end
		if ev.Until != nil {
			until = math.Min(until, tl.MustResolve(*ev.Until))
		}
		if t < tl.MustResolve(ev.At) || t >= until {
			continue
		}
		switch ev.Do {
		case "line":
			dc.SetColor(o.tone(ev.Tone))
			x0, y0, x1, y1 := ev.From.Coords[0], ev.From.Coords[1], ev.To.Coords[0], ev.To.Coords[1]
			pixelLine(dc, x0, y0, x1, y1, ev.Kind == "dash")
			if ev.Kind == "arrow" {
				a := math.Atan2(y1-y0, x1-x0)
				for _, side := range []float64{-1, 1} {
					pixelLine(dc, x1, y1, x1-30*math.Cos(a)+side*18*math.Sin(a), y1-30*math.Sin(a)-side*18*math.Cos(a), false)
				}
			}
		case "text":
			o.text(elementFace(th, ev), ev.Text, ev.Pos[0], ev.Pos[1], o.tone(ev.Tone))
		case "panel":
			o.panel(ev)
		}
	}
}

func pixelLine(dc *gg.Context, x0, y0, x1, y1 float64, dash bool) {
	d := math.Hypot(x1-x0, y1-y0)
	n := max(1, int(math.Ceil(d/3)))
	for i := 0; i <= n; i++ {
		u := float64(i) / float64(n)
		if dash && int(d*u/18)%2 != 0 {
			continue
		}
		x, y := math.Round((x0+(x1-x0)*u)/3)*3, math.Round((y0+(y1-y0)*u)/3)*3
		dc.DrawRectangle(x-3, y-3, 6, 6)
	}
	dc.Fill()
}

func (o overlay) tone(tone string) color.RGBA {
	switch tone {
	case "accent":
		return o.th.Accent
	case "ok":
		return o.th.OK
	case "error":
		return o.th.Error
	default:
		return o.th.Ink
	}
}

func elementFace(th Theme, ev scene.Event) font.Face {
	size := ev.Size
	if size == 0 {
		size = 28
	}
	return Face(th.Fonts.Pixel, size)
}

func TextBounds(th Theme, ev scene.Event) [4]float64 {
	face := elementFace(th, ev)
	b, advance := font.BoundString(face, ev.Text)
	x := ev.Pos[0] - float64(advance>>6)/2
	y := ev.Pos[1] + 0.35*float64(face.Metrics().Height)/64
	return [4]float64{x + float64(b.Min.X)/64, y + float64(b.Min.Y)/64, x + float64(b.Max.X)/64, y + float64(b.Max.Y)/64}
}

func (o overlay) panel(ev scene.Event) {
	x0, y0, x1, y1 := ev.Box[0], ev.Box[1], ev.Box[2], ev.Box[3]
	fill, ink := o.th.Card, o.th.Ink
	if ev.Fill == "accent" {
		fill, ink = o.th.Accent, o.th.Card
	} else if ev.Fill == "error" {
		fill = o.th.Error
	}
	border := o.tone(ev.Tone)
	FrameBox(o.dc, x0, y0, x1, y1, border, fill, 6)
	width := math.Max(1, x1-x0-52)
	if ev.Title != "" {
		base := 64.0
		cy := (y0 + y1) / 2
		if len(ev.Lines) > 0 {
			base = 32
			cy = y0 + 38
			y0 += 62
		}
		size := numberSize(o.th.Fonts.Pixel, math.Min(base, (y1-ev.Box[1]-24)/2), width, ev.Title, true)
		titleColor := ink
		if ev.Tone != "" {
			titleColor = border
		}
		o.text(Face(o.th.Fonts.Pixel, math.Max(1, size)), ev.Title, (x0+x1)/2, cy, titleColor)
	}
	lh := math.Min(38, (y1-y0-40)/float64(max(1, len(ev.Lines))))
	size := math.Max(1, 25*math.Min(1, lh/38))
	for _, l := range ev.Lines {
		size = math.Min(size, numberSize(o.th.Fonts.Mono, size, width, l.Text, false))
	}
	face := Face(o.th.Fonts.Mono, size)
	row := 0
	for _, l := range ev.Lines {
		if l.Appear != nil && o.t < o.tl.MustResolve(*l.Appear) {
			continue
		}
		y := y0 + 22 + float64(row)*lh
		row++
		col := o.th.CodeDim
		if ev.Fill == "accent" || ev.Fill == "error" {
			col = ink
		}
		tone := l.Tone
		if tone == "accent" {
			tone = "yel"
		}
		if highlight, ok := o.th.Tones[tone]; ok {
			o.dc.SetColor(highlight.BG)
			o.dc.DrawRectangle(x0+12, y-4, x1-x0-24, lh-2)
			o.dc.Fill()
			col = highlight.FG
		} else if l.Tone != "" {
			col = o.tone(l.Tone)
		}
		o.dc.SetFontFace(face)
		o.dc.SetColor(col)
		o.dc.DrawStringAnchored(l.Text, x0+26, y+lh/2-7, 0, 0.5)
	}
}
