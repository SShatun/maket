package park

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"strings"

	"github.com/fogleman/gg"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"

	"maket/internal/render"
	"maket/internal/scene"
	"maket/internal/timeline"
)

type caption struct {
	at, end  float64
	words    []render.CaptionWord
	edges    []int
	ink, dim *image.RGBA
}

func newCaption(fonts *render.Fonts, p scene.Phrase, pt timeline.PhraseTime) (caption, int) {
	words := render.CaptionWords(p, pt)
	c := caption{at: pt.Start, end: pt.End, words: words}
	if len(words) == 0 {
		return c, 0
	}
	face := render.Face(fonts.Bold, 50)
	defer face.Close()
	space := font.MeasureString(face, " ")
	width := -space
	for _, w := range words {
		width += font.MeasureString(face, w.Text) + space
	}
	x := (fixed.I(render.W) - width) / 2
	const height = 97
	dc := gg.NewContext(render.W, height)
	dc.SetRGBA255(24, 16, 34, 200)
	dc.DrawRoundedRectangle(float64(x.Round()-34), 0, float64(width.Round()+69), height, 22)
	dc.Fill()
	c.dim = dc.Image().(*image.RGBA)
	c.ink = image.NewRGBA(c.dim.Bounds())
	copy(c.ink.Pix, c.dim.Pix)
	edge := color.RGBA{30, 20, 40, 255}
	for i, w := range words {
		for _, paint := range []struct {
			layer *image.RGBA
			col   color.RGBA
		}{
			{c.ink, color.RGBA{255, 255, 255, 255}},
			{c.dim, color.RGBA{190, 190, 200, 255}},
		} {
			render.StrokeText(paint.layer, face, w.Text, float64(x)/64, 45.5, 7, edge, paint.col)
		}
		x += font.MeasureString(face, w.Text) + space
		end := min(render.W, max(0, (x-space/2).Round()))
		if i == len(words)-1 {
			end = render.W
		}
		c.edges = append(c.edges, end)
	}
	return c, width.Round()
}

func (c caption) draw(dst *image.RGBA, t float64) {
	x := 0
	for i, w := range c.words {
		src := c.dim
		if t > w.At {
			src = c.ink
		}
		r := image.Rect(x, 0, c.edges[i], src.Bounds().Dy())
		draw.Draw(dst, r.Add(image.Pt(0, 1653)), src, r.Min, draw.Over)
		x = c.edges[i]
	}
}

func titleImage(fonts *render.Fonts, episode int, line string) (*image.RGBA, error) {
	if episode < 0 || strings.TrimSpace(line) == "" || strings.ContainsAny(line, "\r\n") {
		return nil, fmt.Errorf("title needs episode and one nonempty line")
	}
	im := image.NewRGBA(image.Rect(0, 0, 901, 251))
	fill := func(r image.Rectangle, c color.RGBA) { draw.Draw(im, r, image.NewUniform(c), image.Point{}, draw.Src) }
	edge := color.RGBA{40, 24, 10, 255}
	fill(image.Rect(0, 40, 901, 251), edge)
	fill(image.Rect(8, 48, 893, 243), color.RGBA{250, 214, 70, 255})
	for i := 0; i < 10; i++ {
		col := color.RGBA{230, 60, 80, 255}
		if i%2 == 1 {
			col = color.RGBA{255, 250, 240, 255}
		}
		fill(image.Rect(i*90, 0, i*90+45, 40), col)
	}
	semi, bold := render.Face(fonts.Semi, 36), render.Face(fonts.Bold, 70)
	defer semi.Close()
	defer bold.Close()
	center := func(face font.Face, text string, y int, c color.RGBA) {
		m := face.Metrics()
		d := font.Drawer{Dst: im, Src: image.NewUniform(c), Face: face, Dot: fixed.Point26_6{
			X: fixed.I(450) - font.MeasureString(face, text)/2,
			Y: fixed.I(y) + (m.Ascent-m.Descent)/2,
		}}
		d.DrawString(text)
	}
	center(semi, fmt.Sprintf("ЭПИЗОД %d", episode), 110, color.RGBA{120, 40, 30, 255})
	center(bold, line, 180, edge)
	return im, nil
}
