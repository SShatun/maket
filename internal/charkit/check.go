package charkit

import (
	"fmt"
	"image/color"
	"path/filepath"
	"sort"
)

type Rules struct {
	Palette map[color.NRGBA]bool
	Size    [2]int
	States  []string
	Anchor  [2]int
	Top     [2]int
}

func (c *Catalog) Rules() (Rules, error) {
	r := Rules{Palette: map[color.NRGBA]bool{}}
	var common map[string]bool
	first := true
	for _, name := range c.Names() {
		e := c.Entries[name]
		if e.Origin != "" && e.Origin != "designer" {
			continue
		}
		if first {
			r.Size, r.Anchor, r.Top = e.Size, e.Anchor, e.Top
			first = false
		}
		if e.Size != r.Size {
			r.Size = [2]int{}
		}
		if e.Anchor != r.Anchor || e.Top != r.Top {
			r.Anchor, r.Top = [2]int{}, [2]int{}
		}
		have := map[string]bool{}
		for st, spec := range e.States {
			if common == nil || common[st] {
				have[st] = true
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
		common = have
	}
	if first {
		return r, fmt.Errorf("%s: no designer characters to take rules from", c.Dir)
	}
	for st := range common {
		r.States = append(r.States, st)
	}
	sort.Strings(r.States)
	return r, nil
}

func Check(e Entry, frames string, r Rules) []string {
	var bad []string
	add := func(f string, a ...any) { bad = append(bad, fmt.Sprintf(f, a...)) }
	if r.Size != [2]int{} && e.Size != r.Size {
		add("size %v, designer characters have %v", e.Size, r.Size)
	}
	if r.Anchor != [2]int{} && (e.Anchor != r.Anchor || e.Top != r.Top) {
		add("anchor %v top %v, designer characters have %v and %v", e.Anchor, e.Top, r.Anchor, r.Top)
	}
	for _, st := range r.States {
		if _, ok := e.States[st]; !ok {
			add("no state %s, every designer character has it", st)
		}
	}
	if _, ok := e.States[e.Rest]; !ok {
		add("rest %q is not a state", e.Rest)
	}
	for _, st := range e.StateNames() {
		for i := 0; i < e.States[st].Frames; i++ {
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
			colours := map[color.NRGBA]bool{}
			uneven, half, filled := 0, 0, 0
			for cy := b.Min.Y; cy < b.Max.Y; cy += cell {
				for cx := b.Min.X; cx < b.Max.X; cx += cell {
					want := opaque(im.At(cx, cy))
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
					if want.A > 0 {
						filled++
						if !r.Palette[want] {
							colours[want] = true
						}
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
