package render

import (
	"image"
	"image/color"
	"math"
	"strings"

	"image/draw"

	"github.com/fogleman/gg"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"

	"maket/internal/scene"
	"maket/internal/timeline"
)

type CaptionStyle struct {
	Y, Size, Stroke, Center float64
	Ink, Dim, Edge          color.RGBA
	Karaoke                 bool
}

func Caption(dst *image.RGBA, sc *scene.Scene, tl *timeline.Timeline, th Theme, t float64) {
	if th.Caption.Size > 0 {
		styledCaption(dst, sc, tl, th, t)
		return
	}
	overlay{dc: gg.NewContextForRGBA(dst), sc: sc, tl: tl, th: th, t: t}.caption()
}

type spoken struct {
	text string
	at   float64
}

func styledCaption(dst *image.RGBA, sc *scene.Scene, tl *timeline.Timeline, th Theme, t float64) {
	cur := -1
	for i, p := range tl.Phrases {
		if i < len(sc.Phrases) && t >= p.Start {
			cur = i
		}
	}
	if cur < 0 || t > tl.Phrases[cur].End+0.35 {
		return
	}
	text := sc.Phrases[cur].Caption
	if text == "" {
		text = sc.Phrases[cur].Text
	}
	text = strings.ReplaceAll(text, "\u0301", "")
	words := strings.Fields(text)
	if len(words) == 0 {
		return
	}
	pt := tl.Phrases[cur]
	marks := make([]spoken, len(words))
	k := 0
	for i, w := range words {
		marks[i] = spoken{w, pt.Start}
		ws := scene.Words(w)
		if len(ws) == 0 {
			if i > 0 {
				marks[i].at = marks[i-1].at
			}
			continue
		}
		f := scene.Fold(ws[0])
		for j := k; j < len(pt.Words); j++ {
			if scene.Fold(pt.Words[j].Text) == f {
				marks[i].at, k = pt.Words[j].S, j+1
				break
			}
		}
	}
	cs := th.Caption
	face := Face(th.Fonts.Bold, cs.Size)
	center := cs.Center
	if center == 0 {
		center = W / 2
	}
	lines := wrapCaption(face, marks, W-2*(80+cs.Stroke))
	lh := cs.Size * 1.25
	y := cs.Y - lh*float64(len(lines)-1)/2
	space := float64(font.MeasureString(face, " ").Round())
	for _, line := range lines {
		width := -space
		for _, m := range line {
			width += float64(font.MeasureString(face, m.text).Round()) + space
		}
		x := center - width/2
		for _, m := range line {
			col := cs.Ink
			if cs.Karaoke && t < m.at {
				col = cs.Dim
			}
			strokeText(dst, face, m.text, x, y, cs.Stroke, cs.Edge, col)
			x += float64(font.MeasureString(face, m.text).Round()) + space
		}
		y += lh
	}
}

func wrapCaption(face font.Face, marks []spoken, max float64) [][]spoken {
	var lines [][]spoken
	var line []spoken
	w := 0.0
	space := float64(font.MeasureString(face, " ").Round())
	for _, m := range marks {
		mw := float64(font.MeasureString(face, m.text).Round())
		if len(line) > 0 && w+space+mw > max {
			lines = append(lines, line)
			line, w = nil, 0
		}
		if len(line) > 0 {
			w += space
		}
		line = append(line, m)
		w += mw
	}
	return append(lines, line)
}

func strokeText(dst *image.RGBA, face font.Face, s string, x, y, r float64, edge, ink color.RGBA) {
	m := face.Metrics()
	asc, desc := m.Ascent.Ceil(), m.Descent.Ceil()
	pad := int(math.Ceil(r)) + 1
	w := font.MeasureString(face, s).Ceil() + 2*pad
	h := asc + desc + 2*pad
	mask := image.NewAlpha(image.Rect(0, 0, w, h))
	d := font.Drawer{Dst: mask, Src: image.Opaque, Face: face, Dot: fixed.P(pad, pad+asc)}
	d.DrawString(s)
	base := y + 0.35*float64(m.Height.Ceil())
	ox := int(math.Round(x)) - pad
	oy := int(math.Round(base)) - asc - pad
	stamp := func(dx, dy int, c color.RGBA) {
		rect := image.Rect(ox+dx, oy+dy, ox+dx+w, oy+dy+h)
		draw.DrawMask(dst, rect, image.NewUniform(c), image.Point{}, mask, image.Point{}, draw.Over)
	}
	if r > 0 {
		n := int(math.Max(8, r*2))
		for i := 0; i < n; i++ {
			a := 2 * math.Pi * float64(i) / float64(n)
			stamp(int(math.Round(r*math.Cos(a))), int(math.Round(r*math.Sin(a))), edge)
		}
	}
	stamp(0, 0, ink)
}
