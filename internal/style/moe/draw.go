package moe

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"sort"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/math/f64"

	"maket/internal/render"
)

func (s *Style) Draw(dst *image.RGBA, t float64) {
	if len(s.shots) == 0 {
		return
	}
	i := sort.Search(len(s.shots), func(i int) bool { return s.shots[i].at > t }) - 1
	sh := &s.shots[max(0, i)]
	bg := backgroundAt(sh, t)
	cam := s.cameraAt(sh, t)
	drawBackground(dst, bg.image, cam)
	for _, p := range sh.panels {
		if t >= p.at && t < p.until {
			drawPanel(dst, p, bg.bright, cam, t)
		}
	}
	for _, title := range sh.titles {
		if t < title.at || t >= title.until {
			continue
		}
		k := clamp((t - title.at - 0.4) / 0.8)
		addImage(dst, title.glow, cam, k)
		transformAlpha(dst, title.image, cam, k)
	}
	type placed struct {
		visual visual
		matrix f64.Aff3
		shadow f64.Aff3
	}
	placedActors := make([]placed, 0, len(sh.actors))
	for _, a := range sh.actors {
		j := sort.Search(len(a.visuals), func(j int) bool { return a.visuals[j].at > t }) - 1
		v := a.visuals[max(0, j)]
		x, y := a.x, a.y
		bob, breath, hop, squash := 0.0, 1.0, 0.0, 1.0
		dt := t - sh.at
		if a.motion != "none" {
			bob = 3 * math.Sin(math.Pi*dt)
			breath = 1 + 0.01*math.Sin(math.Pi*dt)
		}
		if a.motion == "jump" {
			phase := math.Mod(math.Max(0, dt)*2, 1)
			hop = 45 * math.Sin(math.Pi*phase)
			squash = 1 - 0.06*math.Exp(-math.Pow(math.Min(phase, 1-phase)/0.08, 2))
		}
		sx, sy := a.size, a.size*breath*squash
		spec := s.sprites.Specs[a.kind]
		ax := float64(spec.Anchor[0])
		if a.flip {
			ax = float64(spec.Size[0]) - ax
		}
		m := compose(cam, f64.Aff3{sx, 0, x - ax*sx, 0, sy, y + bob - hop - float64(spec.Anchor[1])*sy})
		shadowScale := a.size * (1 - 0.12*hop/45)
		shadow := compose(cam, f64.Aff3{shadowScale, 0, x, 0, shadowScale, y})
		placedActors = append(placedActors, placed{v, m, shadow})
		if v.glow != nil {
			addImage(dst, v.glow, compose(m, f64.Aff3{4, 0, 0, 0, 4, 0}), 1)
		}
	}
	for _, p := range placedActors {
		xdraw.ApproxBiLinear.Transform(dst, p.shadow, s.shadow, s.shadow.Bounds(), draw.Over, nil)
	}
	for _, p := range placedActors {
		xdraw.ApproxBiLinear.Transform(dst, p.matrix, p.visual.body, p.visual.body.Bounds(), draw.Over, nil)
	}
	s.caption(dst, t)
	for _, shot := range s.shots {
		for _, c := range shot.cuts {
			k := clamp(1 - math.Abs(t-c.at)/(c.dur/2))
			if k > 0 {
				v := uint8(0)
				if c.white {
					v = 255
				}
				overlay(dst, color.RGBA{v, v, v, 255}, k)
			}
		}
	}
	for _, shock := range sh.shocks {
		dt := t - shock.at
		if dt >= 0 && dt < 0.12 {
			overlay(dst, color.RGBA{255, 255, 255, 255}, 1-dt/0.12)
		}
	}
}

func (s *Style) cameraAt(sh *shot, t float64) f64.Aff3 {
	z := lerp(1, 1.05, clamp((t-sh.at)/(sh.until-sh.at)))
	dx, dy := 0.0, 0.0
	for i, shock := range sh.shocks {
		dt := t - shock.at
		if dt < 0 || dt >= 0.7 {
			continue
		}
		kick := 1.0
		if dt < 0.25 {
			kick = lerp(shock.zoom, 1.02, clamp(dt/0.25))
		} else {
			kick = lerp(1.02, 1, clamp((dt-0.25)/0.45))
		}
		z *= kick
		amp := shock.shake * (1 - dt/0.7)
		seed := uint64(int64(s.sc.Seed)) + uint64(i)*0x9e3779b97f4a7c15
		frame := uint64(int64(t * render.FPS))
		dx += noise(seed^frame*2) * amp
		dy += noise(seed^(frame*2+1)) * amp
	}
	x := float64(render.W)/2*(1-z) + dx
	y := float64(render.H)*0.65*(1-z) + dy
	x = math.Max(float64(render.W)*(1-z), math.Min(0, x))
	y = math.Max(float64(render.H)*(1-z), math.Min(0, y))
	return f64.Aff3{z, 0, x, 0, z, y}
}

func drawPanel(dst *image.RGBA, p *panel, bright float64, cam f64.Aff3, t float64) {
	k := clamp((t - p.entrance) / 0.3)
	if t-p.entrance < 0.45 && int((t-p.entrance)*render.FPS)%4 == 0 {
		k *= 0.35
	}
	if k == 0 {
		return
	}
	im := image.NewRGBA(p.shell.Bounds())
	if bright > 0 {
		draw.DrawMask(im, im.Bounds(), p.dark, image.Point{}, image.NewUniform(color.Alpha{uint8(255 * bright)}), image.Point{}, draw.Src)
	}
	draw.Draw(im, im.Bounds(), p.shell, image.Point{}, draw.Over)
	row := 0
	for i, l := range p.rows {
		if t < l.at {
			continue
		}
		a := uint8(255)
		if i == p.blink {
			a = uint8(255 * (0.35 + 0.65*(0.5+0.5*math.Cos(2*math.Pi*2.4*(t-p.at)))))
		}
		r := l.image.Bounds().Add(image.Pt(28+22, 28+64+40*row))
		draw.DrawMask(im, r, l.image, image.Point{}, image.NewUniform(color.Alpha{a}), image.Point{}, draw.Over)
		row++
	}
	m := compose(cam, f64.Aff3{1, 0, p.rect[0] - 28, 0, 1, p.rect[1] - 28})
	addImage(dst, p.glow, m, k)
	transformAlpha(dst, im, m, k)
}

func transformAlpha(dst, src *image.RGBA, m f64.Aff3, k float64) {
	if k <= 0 {
		return
	}
	if k >= 1 {
		xdraw.ApproxBiLinear.Transform(dst, m, src, src.Bounds(), draw.Over, nil)
		return
	}
	faded := image.NewRGBA(src.Bounds())
	gain := uint32(clamp(k) * 256)
	for i, v := range src.Pix {
		faded.Pix[i] = uint8(uint32(v) * gain >> 8)
	}
	xdraw.ApproxBiLinear.Transform(dst, m, faded, faded.Bounds(), draw.Over, nil)
}

func addImage(dst, src *image.RGBA, m f64.Aff3, k float64) {
	if k <= 0 {
		return
	}
	b := src.Bounds()
	x0, x1 := float64(b.Min.X)*m[0]+m[2], float64(b.Max.X)*m[0]+m[2]
	y0, y1 := float64(b.Min.Y)*m[4]+m[5], float64(b.Max.Y)*m[4]+m[5]
	full := image.Rect(int(math.Floor(x0)), int(math.Floor(y0)), int(math.Ceil(x1)), int(math.Ceil(y1)))
	r := full.Intersect(dst.Bounds())
	if r.Empty() {
		return
	}
	var left, right [render.W]int
	var weight [render.W]uint32
	sxScale, syScale := float64(b.Dx())/float64(full.Dx()), float64(b.Dy())/float64(full.Dy())
	for x := 0; x < r.Dx(); x++ {
		sx := math.Max(0, math.Min(float64(b.Dx()-1), (float64(x+r.Min.X-full.Min.X)+0.5)*sxScale-0.5))
		base := int(sx)
		left[x], right[x] = base*4, min(base+1, b.Dx()-1)*4
		weight[x] = uint32(math.Round((sx - float64(base)) * 256))
	}
	gain := uint32(clamp(k) * 256)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		sy := math.Max(0, math.Min(float64(b.Dy()-1), (float64(y-full.Min.Y)+0.5)*syScale-0.5))
		base := int(sy)
		top, bottom := base*src.Stride, min(base+1, b.Dy()-1)*src.Stride
		wy := uint32(math.Round((sy - float64(base)) * 256))
		di := dst.PixOffset(r.Min.X, y)
		for x := 0; x < r.Dx(); x++ {
			wx := weight[x]
			a, b, c, d := top+left[x], top+right[x], bottom+left[x], bottom+right[x]
			for channel := 0; channel < 3; channel++ {
				upper := uint32(src.Pix[a+channel])*(256-wx) + uint32(src.Pix[b+channel])*wx
				lower := uint32(src.Pix[c+channel])*(256-wx) + uint32(src.Pix[d+channel])*wx
				v := (upper*(256-wy) + lower*wy + 32768) >> 16
				dst.Pix[di+channel] = uint8(min(255, uint32(dst.Pix[di+channel])+(v*gain>>8)))
			}
			di += 4
		}
	}
}

func compose(a, b f64.Aff3) f64.Aff3 {
	return f64.Aff3{a[0] * b[0], 0, a[0]*b[2] + a[2], 0, a[4] * b[4], a[4]*b[5] + a[5]}
}

func overlay(dst *image.RGBA, c color.RGBA, k float64) {
	draw.DrawMask(dst, dst.Bounds(), solid(c), image.Point{}, image.NewUniform(color.Alpha{uint8(clamp(k) * 255)}), image.Point{}, draw.Over)
}

func noise(x uint64) float64 {
	x += 0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	x ^= x >> 31
	return float64(x>>11)/float64(uint64(1)<<53)*2 - 1
}

func (s *Style) caption(dst *image.RGBA, t float64) {
	i := sort.Search(len(s.tl.Phrases), func(i int) bool { return s.tl.Phrases[i].Start > t }) - 1
	if i < 0 {
		return
	}
	k := clamp((t - s.tl.Phrases[i].Start) / 0.2)
	if k >= 1 {
		render.Caption(dst, s.sc, s.tl, s.theme, t)
		return
	}
	layer := image.NewRGBA(dst.Bounds())
	render.Caption(layer, s.sc, s.tl, s.theme, t)
	draw.DrawMask(dst, dst.Bounds(), layer, image.Point{}, image.NewUniform(color.Alpha{uint8(255 * k)}), image.Point{}, draw.Over)
}

func drawBackground(dst, src *image.RGBA, m f64.Aff3) {
	if m == (f64.Aff3{1, 0, 0, 0, 1, 0}) {
		draw.Draw(dst, dst.Bounds(), src, image.Point{}, draw.Src)
		return
	}
	var left, right [render.W]int
	var weight [render.W]uint32
	for x := 0; x < render.W; x++ {
		sx := math.Max(0, math.Min(render.W-1, (float64(x)+0.5-m[2])/m[0]-0.5))
		base := int(sx)
		left[x], right[x] = base*4, min(base+1, render.W-1)*4
		weight[x] = uint32(math.Round((sx - float64(base)) * 256))
	}
	for y := 0; y < render.H; y++ {
		sy := math.Max(0, math.Min(render.H-1, (float64(y)+0.5-m[5])/m[4]-0.5))
		base := int(sy)
		top, bottom := base*src.Stride, min(base+1, render.H-1)*src.Stride
		wy := uint32(math.Round((sy - float64(base)) * 256))
		for x := 0; x < render.W; x++ {
			wx := weight[x]
			a, b, c, d := top+left[x], top+right[x], bottom+left[x], bottom+right[x]
			i := y*dst.Stride + x*4
			for channel := 0; channel < 3; channel++ {
				upper := uint32(src.Pix[a+channel])*(256-wx) + uint32(src.Pix[b+channel])*wx
				lower := uint32(src.Pix[c+channel])*(256-wx) + uint32(src.Pix[d+channel])*wx
				dst.Pix[i+channel] = uint8((upper*(256-wy) + lower*wy + 32768) >> 16)
			}
			dst.Pix[i+3] = 255
		}
	}
}
