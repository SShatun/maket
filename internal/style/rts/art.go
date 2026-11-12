package rts

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"path/filepath"

	xdraw "golang.org/x/image/draw"

	"maket/internal/charkit"
	"maket/internal/render"
)

var (
	black      = color.RGBA{0, 0, 0, 255}
	white      = color.RGBA{255, 255, 255, 255}
	frameInk   = []color.RGBA{{22, 18, 14, 255}, {170, 164, 150, 255}, {118, 114, 108, 255}, {118, 114, 108, 255}, {50, 46, 42, 255}}
	bevelDark  = color.RGBA{50, 46, 42, 255}
	bevelLight = color.RGBA{170, 164, 150, 255}
	well       = color.RGBA{40, 36, 32, 255}
	backing    = color.RGBA{20, 16, 12, 255}
	hpOn       = color.RGBA{60, 230, 60, 255}
	hpOff      = color.RGBA{40, 0, 0, 255}
	toneInk    = map[string]color.RGBA{"ok": {120, 255, 120, 255}, "warn": {255, 230, 120, 255}, "bad": {255, 80, 70, 255}}

	commands   = []string{"move", "stop", "wait", "retry", "cancel", "build", "attack", "mine"}
	resKeys    = []string{"cpu", "ram", "conn", "gold", "rps"}
	markFrames = map[string]int{"bang": 2, "question": 1, "wait": 1, "dots": 3, "zzz": 3}

	minimapBox = image.Rect(16, 1456, 288, 1728)
	minimapIn  = image.Rect(28, 1468, 276, 1716)
	infoBox    = image.Rect(312, 1456, 1064, 1648)
)

type art struct {
	grass    [4]*image.RGBA
	dirt     [2]*image.RGBA
	tree     *image.RGBA
	rock     *image.RGBA
	stone    *image.RGBA
	button   *image.RGBA
	pressed  *image.RGBA
	icons    map[string]*image.RGBA
	lit      map[string]*image.RGBA
	res      map[string]*image.RGBA
	marks    map[string][]*image.RGBA
	small    map[string][]*image.RGBA
	glyphs   map[string]*image.RGBA
	chip     *image.RGBA
	frame    *image.RGBA
	sel      *image.RGBA
	selInk   color.RGBA
	chrome   *image.RGBA
	fog      *image.RGBA
	fogRuns  [][3]int
	wallTile [14][3]color.RGBA
}

func loadArt(dir string) (*art, error) {
	a := &art{icons: map[string]*image.RGBA{}, lit: map[string]*image.RGBA{}, res: map[string]*image.RGBA{}, marks: map[string][]*image.RGBA{},
		small: map[string][]*image.RGBA{}, glyphs: map[string]*image.RGBA{}}
	var err error
	get := func(name string, k int) *image.RGBA {
		if err != nil {
			return nil
		}
		var src image.Image
		src, err = charkit.ReadPNG(filepath.Join(dir, name))
		if err != nil {
			return nil
		}
		return shrink(src, k)
	}
	for i := range a.grass {
		a.grass[i] = get(fmt.Sprintf("tiles/grass_%d.png", i), 12)
	}
	for i := range a.dirt {
		a.dirt[i] = get(fmt.Sprintf("tiles/dirt_%d.png", i), 12)
	}
	a.tree = get("tiles/tree.png", 12)
	a.rock = get("tiles/rock.png", 12)
	a.stone = get("ui/stone.png", 3)
	a.chip = get("ui/stone.png", 4)
	a.button = get("ui/button.png", 3)
	a.pressed = get("ui/button_pressed.png", 3)
	a.frame = get("ui/portrait_frame.png", 3)
	a.sel = get("ui/select.png", 2)
	for _, c := range commands {
		a.icons[c] = get("ui/cmd-"+c+".png", 3)
		if a.icons[c] != nil {
			a.lit[c] = lighten(a.icons[c])
		}
		a.glyphs["cmd-"+c] = get("ui/cmd-"+c+".png", 6)
	}
	for _, r := range resKeys {
		a.res[r] = get("ui/res-"+r+".png", 3)
		a.glyphs["res-"+r] = a.res[r]
	}
	for m, n := range markFrames {
		for i := 0; i < n; i++ {
			a.marks[m] = append(a.marks[m], get(fmt.Sprintf("ui/mark-%s_%d.png", m, i), 2))
		}
		if err != nil {
			return nil, err
		}
		a.small[m] = cropAll(a.marks[m])
		if a.small[m][0].Bounds().Dy() > 48 {
			var frames []*image.RGBA
			for i := 0; i < n; i++ {
				frames = append(frames, get(fmt.Sprintf("ui/mark-%s_%d.png", m, i), 3))
			}
			if err != nil {
				return nil, err
			}
			a.small[m] = cropAll(frames)
		}
		a.glyphs["mark-"+m] = a.small[m][0]
	}
	lock := get("lock/idle_0.png", 12)
	if err != nil {
		return nil, err
	}
	for i := 0; i < len(a.sel.Pix); i += 4 {
		if a.sel.Pix[i+3] == 255 {
			a.selInk = color.RGBA{a.sel.Pix[i], a.sel.Pix[i+1], a.sel.Pix[i+2], 255}
			break
		}
	}
	for r := 0; r < 14; r++ {
		for c := 0; c < 3; c++ {
			a.wallTile[r][c] = lock.RGBAAt(c, 30+r)
		}
	}
	a.chrome = chrome(a)
	a.fog, a.fogRuns = fog()
	return a, nil
}

func shrink(src image.Image, k int) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx()/k, b.Dy()/k))
	xdraw.NearestNeighbor.Scale(dst, dst.Bounds(), src, b, xdraw.Src, nil)
	return dst
}

func cropAll(frames []*image.RGBA) []*image.RGBA {
	r := image.Rectangle{}
	for _, f := range frames {
		r = r.Union(opaque(f))
	}
	out := make([]*image.RGBA, len(frames))
	for i, f := range frames {
		out[i] = image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
		draw.Draw(out[i], out[i].Bounds(), f, r.Min, draw.Src)
	}
	return out
}

func lighten(src *image.RGBA) *image.RGBA {
	out := image.NewRGBA(src.Bounds())
	copy(out.Pix, src.Pix)
	for i := 0; i < len(out.Pix); i += 4 {
		if out.Pix[i+3] == 0 {
			continue
		}
		for c := 0; c < 3; c++ {
			v := int(out.Pix[i+c])
			out.Pix[i+c] = uint8(v + (int(out.Pix[i+3])-v)*45/100)
		}
	}
	return out
}

func mirror(src *image.RGBA) *image.RGBA {
	b := src.Bounds()
	out := image.NewRGBA(b)
	w := b.Dx()
	for y := 0; y < b.Dy(); y++ {
		row := src.Pix[y*src.Stride : y*src.Stride+w*4]
		dst := out.Pix[y*out.Stride : y*out.Stride+w*4]
		for x := 0; x < w; x++ {
			copy(dst[(w-1-x)*4:(w-x)*4], row[x*4:x*4+4])
		}
	}
	return out
}

func opaque(im *image.RGBA) image.Rectangle {
	if im == nil {
		return image.Rectangle{}
	}
	b := im.Bounds()
	x0, y0, x1, y1 := b.Max.X, b.Max.Y, b.Min.X, b.Min.Y
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if im.Pix[im.PixOffset(x, y)+3] > 0 {
				x0, y0, x1, y1 = min(x0, x), min(y0, y), max(x1, x+1), max(y1, y+1)
			}
		}
	}
	if x1 <= x0 {
		return b
	}
	return image.Rect(x0, y0, x1, y1)
}

func fill(dst *image.RGBA, r image.Rectangle, c color.RGBA) {
	draw.Draw(dst, r, image.NewUniform(c), image.Point{}, draw.Src)
}

func tile(dst *image.RGBA, r image.Rectangle, im *image.RGBA) {
	w, h := im.Bounds().Dx(), im.Bounds().Dy()
	for y := r.Min.Y; y < r.Max.Y; y += h {
		for x := r.Min.X; x < r.Max.X; x += w {
			draw.Draw(dst, image.Rect(x, y, x+w, y+h).Intersect(r), im, image.Point{}, draw.Over)
		}
	}
}

func inset(dst *image.RGBA, r image.Rectangle) {
	fill(dst, r, frameInk[0])
	in := r.Inset(4)
	fill(dst, in, bevelLight)
	fill(dst, image.Rect(in.Min.X, in.Min.Y, in.Max.X-4, in.Max.Y-4), bevelDark)
	fill(dst, in.Inset(4), well)
}

func outline(dst *image.RGBA, r image.Rectangle, w int, c color.RGBA) {
	fill(dst, image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+w), c)
	fill(dst, image.Rect(r.Min.X, r.Max.Y-w, r.Max.X, r.Max.Y), c)
	fill(dst, image.Rect(r.Min.X, r.Min.Y, r.Min.X+w, r.Max.Y), c)
	fill(dst, image.Rect(r.Max.X-w, r.Min.Y, r.Max.X, r.Max.Y), c)
}

func chrome(a *art) *image.RGBA {
	c := image.NewRGBA(image.Rect(0, 0, render.W, render.H))
	fill(c, c.Bounds(), backing)
	tile(c, image.Rect(0, 0, render.W, mapTop), a.stone)
	fill(c, image.Rect(0, mapTop-1, render.W, mapTop), black)
	fill(c, image.Rect(0, mapBottom, render.W, panelTop), black)
	tile(c, image.Rect(0, panelTop, render.W, render.H), a.stone)
	inset(c, minimapBox)
	fill(c, minimapIn, black)
	inset(c, infoBox)
	return c
}

func fogged(x, y int) bool {
	e := min(x, y, mapW-1-x, mapH-1-y)
	switch {
	case e < 5:
		return true
	case e < 10:
		return (x+y)%2 == 0
	case e < 15:
		return x%2 == 0 && y%2 == 0
	}
	return false
}

func fog() (*image.RGBA, [][3]int) {
	f := image.NewRGBA(image.Rect(0, 0, mapW*px, mapH*px))
	for y := 0; y < mapH; y++ {
		for x := 0; x < mapW; x++ {
			if fogged(x, y) {
				fill(f, image.Rect(x*px, y*px, (x+1)*px, (y+1)*px), black)
			}
		}
	}
	for i, c := range frameInk {
		outline(f, f.Bounds().Inset(i*4), 4, c)
	}
	var runs [][3]int
	for y := 0; y < f.Bounds().Dy(); y++ {
		start := -1
		for x := 0; x <= f.Bounds().Dx(); x++ {
			on := x < f.Bounds().Dx() && f.Pix[f.PixOffset(x, y)+3] == 255
			if on && start < 0 {
				start = x
			}
			if !on && start >= 0 {
				runs = append(runs, [3]int{y, start, x})
				start = -1
			}
		}
	}
	return f, runs
}
