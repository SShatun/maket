package charkit

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"path/filepath"
	"strconv"
	"strings"

	xdraw "golang.org/x/image/draw"
)

const (
	sheetScale = 3
	sheetGap   = 16
)

type sheetRow struct {
	entry  Entry
	frames string
	tint   color.RGBA
}

func Sheet(e Entry, frames string, c *Catalog, refs []string, roles []color.RGBA, bg color.RGBA) (*image.RGBA, error) {
	var cols []string
	for _, st := range append([]string{e.Rest}, e.StateNames()...) {
		if st == e.Rest && len(cols) > 0 {
			continue
		}
		for i := 0; i < e.States[st].Frames; i++ {
			cols = append(cols, fmt.Sprintf("%s_%d", st, i))
		}
	}
	white := color.RGBA{255, 255, 255, 255}
	rows := []sheetRow{{e, frames, white}}
	for _, r := range refs {
		re, ok := c.Entries[r]
		if !ok {
			return nil, fmt.Errorf("no character %q", r)
		}
		rows = append(rows, sheetRow{re, filepath.Join(c.Dir, r), white})
	}
	tinted := ""
	for _, r := range refs {
		if t := c.Entries[r].Tint; t == nil || *t {
			tinted = r
			break
		}
	}
	for _, role := range roles {
		rows = append(rows, sheetRow{e, frames, role})
		if tinted != "" {
			rows = append(rows, sheetRow{c.Entries[tinted], filepath.Join(c.Dir, tinted), role})
		}
	}
	w, h := e.Size[0]/sheetScale, e.Size[1]/sheetScale
	out := image.NewRGBA(image.Rect(0, 0, sheetGap+len(cols)*(w+sheetGap), sheetGap+len(rows)*(h+sheetGap)))
	draw.Draw(out, out.Bounds(), image.NewUniform(bg), image.Point{}, draw.Src)
	for ri, row := range rows {
		for ci, col := range cols {
			id := sameFrame(row.entry, col)
			im, err := ReadPNG(filepath.Join(row.frames, id+".png"))
			if err != nil {
				return nil, err
			}
			if row.tint != white && (row.entry.Tint == nil || *row.entry.Tint) {
				im = Tint(im, row.tint)
			}
			x, y := sheetGap+ci*(w+sheetGap), sheetGap+ri*(h+sheetGap)
			xdraw.NearestNeighbor.Scale(out, image.Rect(x, y, x+w, y+h), im, im.Bounds(), xdraw.Over, nil)
		}
	}
	return out, nil
}

func sameFrame(e Entry, id string) string {
	k := strings.LastIndexByte(id, '_')
	st := id[:k]
	i, _ := strconv.Atoi(id[k+1:])
	spec, ok := e.States[st]
	if !ok {
		st, spec = e.Rest, e.States[e.Rest]
	}
	if spec.Frames > 0 {
		i %= spec.Frames
	}
	return fmt.Sprintf("%s_%d", st, i)
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
