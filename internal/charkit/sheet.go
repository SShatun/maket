package charkit

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"path/filepath"
	"sort"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const (
	sheetScale = 3
	sheetGap   = 16
)

type sheetFigure struct {
	name   string
	entry  Entry
	frames string
}

func Sheet(e Entry, frames string, c *Catalog, refs []string, roles map[string]color.RGBA, bg color.RGBA) (*image.RGBA, error) {
	var ids []string
	for _, st := range append([]string{e.Rest}, e.StateNames()...) {
		if st == e.Rest && len(ids) > 0 {
			continue
		}
		for i := 0; i < e.States[st].Frames; i++ {
			ids = append(ids, fmt.Sprintf("%s_%d", st, i))
		}
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("no frames to show")
	}
	figures := []sheetFigure{{filepath.Base(frames), e, frames}}
	for _, name := range refs {
		re, ok := c.Entries[name]
		if !ok {
			return nil, fmt.Errorf("no character %q", name)
		}
		figures = append(figures, sheetFigure{name, re, filepath.Join(c.Dir, name)})
	}
	left, right, up, down := 0, 0, 0, 0
	for _, f := range figures {
		left = max(left, f.entry.Anchor[0])
		right = max(right, f.entry.Size[0]-f.entry.Anchor[0])
		up = max(up, f.entry.Anchor[1])
		down = max(down, f.entry.Size[1]-f.entry.Anchor[1])
	}
	w := (left+right)/sheetScale + sheetGap*2
	h := (up+down)/sheetScale + sheetGap*3
	var roleNames []string
	for name := range roles {
		roleNames = append(roleNames, name)
	}
	sort.Strings(roleNames)
	columns := min(6, len(ids))
	compareRows := len(roleNames) + 1
	frameRows := (len(ids) + columns - 1) / columns
	out := image.NewRGBA(image.Rect(0, 0, sheetGap+max(columns, len(figures))*w, sheetGap+(compareRows+frameRows)*h+32))
	draw.Draw(out, out.Bounds(), image.NewUniform(bg), image.Point{}, draw.Src)
	ft, err := opentype.Parse(goregular.TTF)
	if err != nil {
		return nil, err
	}
	face, err := opentype.NewFace(ft, &opentype.FaceOptions{Size: 14, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return nil, err
	}
	defer face.Close()
	ink := color.RGBA{235, 237, 242, 255}
	if int(bg.R)*3+int(bg.G)*6+int(bg.B) > 1400 {
		ink = color.RGBA{35, 40, 52, 255}
	}
	text := func(label string, x, y int) {
		d := font.Drawer{Dst: out, Src: image.NewUniform(ink), Face: face, Dot: fixed.P(x, y)}
		d.DrawString(label)
	}
	white := color.RGBA{255, 255, 255, 255}
	paint := func(f sheetFigure, id string, tint color.RGBA, x, y int) error {
		im, err := ReadPNG(filepath.Join(f.frames, id+".png"))
		if err != nil {
			return err
		}
		if tint != white && (f.entry.Tint == nil || *f.entry.Tint) {
			im = Tint(im, tint)
		}
		base := y + sheetGap*2 + up/sheetScale
		x0 := x + sheetGap + (left-f.entry.Anchor[0])/sheetScale
		y0 := base - f.entry.Anchor[1]/sheetScale
		rect := image.Rect(x0, y0, x0+f.entry.Size[0]/sheetScale, y0+f.entry.Size[1]/sheetScale)
		line := image.Rect(x+sheetGap/2, base, x+w-sheetGap/2, base+1)
		draw.Draw(out, line, image.NewUniform(color.NRGBA{ink.R, ink.G, ink.B, 60}), image.Point{}, draw.Over)
		if f.entry.Pixel > 0 {
			xdraw.NearestNeighbor.Scale(out, rect, im, im.Bounds(), xdraw.Over, nil)
		} else {
			xdraw.CatmullRom.Scale(out, rect, im, im.Bounds(), xdraw.Over, nil)
		}
		text(f.name+" / "+id, x+sheetGap, y+14)
		return nil
	}
	for row, role := range append([]string{"original"}, roleNames...) {
		tint := white
		if row > 0 {
			tint = roles[role]
		}
		y := sheetGap + row*h
		for col, f := range figures {
			x := sheetGap + col*w
			if err := paint(f, f.entry.Rest+"_0", tint, x, y); err != nil {
				return nil, err
			}
			text(role, x+sheetGap, y+h-5)
		}
	}
	start := sheetGap + compareRows*h + 32
	text(figures[0].name+" / all frames", sheetGap*2, start-12)
	for i, id := range ids {
		if err := paint(figures[0], id, white, sheetGap+(i%columns)*w, start+(i/columns)*h); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func Tint(src image.Image, c color.RGBA) image.Image {
	b := src.Bounds()
	out := image.NewNRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			p := color.NRGBAModel.Convert(src.At(x, y)).(color.NRGBA)
			if p.A > 0 && p.R == p.G && p.G == p.B {
				p.R = uint8(int(p.R) * int(c.R) / 255)
				p.G = uint8(int(p.G) * int(c.G) / 255)
				p.B = uint8(int(p.B) * int(c.B) / 255)
			}
			out.SetNRGBA(x, y, p)
		}
	}
	return out
}
