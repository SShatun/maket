package paper

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"sort"
	"strings"

	"github.com/fogleman/gg"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/math/f64"

	"maket/internal/render"
	"maket/internal/style/sprites"
)

func (s *Style) Draw(dst *image.RGBA, t float64) {
	if len(s.shots) == 0 {
		return
	}
	i := sort.Search(len(s.shots), func(i int) bool { return s.shots[i].at > t }) - 1
	sh := &s.shots[max(0, i)]
	bg := s.backgrounds[sh.backgroundAt(t)]
	draw.Draw(dst, dst.Bounds(), bg.image, image.Point{}, draw.Src)
	if sh.backgroundAt(t) == "classroom" {
		for _, text := range sh.chalk {
			if t >= text.at && t < text.until {
				s.drawChalk(dst, bg, text, t)
			}
		}
	}
	stepped := sh.at + math.Floor((t-sh.at)*12+1e-8)/12
	speaker := ""
	phrase := render.CaptionPhrase(s.sc, s.tl, t)
	if phrase >= 0 {
		speaker = s.sc.Phrases[phrase].Speaker
	}
	type placed struct {
		a    *actor
		x, y float64
	}
	actors := make([]placed, 0, len(sh.actors))
	for _, a := range sh.actors {
		x, y := a.position(stepped)
		actors = append(actors, placed{a, x, y})
	}
	sort.SliceStable(actors, func(i, j int) bool { return actors[i].y < actors[j].y })
	for _, p := range actors {
		a := p.a
		role := a.roles[0].name
		for _, r := range a.roles {
			if t >= r.at {
				role = r.name
			}
		}
		states := a.states
		if a.id == speaker {
			state := s.talkingState(a)
			if state == "talk" || s.set.StateAt(a.kind, states, t) == "idle" {
				states = []sprites.Step{{T: s.tl.Phrases[phrase].Start, V: state}}
			}
		}
		im := s.set.Frame(a.kind, states, t, role)
		if im == nil {
			continue
		}
		spec := s.set.Specs[a.kind]
		sx, ax := a.size, float64(spec.Anchor[0])
		if a.flip {
			sx = -sx
		}
		m := f64.Aff3{sx, 0, p.x - ax*sx, 0, a.size, p.y - float64(spec.Anchor[1])*a.size}
		xdraw.ApproxBiLinear.Transform(dst, m, im, im.Bounds(), draw.Over, nil)
		if a.label != nil {
			drawCentered(dst, a.label, p.x, p.y+46)
		}
	}
	for _, l := range sh.labels {
		if t >= l.at && t < l.until {
			drawCentered(dst, l.image, l.pos[0], l.pos[1])
		}
	}
	for _, title := range sh.titles {
		if t < title.at || t >= title.until {
			continue
		}
		k := math.Min(1, (t-title.at)/0.4)
		y := -float64(title.image.Bounds().Dy()) + (600+float64(title.image.Bounds().Dy()))*(1-math.Pow(1-k, 3))
		drawCentered(dst, title.image, 540, y)
	}
	render.Caption(dst, s.sc, s.tl, s.theme, t)
	for _, c := range s.cuts {
		k := math.Max(0, 1-math.Abs(t-c.at)/(c.dur/2))
		if k <= 0 {
			continue
		}
		v := uint8(0)
		if c.white {
			v = 255
		}
		draw.Draw(dst, dst.Bounds(), image.NewUniform(color.NRGBA{v, v, v, uint8(math.Round(255 * k))}), image.Point{}, draw.Over)
	}
}

func drawCentered(dst *image.RGBA, src *image.RGBA, x, y float64) {
	b := src.Bounds()
	p := image.Pt(int(math.Round(x-float64(b.Dx())/2)), int(math.Round(y-float64(b.Dy())/2)))
	draw.Draw(dst, b.Add(p), src, b.Min, draw.Over)
}

func (s *Style) textImage(text string, size float64, col color.RGBA) (*image.RGBA, error) {
	if strings.TrimSpace(text) == "" || strings.ContainsAny(text, "\n\r") {
		return nil, fmt.Errorf("text needs one nonempty line")
	}
	if size <= 0 || size > 300 {
		return nil, fmt.Errorf("text size must be in (0, 300]")
	}
	face := render.Face(s.font, size)
	defer face.Close()
	w := font.MeasureString(face, text).Ceil() + 12
	if w > 1000 {
		return nil, fmt.Errorf("text is too wide; shorten it or reduce size")
	}
	h := face.Metrics().Height.Ceil() + 12
	dc := gg.NewContext(w, h)
	dc.SetColor(col)
	dc.SetFontFace(face)
	dc.DrawStringAnchored(text, float64(w)/2, float64(h)/2, 0.5, 0.5)
	return dc.Image().(*image.RGBA), nil
}

func (s *Style) titleImage(episode int, lines []string) (*image.RGBA, error) {
	if episode < 0 || len(lines) < 1 || len(lines) > 2 {
		return nil, fmt.Errorf("title needs nonnegative episode and one or two lines")
	}
	dc := gg.NewContext(980, 450)
	dc.SetRGB255(248, 244, 226)
	dc.DrawRectangle(32, 32, 916, 386)
	dc.Fill()
	dc.SetRGBA255(215, 202, 164, 210)
	for _, x := range []float64{110, 775} {
		dc.DrawRectangle(x, 12, 105, 58)
		dc.Fill()
	}
	face := render.Face(s.font, 44)
	defer face.Close()
	dc.SetFontFace(face)
	dc.SetRGB255(150, 60, 50)
	dc.DrawStringAnchored(fmt.Sprintf("ЭПИЗОД %d", episode), 490, 110, 0.5, 0.5)
	for i, line := range lines {
		size := 104.0
		if i > 0 {
			size = 92
		}
		f := render.Face(s.font, size)
		if font.MeasureString(f, line).Ceil() > 860 {
			f.Close()
			return nil, fmt.Errorf("title line %d is too wide; use two short lines", i+1)
		}
		dc.SetFontFace(f)
		dc.SetRGB255(24, 20, 24)
		dc.DrawStringAnchored(line, 490, 220+float64(i)*110, 0.5, 0.5)
		f.Close()
	}
	return dc.Image().(*image.RGBA), nil
}

func (s *Style) checkChalk(lines []string) error {
	rect := s.backgrounds["classroom"].Board.Rect
	if rect[2] <= rect[0] || rect[3] <= rect[1] {
		return fmt.Errorf("classroom needs a board rect")
	}
	face := render.Face(s.font, 60)
	defer face.Close()
	for i, line := range lines {
		if strings.ContainsAny(line, "\n\r") || 120+font.MeasureString(face, line).Ceil() > rect[2]-20 || 340+74*i+face.Metrics().Descent.Ceil() > rect[3]-20 {
			return fmt.Errorf("chalk line %d exceeds the classroom board", i+1)
		}
	}
	return nil
}

func (s *Style) drawChalk(dst *image.RGBA, bg backdrop, text lettering, t float64) {
	r := bg.Board.Rect
	board := dst.SubImage(image.Rect(r[0], r[1], r[2], r[3])).(*image.RGBA)
	layer := image.NewRGBA(dst.Bounds())
	dc := gg.NewContextForRGBA(layer)
	face := render.Face(s.font, 60)
	defer face.Close()
	dc.SetFontFace(face)
	c := bg.Board.Chalk
	dc.SetRGB255(int(c[0]), int(c[1]), int(c[2]))
	count := int(math.Floor((t - text.at) * text.cps))
	for i, line := range text.lines {
		runes := []rune(line)
		n := min(count, len(runes))
		if n > 0 {
			dc.DrawString(string(runes[:n]), 120, 340+74*float64(i))
		}
		count -= n
		if count <= 0 {
			break
		}
	}
	draw.Draw(board, board.Bounds(), layer, board.Bounds().Min, draw.Over)
}
