package park

import (
	"image"
	"image/color"
	"image/draw"
	"strconv"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"

	"maket/internal/render"
)

type label struct{ kind, text, tone string }

func inkText(im *image.RGBA, face font.Face, text string, center image.Point, c color.RGBA) {
	b, _ := font.BoundString(face, text)
	mask := image.NewAlpha(im.Bounds())
	d := font.Drawer{Dst: mask, Src: image.White, Face: face, Dot: fixed.P(center.X-(b.Min.X+b.Max.X).Round()/2, center.Y-(b.Min.Y+b.Max.Y).Round()/2)}
	d.DrawString(text)
	for y := 0; y < mask.Bounds().Dy(); y++ {
		for x := 0; x < mask.Bounds().Dx(); x++ {
			if mask.AlphaAt(x, y).A >= 128 {
				im.SetRGBA(x, y, c)
			}
		}
	}
}

func (sh *shot) prepareBoards() {
	face := render.Face(sh.theme.Fonts.MonoBold, 11)
	defer face.Close()
	for _, o := range sh.objects {
		b := sh.sprites.Boards[o.kind]
		if b.PNG == "" {
			continue
		}
		values := []string{""}
		for _, m := range o.values {
			values = append(values, m.value)
		}
		if o.link != nil {
			for n := 0; n <= o.link.seats; n++ {
				values = append(values, strconv.Itoa(n))
			}
		}
		for _, v := range values {
			for tone, col := range b.Colors {
				key := label{o.kind, v, tone}
				if sh.labels[key] != nil {
					continue
				}
				base := sh.images[b.PNG]
				im := image.NewRGBA(base.Bounds())
				copy(im.Pix, base.Pix)
				inkText(im, face, v, b.Centre.pixel(), color.RGBA{col[0], col[1], col[2], 255})
				sh.labels[key] = im
			}
		}
	}
}

func bubbleKey(msg string) string {
	return "fx/bubble_" + strings.ReplaceAll(strings.TrimRight(msg, "!.?"), " ", "_") + ".png"
}

func (sh *shot) bubble(msg string) *image.RGBA {
	if im := sh.images[bubbleKey(msg)]; im != nil {
		return im
	}
	return sh.labels[label{kind: "bubble", text: msg}]
}

func (sh *shot) prepareBubble(msg string) {
	if sh.bubble(msg) != nil {
		return
	}
	face := render.Face(sh.theme.Fonts.MonoBold, 8)
	defer face.Close()
	w := font.MeasureString(face, msg).Ceil() + 8
	im := image.NewRGBA(image.Rect(0, 0, w, 19))
	edge := color.RGBA{40, 26, 46, 255}
	fill := func(r image.Rectangle, c color.RGBA) { draw.Draw(im, r, image.NewUniform(c), image.Point{}, draw.Src) }
	fill(image.Rect(0, 0, w, 15), edge)
	fill(image.Rect(1, 1, w-1, 14), color.RGBA{255, 255, 255, 255})
	for i := 0; i < 4; i++ {
		fill(image.Rect(2, 14+i, 7-i, 15+i), edge)
		fill(image.Rect(3, 14+i, 6-i, 15+i).Intersect(image.Rect(3, 14, 6, 17)), color.RGBA{255, 255, 255, 255})
	}
	inkText(im, face, msg, image.Pt(w/2, 7), color.RGBA{30, 26, 40, 255})
	sh.labels[label{kind: "bubble", text: msg}] = im
}
