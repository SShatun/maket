package moe

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"sort"
)

type litKey struct {
	source *image.RGBA
	light  light
	flip   bool
}

func (s *Style) prepareActor(sh *shot, a *actor, cache map[litKey]visual) error {
	sort.SliceStable(a.states, func(i, j int) bool { return a.states[i].T < a.states[j].T })
	sort.SliceStable(a.roles, func(i, j int) bool { return a.roles[i].at < a.roles[j].at })
	roles, states := map[string]bool{}, map[string]bool{}
	spec := s.sprites.Specs[a.kind]
	for _, r := range a.roles {
		if _, ok := s.roles[r.value]; !ok {
			return fmt.Errorf("unknown role %q", r.value)
		}
		roles[r.value] = true
	}
	for i, st := range a.states {
		v, ok := spec.States[st.V]
		if !ok || v.Frames < 1 || v.FPS <= 0 {
			return fmt.Errorf("%s has no valid state %q", a.kind, st.V)
		}
		states[st.V] = true
		if st.V == "catch" || st.V == "flinch" {
			a.states[i].Hold = false
		}
	}
	if err := s.sprites.Prepare(a.kind, roles, states); err != nil {
		return err
	}
	times := []float64{sh.at}
	for _, bg := range sh.backgrounds {
		if bg.at < sh.until {
			times = append(times, bg.at)
		}
	}
	for _, r := range a.roles {
		times = append(times, r.at)
	}
	for _, w := range a.walks {
		times = append(times, w.at)
	}
	for i, st := range a.states {
		end := sh.until
		if i+1 < len(a.states) {
			end = a.states[i+1].T
		}
		v := spec.States[st.V]
		times = append(times, st.T)
		for j := 1; ; j++ {
			t := st.T + float64(j)/v.FPS
			if t >= end {
				break
			}
			if !v.Loop && j > v.Frames {
				break
			}
			times = append(times, t)
		}
		if st.V == "catch" || st.V == "flinch" {
			rest := spec.States[spec.Rest]
			for j := int(math.Ceil(float64(v.Frames) / v.FPS * rest.FPS)); ; j++ {
				t := st.T + float64(j)/rest.FPS
				if t >= end {
					break
				}
				times = append(times, t)
			}
		}
		s.stateSound(a.kind, st.V, st.T, end)
	}
	sort.Float64s(times)
	last := -1.0
	for _, t := range times {
		if t-last < 1e-8 {
			continue
		}
		last = t
		bg := backgroundAt(sh, t+1e-8)
		src := s.sprites.Frame(a.kind, a.states, t+1e-8, roleAt(a, t+1e-8))
		if src == nil {
			return fmt.Errorf("missing sprite at %.3f", t)
		}
		key := litKey{source: src, light: bg.light, flip: a.facing(t + 1e-8)}
		v, ok := cache[key]
		if !ok {
			v = illuminate(src, bg.light, key.flip)
			cache[key] = v
		}
		v.at = t
		if len(a.visuals) == 0 || a.visuals[len(a.visuals)-1].body != v.body {
			a.visuals = append(a.visuals, v)
		}
	}
	return nil
}

func rgba(src image.Image) *image.RGBA {
	dst := image.NewRGBA(src.Bounds())
	draw.Draw(dst, dst.Bounds(), src, src.Bounds().Min, draw.Src)
	return dst
}

func mixImage(a, b *image.RGBA, k float64) *image.RGBA {
	if k <= 0 {
		return a
	}
	if k >= 1 {
		return b
	}
	out := image.NewRGBA(a.Bounds())
	u := int(math.Round(k * 256))
	for i := range out.Pix {
		out.Pix[i] = uint8((int(a.Pix[i])*(256-u) + int(b.Pix[i])*u + 128) >> 8)
	}
	return out
}

func mixLight(a, b light, k float64) light {
	v := light{AmbK: lerp(a.AmbK, b.AmbK, k)}
	for i := range v.Amb {
		v.Amb[i] = lerp(a.Amb[i], b.Amb[i], k)
	}
	for i := range v.Rim {
		v.Rim[i] = lerp(a.Rim[i], b.Rim[i], k)
		v.Glow[i] = lerp(a.Glow[i], b.Glow[i], k)
	}
	for i := range v.Dir {
		v.Dir[i] = lerp(a.Dir[i], b.Dir[i], k)
	}
	return v
}

func illuminate(src *image.RGBA, l light, flip bool) visual {
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	base := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			sx := x
			if flip {
				sx = w - 1 - x
			}
			copy(base.Pix[y*base.Stride+x*4:y*base.Stride+x*4+4], src.Pix[y*src.Stride+sx*4:y*src.Stride+sx*4+4])
		}
	}
	mask := make([]float32, w*h)
	dx, dy := int(math.Round(l.Dir[0]*14)), int(math.Round(l.Dir[1]*14))
	if l.Rim[3] > 0 {
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				a := int(base.Pix[y*base.Stride+x*4+3])
				xx, yy := x+dx, y+dy
				if xx >= 0 && xx < w && yy >= 0 && yy < h {
					a -= int(base.Pix[yy*base.Stride+xx*4+3])
				}
				mask[y*w+x] = float32(max(0, a)) / 255
			}
		}
		mask = blur(mask, w, h, 1.2)
	}
	body := image.NewRGBA(base.Bounds())
	bounds := image.Rectangle{Min: image.Pt(w, h)}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := y*body.Stride + x*4
			a := float64(base.Pix[i+3]) / 255
			if a == 0 {
				continue
			}
			bounds.Min.X = min(bounds.Min.X, x)
			bounds.Max.X = max(bounds.Max.X, x+1)
			bounds.Min.Y = min(bounds.Min.Y, y)
			bounds.Max.Y = max(bounds.Max.Y, y+1)
			rim := math.Min(a, float64(mask[y*w+x])) * 0.85 * l.Rim[3] / 255
			for c := 0; c < 3; c++ {
				v := float64(base.Pix[i+c]) * (1 - l.AmbK + l.AmbK*l.Amb[c]/255)
				v = v*(1-rim/a) + l.Rim[c]*rim
				body.Pix[i+c] = uint8(math.Min(a*255, math.Max(0, v)) + 0.5)
			}
			body.Pix[i+3] = base.Pix[i+3]
		}
	}
	if bounds.Empty() {
		bounds = image.Rect(0, 0, 1, 1)
	}
	body = rgba(body.SubImage(bounds))
	v := visual{body: body}
	if l.Glow[3] > 0 {
		b := image.Rect(-20, -20, (w+3)/4+20, (h+3)/4+20)
		gw, gh := b.Dx(), b.Dy()
		alpha := make([]float32, gw*gh)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				alpha[(y/4-b.Min.Y)*gw+x/4-b.Min.X] += float32(base.Pix[y*base.Stride+x*4+3]) / (16 * 255)
			}
		}
		alpha = blur(alpha, gw, gh, 26.0/4)
		v.glow = image.NewRGBA(b)
		for i, a := range alpha {
			for c := 0; c < 3; c++ {
				v.glow.Pix[i*4+c] = uint8(math.Max(0, math.Min(255, float64(a)*l.Glow[c]*0.9*l.Glow[3]/255)) + 0.5)
			}
			v.glow.Pix[i*4+3] = 255
		}
	}
	return v
}

func blur(src []float32, w, h int, sigma float64) []float32 {
	ideal := math.Sqrt(4*sigma*sigma + 1)
	lo := int(math.Floor(ideal))
	if lo%2 == 0 {
		lo--
	}
	lo = max(1, lo)
	m := int(math.Round((12*sigma*sigma - 3*float64(lo*lo) - 12*float64(lo) - 9) / (-4*float64(lo) - 4)))
	tmp, dst := make([]float32, len(src)), make([]float32, len(src))
	for pass := 0; pass < 3; pass++ {
		r := (lo + 2 - 1) / 2
		if pass < m {
			r = (lo - 1) / 2
		}
		if r == 0 {
			continue
		}
		div := float32(2*r + 1)
		for y := 0; y < h; y++ {
			sum := float32(0)
			for x := 0; x <= r && x < w; x++ {
				sum += src[y*w+x]
			}
			for x := 0; x < w; x++ {
				tmp[y*w+x] = sum / div
				if x-r >= 0 {
					sum -= src[y*w+x-r]
				}
				if x+r+1 < w {
					sum += src[y*w+x+r+1]
				}
			}
		}
		for x := 0; x < w; x++ {
			sum := float32(0)
			for y := 0; y <= r && y < h; y++ {
				sum += tmp[y*w+x]
			}
			for y := 0; y < h; y++ {
				dst[y*w+x] = sum / div
				if y-r >= 0 {
					sum -= tmp[(y-r)*w+x]
				}
				if y+r+1 < h {
					sum += tmp[(y+r+1)*w+x]
				}
			}
		}
		src, dst = dst, src
	}
	return src
}

func makeShadow() *image.RGBA {
	b := image.Rect(-150, -44, 150, 44)
	mask := make([]float32, b.Dx()*b.Dy())
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if float64(x*x)/(130*130)+float64(y*y)/(24*24) <= 1 {
				mask[(y-b.Min.Y)*b.Dx()+x-b.Min.X] = 0.35
			}
		}
	}
	mask = blur(mask, b.Dx(), b.Dy(), 5)
	im := image.NewRGBA(b)
	for i, a := range mask {
		im.Pix[i*4+3] = uint8(math.Max(0, float64(a)) * 255)
	}
	return im
}

func solid(c color.RGBA) *image.Uniform { return image.NewUniform(c) }
