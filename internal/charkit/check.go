package charkit

import (
	"fmt"
	"image/color"
	"math"
	"path/filepath"
	"regexp"
	"sort"
)

var stateName = regexp.MustCompile(`^[a-z0-9_-]+$`)

type Rules struct {
	Palette map[color.NRGBA]bool
	Size    [2]int
	Pixel   int
	States  map[string]State
	Anchor  [2]int
	Top     [2]int
}

func (c *Catalog) Rules() (Rules, error) {
	r := Rules{Palette: map[color.NRGBA]bool{}}
	first := true
	for _, name := range c.Names() {
		e := c.Entries[name]
		if e.Origin != "" && e.Origin != "designer" {
			continue
		}
		if first {
			r.Size, r.Pixel, r.Anchor, r.Top = e.Size, e.Pixel, e.Anchor, e.Top
			first = false
		}
		if e.Pixel != r.Pixel {
			return r, fmt.Errorf("%s: designer characters use different pixel sizes", c.Dir)
		}
		if e.Size != r.Size {
			r.Size = [2]int{}
		}
		for i := range r.Anchor {
			if e.Anchor[i] != r.Anchor[i] {
				r.Anchor[i] = -1
			}
			if e.Top[i] != r.Top[i] {
				r.Top[i] = -1
			}
		}
		have := map[string]State{}
		for st, spec := range e.States {
			if previous, ok := r.States[st]; r.States == nil || ok {
				common := spec
				if r.States != nil && previous.Frames != spec.Frames {
					common.Frames = 0
				}
				have[st] = common
			}
			if r.Pixel == 0 {
				continue
			}
			for i := 0; i < spec.Frames; i++ {
				im, err := ReadPNG(filepath.Join(c.Dir, name, fmt.Sprintf("%s_%d.png", st, i)))
				if err != nil {
					return r, err
				}
				b := im.Bounds()
				for y := b.Min.Y; y < b.Max.Y; y++ {
					for x := b.Min.X; x < b.Max.X; x++ {
						if p := opaque(im.At(x, y)); p.A > 0 {
							r.Palette[p] = true
						}
					}
				}
			}
		}
		r.States = have
	}
	if first {
		return r, fmt.Errorf("%s: no designer characters to take rules from", c.Dir)
	}
	return r, nil
}

func Check(e Entry, frames string, r Rules) []string {
	var bad []string
	add := func(f string, a ...any) { bad = append(bad, fmt.Sprintf(f, a...)) }
	if e.Size[0] <= 0 || e.Size[1] <= 0 {
		add("size %v must be positive", e.Size)
	}
	if r.Size != [2]int{} && e.Size != r.Size {
		add("size %v, designer characters have %v", e.Size, r.Size)
	}
	if e.Pixel != r.Pixel {
		add("pixel %d, designer characters use %d (0 means smooth drawing)", e.Pixel, r.Pixel)
	}
	if r.Pixel > 0 && (e.Size[0]%r.Pixel != 0 || e.Size[1]%r.Pixel != 0) {
		add("size %v must be a multiple of pixel %d", e.Size, r.Pixel)
	}
	for i := range e.Anchor {
		if e.Anchor[i] < 0 || e.Anchor[i] > e.Size[i] || e.Top[i] < 0 || e.Top[i] > e.Size[i] {
			add("anchor %v and top %v must be inside size %v", e.Anchor, e.Top, e.Size)
			break
		}
	}
	for i := range e.Anchor {
		if r.Anchor[i] >= 0 && e.Anchor[i] != r.Anchor[i] {
			add("anchor[%d] is %d, designer characters have %d", i, e.Anchor[i], r.Anchor[i])
		}
		if r.Top[i] >= 0 && e.Top[i] != r.Top[i] {
			add("top[%d] is %d, designer characters have %d", i, e.Top[i], r.Top[i])
		}
	}
	var common []string
	for st := range r.States {
		common = append(common, st)
	}
	sort.Strings(common)
	for _, st := range common {
		spec, ok := e.States[st]
		if !ok {
			add("no state %s, every designer character has it", st)
		} else if n := r.States[st].Frames; n > 0 && spec.Frames != n {
			add("state %s: %d frames, every designer character has %d", st, spec.Frames, n)
		}
	}
	if _, ok := e.States[e.Rest]; !ok {
		add("rest %q is not a state", e.Rest)
	}
	for _, st := range e.StateNames() {
		spec := e.States[st]
		if !stateName.MatchString(st) {
			add("state %q must use lowercase letters, digits, - and _", st)
			continue
		}
		if spec.Frames <= 0 {
			add("state %s: frames must be positive", st)
		}
		if spec.FPS <= 0 || math.IsNaN(spec.FPS) || math.IsInf(spec.FPS, 0) {
			add("state %s: fps must be positive and finite", st)
		}
		for i := 0; i < spec.Frames; i++ {
			id := fmt.Sprintf("%s_%d", st, i)
			im, err := ReadPNG(filepath.Join(frames, id+".png"))
			if err != nil {
				add("%s: %v", id, err)
				continue
			}
			b := im.Bounds()
			if b.Dx() != e.Size[0] || b.Dy() != e.Size[1] {
				add("%s: %dx%d, entry says %dx%d", id, b.Dx(), b.Dy(), e.Size[0], e.Size[1])
				continue
			}
			cell := max(1, r.Pixel)
			colours := map[color.NRGBA]bool{}
			uneven, half, filled := 0, 0, 0
			for cy := b.Min.Y; cy < b.Max.Y; cy += cell {
				for cx := b.Min.X; cx < b.Max.X; cx += cell {
					want := opaque(im.At(cx, cy))
					if want.A > 0 {
						filled++
					}
					if r.Pixel == 0 {
						continue
					}
					for y := cy; y < cy+cell && y < b.Max.Y; y++ {
						for x := cx; x < cx+cell && x < b.Max.X; x++ {
							p := color.NRGBAModel.Convert(im.At(x, y)).(color.NRGBA)
							if p.A != 0 && p.A != 255 {
								half++
							}
							if opaque(p) != want {
								uneven++
							}
						}
					}
					if want.A > 0 && !r.Palette[want] {
						colours[want] = true
					}
				}
			}
			if filled == 0 {
				add("%s: empty frame", id)
			}
			if uneven > 0 {
				add("%s: %d pixels break the %dx%d cell grid", id, uneven, cell, cell)
			}
			if half > 0 {
				add("%s: %d half-transparent pixels", id, half)
			}
			for _, p := range sortedColours(colours) {
				add("%s: colour %d,%d,%d is not in the designer palette", id, p.R, p.G, p.B)
			}
		}
	}
	return bad
}

func sortedColours(m map[color.NRGBA]bool) []color.NRGBA {
	out := make([]color.NRGBA, 0, len(m))
	for p := range m {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.R != b.R {
			return a.R < b.R
		}
		if a.G != b.G {
			return a.G < b.G
		}
		return a.B < b.B
	})
	return out
}
