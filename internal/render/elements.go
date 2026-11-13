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

func ShotLineLabels(dc *gg.Context, events []scene.Event, tl *timeline.Timeline, th Theme, t, end float64) {
	o := overlay{dc: dc, tl: tl, th: th, t: t}
	for _, ev := range events {
		if ev.Do != "line" || ev.Label == "" {
			continue
		}
		until := end
		if ev.Until != nil {
			until = math.Min(until, tl.MustResolve(*ev.Until))
		}
		if t < tl.MustResolve(ev.At) || t >= until {
			continue
		}
		o.lineLabel(ev.Label, ev.From.Coords[0], ev.From.Coords[1], ev.To.Coords[0], ev.To.Coords[1], o.tone(ev.Tone))
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
	o.panelLines(ev, x0, y0, x1, y1, width, ink)
}

func LineLabelBox(th Theme, label string, x0, y0, x1, y1 float64) (box [4]float64, ax, ay float64) {
	ax, ay = (x0+x1)/2, (y0+y1)/2
	bounds := func(x, y float64) [4]float64 {
		b := TextBounds(th, scene.Event{Text: label, Pos: []float64{x, y}, Size: 28})
		return [4]float64{b[0] - 8, b[1] - 8, b[2] + 8, b[3] + 8}
	}
	box = bounds(ax, ay)
	if math.Hypot(x1-x0, y1-y0) >= box[2]-box[0] {
		return box, ax, ay
	}
	dx, dy := x1-x0, y1-y0
	length := math.Hypot(dx, dy)
	nx, ny := 0.0, -1.0
	if length >= 1e-6 {
		nx, ny = -dy/length, dx/length
		if ny > 0 || ny == 0 && nx > 0 {
			nx, ny = -nx, -ny
		}
	}
	minProj := math.Inf(1)
	for _, c := range [][2]float64{{box[0], box[1]}, {box[2], box[1]}, {box[0], box[3]}, {box[2], box[3]}} {
		minProj = math.Min(minProj, (c[0]-ax)*nx+(c[1]-ay)*ny)
	}
	d := -minProj + 8
	ax += nx * d
	ay += ny * d
	return bounds(ax, ay), ax, ay
}

func (o overlay) lineLabel(label string, x0, y0, x1, y1 float64, col color.RGBA) {
	box, x, y := LineLabelBox(o.th, label, x0, y0, x1, y1)
	FrameBox(o.dc, box[0], box[1], box[2], box[3], col, o.th.Card, 6)
	o.text(Face(o.th.Fonts.Pixel, 28), label, x, y, col)
}

func (o overlay) panelLines(ev scene.Event, x0, y0, x1, y1, width float64, ink color.RGBA) {
	mono := o.th.Fonts.Mono
	n := max(1, len(ev.Lines))
	lh := math.Min(38, (y1-y0-40)/float64(n))
	size := math.Max(1, 25*math.Min(1, lh/38))
	plain := size
	for _, l := range ev.Lines {
		if l.Text == "" {
			continue
		}
		plain = math.Min(plain, numberSize(mono, plain, width, l.Text, false))
	}
	fit := func(base, limit float64, text string) float64 {
		return numberSize(mono, base, limit, text, false)
	}
	rows := make([]panelRow, len(ev.Lines))
	for i, l := range ev.Lines {
		explicit := true
		s := l.Size
		if s == 0 {
			s = ev.Size
		}
		if s == 0 {
			s = plain
			explicit = false
		}
		rows[i] = panelRow{size: s, lh: lh, shift: 7, explicit: explicit}
	}
	gap := 16.0
	for i, l := range ev.Lines {
		if l.Field == "" {
			continue
		}
		rows[i].size = math.Max(1, fit(rows[i].size, width, l.Field))
	}
	fieldWidth := func() float64 {
		w := 0.0
		for i, l := range ev.Lines {
			if l.Field == "" {
				continue
			}
			if m := float64(font.MeasureString(Face(mono, rows[i].size), l.Field).Round()); m > w {
				w = m
			}
		}
		return w
	}
	fieldW := fieldWidth()
	valueLimit := math.Max(1, width-fieldW-gap)
	for i, l := range ev.Lines {
		limit := width
		if l.Field != "" {
			limit = valueLimit
		}
		for _, text := range panelSamples(l) {
			rows[i].size = fit(rows[i].size, limit, text)
		}
		rows[i].size = math.Max(1, rows[i].size)
	}
	for i := range rows {
		if rows[i].explicit {
			rows[i].lh, rows[i].shift = 38*rows[i].size/25, 7*rows[i].size/25
		}
	}
	avail := math.Max(1, y1-y0-40)
	var need float64
	for _, r := range rows {
		need += r.lh
	}
	if len(rows) > 0 && need > avail {
		k := avail / need
		for i := range rows {
			rows[i].size = math.Max(1, rows[i].size*k)
			rows[i].lh *= k
			if rows[i].explicit {
				rows[i].shift = 7 * rows[i].size / 25
			}
		}
	}
	fieldW = fieldWidth()
	valueX := x0 + 26 + fieldW + gap
	y := y0 + 22
	for i, l := range ev.Lines {
		if l.Appear != nil && o.t < o.tl.MustResolve(*l.Appear) {
			continue
		}
		o.drawPanelRow(ev, l, x0, x1, y, rows[i].lh, rows[i].shift, valueX, ink, Face(mono, rows[i].size))
		y += rows[i].lh
	}
}

type panelRow struct {
	size, lh, shift float64
	explicit        bool
}

func (o overlay) drawPanelRow(ev scene.Event, l scene.PanelLine, x0, x1, y, lh, shift, valueX float64, ink color.RGBA, face font.Face) {
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
	baseline := y + lh/2 - shift
	text := panelText(l, o.tl, o.t)
	if l.Field != "" {
		o.dc.DrawStringAnchored(l.Field, x0+26, baseline, 0, 0.5)
		o.dc.DrawStringAnchored(text, valueX, baseline, 0, 0.5)
		return
	}
	o.dc.DrawStringAnchored(text, x0+26, baseline, 0, 0.5)
}

func panelSamples(l scene.PanelLine) []string {
	if len(l.Timed) == 0 {
		return []string{l.Text}
	}
	out := make([]string, len(l.Timed))
	for i, x := range l.Timed {
		out[i] = x.V
	}
	return out
}

func panelText(l scene.PanelLine, tl *timeline.Timeline, t float64) string {
	if len(l.Timed) == 0 {
		return l.Text
	}
	v := ""
	for _, x := range l.Timed {
		if t >= tl.MustResolve(x.At) {
			v = x.V
		}
	}
	return v
}
