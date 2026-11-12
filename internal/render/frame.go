package render

import (
	"image"
	"image/color"
	"math"

	"github.com/fogleman/gg"
	"golang.org/x/image/font"

	"maket/internal/scene"
	"maket/internal/timeline"
)

const (
	W, H = 1080, 1920
	FPS  = 30

	stageBottom    = 1100
	captionY       = 1458
	captionH       = 84
	captionX       = 510
	codeX0, codeX1 = 70.0, 950.0
	codeY0         = 1120.0
)

type Tone struct {
	BG, FG color.RGBA
}

type Theme struct {
	Pixel   bool
	BG      color.RGBA
	Ink     color.RGBA
	Card    color.RGBA
	Muted   color.RGBA
	CodeDim color.RGBA
	Accent  color.RGBA
	Error   color.RGBA
	OK      color.RGBA
	OKDark  color.RGBA
	Grid    color.RGBA
	Fonts   *Fonts
	Tones   map[string]Tone
	Badges  map[string]image.Image
}

func rgb(r, g, b uint8) color.RGBA { return color.RGBA{r, g, b, 255} }

func LightTheme(f *Fonts) Theme {
	return Theme{
		BG: rgb(220, 227, 232), Ink: rgb(31, 41, 51), Card: rgb(250, 251, 252), Muted: rgb(84, 95, 106),
		CodeDim: rgb(120, 132, 144), Accent: rgb(250, 251, 252), Error: rgb(229, 72, 77),
		OK: rgb(46, 160, 67), OKDark: rgb(46, 160, 67), Grid: rgb(211, 219, 225), Fonts: f,
		Tones: map[string]Tone{
			"yel":      {rgb(253, 239, 205), rgb(31, 41, 51)},
			"error":    {rgb(252, 226, 227), rgb(176, 38, 46)},
			"ok":       {rgb(223, 243, 227), rgb(31, 41, 51)},
			"lav":      {rgb(234, 228, 252), rgb(31, 41, 51)},
			"producer": {rgb(220, 242, 237), rgb(31, 41, 51)},
			"consumer": {rgb(234, 228, 252), rgb(31, 41, 51)},
			"buffer":   {rgb(253, 239, 205), rgb(31, 41, 51)},
			"storage":  {rgb(222, 233, 252), rgb(31, 41, 51)},
			"external": {rgb(231, 235, 239), rgb(31, 41, 51)},
		},
	}
}

func PixelTheme(f *Fonts, badges map[string]image.Image, over map[string]color.RGBA) Theme {
	t := Theme{
		Pixel: true, BG: rgb(29, 43, 83), Ink: rgb(255, 241, 232), Card: rgb(16, 24, 50),
		Muted: rgb(194, 195, 199), CodeDim: rgb(120, 132, 170), Accent: rgb(255, 236, 39),
		Error: rgb(255, 0, 77), OK: rgb(0, 228, 54), OKDark: rgb(0, 135, 81), Grid: rgb(131, 118, 156),
		Fonts: f, Badges: badges,
		Tones: map[string]Tone{
			"yel":   {rgb(70, 64, 30), rgb(255, 236, 39)},
			"error": {rgb(90, 20, 50), rgb(255, 120, 150)},
			"ok":    {rgb(10, 80, 40), rgb(0, 228, 54)},
			"lav":   {rgb(60, 54, 90), rgb(255, 241, 232)},
		},
	}
	for k, c := range over {
		switch k {
		case "bg":
			t.BG = c
		case "ink":
			t.Ink = c
		case "card":
			t.Card = c
		case "muted":
			t.Muted = c
		case "accent":
			t.Accent = c
		case "error":
			t.Error = c
		case "ok":
			t.OK = c
		case "grid":
			t.Grid = c
		}
	}
	return t
}

func Overlay(dst *image.RGBA, sc *scene.Scene, tl *timeline.Timeline, th Theme, t float64) {
	dc := gg.NewContextForRGBA(dst)
	o := overlay{dc: dc, sc: sc, tl: tl, th: th, t: t}
	o.dim()
	o.guess()
	o.answer()
	o.numbers()
	o.code()
	o.caption()
	o.progress()
}

type overlay struct {
	dc *gg.Context
	sc *scene.Scene
	tl *timeline.Timeline
	th Theme
	t  float64
}

func (o overlay) guessEnd() float64 {
	if len(o.tl.Phrases) == 0 {
		return 1.4
	}
	return o.tl.Phrases[0].End
}

func (o overlay) dim() {
	k := 0.0
	if o.sc.Guess != nil {
		k = math.Max(k, 0.65*(1-prog(o.t, o.guessEnd(), o.guessEnd()+0.2)))
	}
	if a := o.sc.Answer; a != nil && o.sc.Guess != nil {
		at := o.tl.MustResolve(a.At)
		k = math.Max(k, 0.65*prog(o.t, at, at+0.3))
	}
	for _, ev := range o.sc.Events {
		if ev.Do != "number" {
			continue
		}
		a, b := o.span(ev)
		if o.t >= a && o.t < b+0.2 {
			k = math.Max(k, 0.7*(1-prog(o.t, b, b+0.2)))
		}
	}
	if k <= 0 {
		return
	}
	c := o.th.BG
	o.dc.SetRGBA255(int(c.R), int(c.G), int(c.B), int(255*k))
	o.dc.DrawRectangle(0, 140, W, stageBottom-140)
	o.dc.Fill()
}

func (o overlay) span(ev scene.Event) (float64, float64) {
	a := o.tl.MustResolve(ev.At)
	b := a + 2
	if ev.Until != nil {
		b = o.tl.MustResolve(*ev.Until)
	}
	return a, b
}

func (o overlay) guess() {
	if o.sc.Guess == nil || o.t >= o.guessEnd()+0.2 {
		return
	}
	k := o.pop(0.05)
	if k <= 0 {
		return
	}
	o.guessCard(o.sc.Guess.Text, 560, 0, k)
}

func (o overlay) answer() {
	a := o.sc.Answer
	if a == nil || o.sc.Guess == nil {
		return
	}
	at := o.tl.MustResolve(a.At)
	if o.t < at {
		return
	}
	o.guessCard(o.sc.Guess.Text, 470, prog(o.t, at+0.25, at+0.55), 1)
	ct := at + 0.7
	if a.CardAt != nil {
		ct = o.tl.MustResolve(*a.CardAt)
	}
	k := o.pop(ct)
	if k <= 0 {
		return
	}
	size := a.Size
	if size == 0 {
		size = 70
	}
	y0, h := 740.0, 230*k
	if o.th.Pixel {
		FrameBox(o.dc, 140, y0, 940, y0+h, o.th.OK, o.th.OKDark, 8)
	} else {
		o.dc.SetColor(o.th.OK)
		o.dc.DrawRoundedRectangle(140, y0, 800, h, 22)
		o.dc.Fill()
	}
	if k < 1 {
		return
	}
	o.text(o.label(28), "на самом деле", 540, y0+52, white)
	o.text(o.display(size), a.Text, 540, y0+140, white)
	if b := o.th.Badges[a.Badge]; b != nil {
		o.dc.DrawImageAnchored(b, 940, int(y0), 0.5, 0.5)
	}
}

var white = rgb(255, 255, 255)

func (o overlay) guessCard(text string, y, struck, k float64) {
	h := 230 * k
	if o.th.Pixel {
		FrameBox(o.dc, 140, y, 940, y+h, o.th.Ink, o.th.BG, 8)
	} else {
		o.dc.SetColor(o.th.Card)
		o.dc.DrawRoundedRectangle(140, y, 800, h, 22)
		o.dc.Fill()
	}
	if k < 1 {
		return
	}
	o.text(o.label(28), "догадка", 540, y+58, o.th.Muted)
	col := o.th.Accent
	if !o.th.Pixel {
		col = o.th.Ink
	}
	if struck > 0 {
		col = o.th.CodeDim
	}
	face := o.display(76)
	o.text(face, text, 540, y+140, col)
	if struck > 0 {
		tw := font.MeasureString(face, text).Round()
		x0 := 540 - float64(tw)/2 - 20
		o.dc.SetColor(o.th.Error)
		o.dc.DrawRectangle(x0, y+136, (float64(tw)+40)*struck, 10)
		o.dc.Fill()
	}
}

func (o overlay) numbers() {
	for _, ev := range o.sc.Events {
		if ev.Do != "number" {
			continue
		}
		a, b := o.span(ev)
		if o.t < a || o.t >= b {
			continue
		}
		k := o.pop(a)
		if k <= 0 {
			continue
		}
		if o.th.Pixel {
			h := 400 * k
			FrameBox(o.dc, 140, 620-h/2, 940, 620+h/2, o.th.Ink, o.th.BG, 8)
			if k < 1 {
				continue
			}
			o.text(Face(o.th.Fonts.Pixel, 200), ev.Text, 540, 570, o.th.Accent)
			o.text(Face(o.th.Fonts.Pixel, 30), ev.Sub, 540, 740, o.th.Ink)
			continue
		}
		o.text(Face(o.th.Fonts.Bold, 260*k), ev.Text, 540, 560, o.th.Ink)
		if k == 1 {
			o.text(Face(o.th.Fonts.Semi, 44), ev.Sub, 540, 730, o.th.Muted)
		}
	}
}

func (o overlay) code() {
	c := o.sc.Code
	if c == nil {
		return
	}
	y1 := 1390.0
	if o.th.Pixel {
		FrameBox(o.dc, codeX0, codeY0, codeX1, y1, o.th.Ink, o.th.Card, 6)
	} else {
		y1 = 1380
		o.dc.SetColor(o.th.Card)
		o.dc.DrawRoundedRectangle(codeX0, codeY0, codeX1-codeX0, y1-codeY0, 22)
		o.dc.Fill()
	}
	tones := map[int]string{}
	for _, h := range c.Highlight {
		if o.t >= o.tl.MustResolve(h.At) {
			tones[h.Line] = h.Tone
		}
	}
	var lines []int
	for i, l := range c.Lines {
		if l.Appear == nil || o.t >= o.tl.MustResolve(*l.Appear) {
			lines = append(lines, i)
		}
	}
	n := len(c.Lines)
	lh := math.Min(38, (y1-codeY0-40)/float64(max(1, n)))
	face := Face(o.th.Fonts.Mono, 25*math.Min(1, lh/38))
	for row, i := range lines {
		y := codeY0 + 22 + float64(row)*lh
		col := o.th.CodeDim
		if tone, ok := o.th.Tones[tones[i]]; ok {
			o.dc.SetColor(tone.BG)
			if o.th.Pixel {
				o.dc.DrawRectangle(codeX0+12, y-4, codeX1-codeX0-24, lh-2)
			} else {
				o.dc.DrawRoundedRectangle(codeX0+16, y-4, codeX1-codeX0-32, lh-2, 8)
			}
			o.dc.Fill()
			col = tone.FG
		}
		o.dc.SetFontFace(face)
		o.dc.SetColor(col)
		o.dc.DrawStringAnchored(c.Lines[i].Text, codeX0+26, y+lh/2-7, 0, 0.5)
	}
}

func (o overlay) caption() {
	cur := -1
	for i, p := range o.tl.Phrases {
		if i < len(o.sc.Phrases) && o.t >= p.Start {
			cur = i
		}
	}
	if cur >= 0 && o.sc.Phrases[cur].Caption != "" && o.t <= o.tl.Phrases[cur].End+0.35 {
		text := o.sc.Phrases[cur].Caption
		size := 42.0
		if o.th.Pixel {
			size = 44
		}
		face := Face(o.th.Fonts.Bold, size)
		tw := float64(font.MeasureString(face, text).Round())
		x0, x1 := captionX-tw/2-30, captionX+tw/2+30
		if o.th.Pixel {
			FrameBox(o.dc, x0, captionY-captionH/2, x1, captionY+captionH/2, o.th.Ink, o.th.BG, 6)
			o.text(face, text, captionX, captionY, o.th.Accent)
		} else {
			o.dc.SetColor(o.th.Ink)
			o.dc.DrawRoundedRectangle(x0, captionY-captionH/2, x1-x0, captionH, captionH/2)
			o.dc.Fill()
			o.text(face, text, captionX, captionY, o.th.Card)
		}
	}
}

func (o overlay) progress() {
	p := clamp(o.t / o.tl.Duration)
	if o.th.Pixel {
		FrameBox(o.dc, 70, 112, 1010, 136, o.th.Ink, o.th.BG, 4)
		w := math.Floor(928*p/6) * 6
		if w > 0 {
			o.dc.SetColor(o.th.Accent)
			o.dc.DrawRectangle(76, 118, w, 12)
			o.dc.Fill()
		}
		return
	}
	o.dc.SetColor(o.th.Grid)
	o.dc.DrawRoundedRectangle(70, 120, 940, 8, 4)
	o.dc.Fill()
	if p > 0 {
		o.dc.SetColor(o.th.Ink)
		o.dc.DrawRoundedRectangle(70, 120, 940*p, 8, 4)
		o.dc.Fill()
	}
}

func (o overlay) label(size float64) font.Face {
	if o.th.Pixel {
		return Face(o.th.Fonts.Pixel, size*0.62)
	}
	return Face(o.th.Fonts.Semi, size)
}

func (o overlay) display(size float64) font.Face {
	if o.th.Pixel {
		return Face(o.th.Fonts.Pixel, size*0.62)
	}
	return Face(o.th.Fonts.Bold, size)
}

func (o overlay) text(face font.Face, s string, x, y float64, c color.RGBA) {
	o.dc.SetFontFace(face)
	o.dc.SetColor(c)
	o.dc.DrawStringAnchored(s, x, y, 0.5, 0.35)
}

func (o overlay) pop(t0 float64) float64 {
	return Pop(o.th.Pixel, o.t-t0)
}

func Pop(pixel bool, dt float64) float64 {
	if dt < 0 {
		return 0
	}
	if pixel {
		switch {
		case dt < 1.0/FPS:
			return 0.55
		case dt < 2.0/FPS:
			return 1.12
		case dt < 3.0/FPS:
			return 0.96
		}
		return 1
	}
	if dt >= 0.35 {
		return 1
	}
	x := dt / 0.35
	const c1, c3 = 1.70158, 2.70158
	return 1 + c3*math.Pow(x-1, 3) + c1*math.Pow(x-1, 2)
}

func FrameBox(dc *gg.Context, x0, y0, x1, y1 float64, border, fill color.RGBA, bw float64) {
	dc.SetColor(border)
	dc.DrawRectangle(x0, y0, x1-x0, y1-y0)
	dc.Fill()
	dc.SetColor(fill)
	dc.DrawRectangle(x0+bw, y0+bw, x1-x0-2*bw, y1-y0-2*bw)
	dc.Fill()
}

func clamp(x float64) float64 { return math.Max(0, math.Min(1, x)) }

func prog(t, a, b float64) float64 {
	if b <= a {
		if t >= a {
			return 1
		}
		return 0
	}
	return clamp((t - a) / (b - a))
}
