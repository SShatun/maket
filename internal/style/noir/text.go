package noir

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"strings"

	"github.com/fogleman/gg"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"

	"maket/internal/render"
)

func (a *art) width(text string, px float64) float64 {
	f := render.Face(a.font, px)
	defer f.Close()
	return float64(font.MeasureString(f, text)) / 64
}

func (a *art) text(dst *image.RGBA, text string, at point, px, width float64, ink color.RGBA, centered bool) {
	if text == "" || width <= 0 {
		return
	}
	f := render.Face(a.font, px)
	w := font.MeasureString(f, text)
	if w > fixed.I(int(width)) {
		f.Close()
		f = render.Face(a.font, px*width/(float64(w)/64))
		w = font.MeasureString(f, text)
	}
	defer f.Close()
	x := fixed.Int26_6(math.Round(at[0] * 64))
	if centered {
		x -= w / 2
	}
	m := f.Metrics()
	d := font.Drawer{Dst: dst, Src: image.NewUniform(ink), Face: f, Dot: fixed.Point26_6{X: x, Y: fixed.Int26_6(at[1]*64) + (m.Ascent-m.Descent)/2}}
	d.DrawString(text)
}

func typedLines(first, second string, elapsed, dur float64) (string, string) {
	if dur <= 0 || elapsed >= dur {
		return first, second
	}
	a, b := []rune(first), []rune(second)
	n := int(clamp(elapsed/dur) * float64(len(a)+len(b)))
	return string(a[:min(n, len(a))]), string(b[:max(0, n-len(a))])
}

func (a *art) plate(dst *image.RGBA, title, sub string, elapsed, dur float64) {
	c := a.palette.Caption
	w := min(960, 80+int(math.Ceil(max(a.width(title, c.Title), a.width(sub, c.Sub)))))
	title, sub = typedLines(title, sub, elapsed, dur)
	draw.Draw(dst, image.Rect(60, 60, 60+w, 190), image.NewUniform(a.palette.Paper.ink()), image.Point{}, draw.Src)
	draw.Draw(dst, image.Rect(60, 60, 76, 190), image.NewUniform(a.palette.Red.ink()), image.Point{}, draw.Src)
	a.text(dst, title, point{104, 104}, c.Title, float64(w-76), a.palette.Ink.ink(), false)
	a.text(dst, sub, point{104, 157}, c.Sub, float64(w-76), a.palette.Red.ink(), false)
}

func (a *art) bubble(dst *image.RGBA, text string, r box, tip point, autoTip bool) {
	x0, y0, x1, y1 := float64(r[0]), float64(r[1]), float64(r[2]), float64(r[3])
	dc := gg.NewContextForRGBA(dst)
	dc.SetColor(a.palette.Paper.ink())
	dc.DrawRectangle(x0, y0, x1-x0, y1-y0)
	dc.Fill()
	var base, half point
	switch {
	case tip[0] < x0 && tip[1] >= y0:
		base, half = point{x0, y0 + 65}, point{0, 25}
	case tip[0] > x1 && tip[1] >= y0:
		base, half = point{x1, y0 + 65}, point{0, 25}
	case tip[1] > y1:
		base, half = point{x0 + 100, y1}, point{40, 0}
	default:
		base, half = point{x0 + 100, y0}, point{40, 0}
	}
	if autoTip {
		dx, dy := tip[0]-base[0], tip[1]-base[1]
		k := min(1., 100/max(1., math.Hypot(dx, dy)))
		tip = point{base[0] + dx*k, base[1] + dy*k}
	}
	dc.MoveTo(tip[0], tip[1])
	dc.LineTo(base[0]-half[0], base[1]-half[1])
	dc.LineTo(base[0]+half[0], base[1]+half[1])
	dc.ClosePath()
	dc.Fill()
	lines := strings.Split(text, "\n")
	px := a.palette.Speech.Px
	width := x1 - x0 - 20
	if len(lines) == 1 && a.width(text, px) > width {
		words := strings.Fields(text)
		best := math.Inf(1)
		for i := 1; i < len(words); i++ {
			l, r := strings.Join(words[:i], " "), strings.Join(words[i:], " ")
			diff := math.Abs(a.width(l, px) - a.width(r, px))
			if diff < best {
				best = diff
				lines = []string{l, r}
			}
		}
	}
	height := min(76., (y1-y0-12)/float64(len(lines)))
	px = min(px, height*.92)
	for _, s := range lines {
		if w := a.width(s, px); w > width {
			px *= width / w
		}
	}
	for i, s := range lines {
		y := (y0+y1)/2 + (float64(i)-float64(len(lines)-1)/2)*height
		a.text(dst, s, point{(x0 + x1) / 2, y}, px, width, a.palette.Ink.ink(), true)
	}
}

type neonSign struct {
	Text         string
	At           point
	Step, Px     float64
	Vertical     bool
	BehindBlinds bool `json:"behind_blinds"`
}

type neon struct {
	on, off *image.RGBA
	pos     image.Point
}

func (a *art) makeNeon(s neonSign) *neon {
	letters := []string{s.Text}
	if s.Vertical {
		letters = strings.Split(s.Text, "")
	}
	w := 0.
	for _, v := range letters {
		w = max(w, a.width(v, s.Px))
	}
	h := int(s.Px*2+float64(len(letters)-1)*s.Step) + 90
	r := image.Rect(0, 0, int(math.Ceil(w))+90, h)
	pos := image.Pt(int(s.At[0])-r.Dx()/2, int(s.At[1])-int(s.Px)-45)
	mask := image.NewRGBA(r)
	for i, v := range letters {
		p := point{float64(r.Dx()) / 2, s.Px + 45 + float64(i)*s.Step}
		a.text(mask, v, p, s.Px, w+1, color.RGBA{255, 255, 255, 255}, true)

	}
	return neonFromMask(mask, pos)
}

func (sh *shot) windowSign() *neon {
	region := image.Rect(680, 410, 1016, 588)
	mask := image.NewRGBA(image.Rect(0, 0, region.Dx(), region.Dy()))
	bg := sh.images[sh.BG]
	for y := 0; y < region.Dy(); y++ {
		for x := 0; x < region.Dx(); x++ {
			c := bg.RGBAAt(x+region.Min.X, y+region.Min.Y)
			a := uint8(min(255, max(0, int(c.R)-int(c.G))*255/170))
			mask.SetRGBA(x, y, color.RGBA{a, a, a, a})
		}
	}
	n := neonFromMask(mask, region.Min)
	for y := 0; y < region.Dy(); y++ {
		if bg.RGBAAt(710, y+region.Min.Y).G > 55 {
			clear(n.on.Pix[y*n.on.Stride:][:n.on.Stride])
		}
	}
	return n
}

func neonFromMask(mask *image.RGBA, pos image.Point) *neon {
	r := mask.Rect
	n := &neon{image.NewRGBA(r), image.NewRGBA(r), pos}
	draw.DrawMask(n.off, r, image.NewUniform(color.RGBA{90, 30, 34, 255}), image.Point{}, mask, image.Point{}, draw.Src)
	const radius = 42
	var weights [2*radius + 1]float64
	total := 0.
	for i := range weights {
		d := float64(i-radius) / 14
		weights[i] = math.Exp(-d * d / 2)
		total += weights[i]
	}
	tmp := make([]float64, r.Dx()*r.Dy())
	for y := 0; y < r.Dy(); y++ {
		for x := 0; x < r.Dx(); x++ {
			for dx := max(-radius, -x); dx <= min(radius, r.Dx()-1-x); dx++ {
				tmp[y*r.Dx()+x] += float64(mask.RGBAAt(x+dx, y).A) * weights[dx+radius] / total
			}
		}
	}
	for y := 0; y < r.Dy(); y++ {
		for x := 0; x < r.Dx(); x++ {
			glow := 0.
			for dy := max(-radius, -y); dy <= min(radius, r.Dy()-1-y); dy++ {
				glow += tmp[(y+dy)*r.Dx()+x] * weights[dy+radius] / total
			}
			alpha := uint8(glow * .8)
			n.on.SetRGBA(x, y, color.RGBA{alpha, uint8(int(alpha) * 60 / 255), uint8(int(alpha) * 60 / 255), alpha})
		}
	}
	draw.DrawMask(n.on, r, image.NewUniform(color.RGBA{255, 200, 196, 255}), image.Point{}, mask, image.Point{}, draw.Over)
	return n
}

func blinkGap(i int) int { return [...]int{79, 134, 97, 112}[i%4] }

func (n *neon) draw(dst *image.RGBA, dt float64) {
	frame := int(dt * render.FPS)
	im := n.on
	for i, gap := 0, 0; ; i++ {
		gap += blinkGap(i)
		if frame < gap {
			break
		}
		if frame < gap+2+i%2 {
			im = n.off
			break
		}
	}
	paste(dst, im, n.pos)
}
