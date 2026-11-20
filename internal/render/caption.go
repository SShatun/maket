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
	Ink, Dim, Edge, Plate   color.RGBA
	Karaoke                 bool
}

func Caption(dst *image.RGBA, sc *scene.Scene, tl *timeline.Timeline, th Theme, t float64) {
	if th.Caption.Size > 0 {
		styledCaption(dst, sc, tl, th, t)
		return
	}
	overlay{dc: gg.NewContextForRGBA(dst), sc: sc, tl: tl, th: th, t: t}.caption()
}

type CaptionWord struct {
	Text string
	At   float64
}

func styledCaption(dst *image.RGBA, sc *scene.Scene, tl *timeline.Timeline, th Theme, t float64) {
	cur := CaptionPhrase(sc, tl, t)
	if cur < 0 {
		return
	}
	marks := CaptionWords(sc.Phrases[cur], tl.Phrases[cur])
	if len(marks) > 0 {
		drawStyledCaption(dst, th, marks, t)
	}
}

func CaptionWords(p scene.Phrase, pt timeline.PhraseTime) []CaptionWord {
	text := p.Caption
	if text == "" {
		text = p.Text
	}
	text = strings.ReplaceAll(text, "\u0301", "")
	words := strings.Fields(text)
	marks := make([]CaptionWord, len(words))
	k := 0
	for i, w := range words {
		marks[i] = CaptionWord{w, pt.Start}
		ws := scene.Words(w)
		if len(ws) == 0 {
			if i > 0 {
				marks[i].At = marks[i-1].At
			}
			continue
		}
		f := scene.Fold(ws[0])
		for j := k; j < len(pt.Words); j++ {
			if scene.Fold(pt.Words[j].Text) == f {
				marks[i].At, k = pt.Words[j].S, j+1
				break
			}
		}
	}
	return marks
}

func CaptionPhrase(sc *scene.Scene, tl *timeline.Timeline, t float64) int {
	cur := -1
	for i, p := range tl.Phrases {
		if i < len(sc.Phrases) && t >= p.Start {
			cur = i
		}
	}
	if cur < 0 || t > tl.Phrases[cur].End+0.35 {
		return -1
	}
	return cur
}

func drawStyledCaption(dst *image.RGBA, th Theme, marks []CaptionWord, t float64) {
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
	if cs.Plate.A > 0 {
		wide := 0.0
		for _, line := range lines {
			w := -space
			for _, m := range line {
				w += float64(font.MeasureString(face, m.Text).Round()) + space
			}
			wide = math.Max(wide, w)
		}
		pad := cs.Size * 0.45
		dc := gg.NewContextForRGBA(dst)
		dc.SetColor(cs.Plate)
		dc.DrawRoundedRectangle(center-wide/2-pad, y-lh/2-pad*0.4, wide+2*pad, lh*float64(len(lines))+pad*0.8, pad*0.6)
		dc.Fill()
	}
	for _, line := range lines {
		width := -space
		for _, m := range line {
			width += float64(font.MeasureString(face, m.Text).Round()) + space
		}
		x := center - width/2
		for _, m := range line {
			col := cs.Ink
			if cs.Karaoke && t < m.At {
				col = cs.Dim
			}
			StrokeText(dst, face, m.Text, x, y, cs.Stroke, cs.Edge, col)
			x += float64(font.MeasureString(face, m.Text).Round()) + space
		}
		y += lh
	}
}

func wrapCaption(face font.Face, marks []CaptionWord, max float64) [][]CaptionWord {
	var lines [][]CaptionWord
	var line []CaptionWord
	w := 0.0
	space := float64(font.MeasureString(face, " ").Round())
	for _, m := range marks {
		mw := float64(font.MeasureString(face, m.Text).Round())
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

func StrokeText(dst *image.RGBA, face font.Face, s string, x, y, r float64, edge, ink color.RGBA) {
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
