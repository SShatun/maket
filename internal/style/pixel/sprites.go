package pixel

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"

	xdraw "golang.org/x/image/draw"
)

const scale = 1.0

type stateSpec struct {
	Frames int     `json:"frames"`
	FPS    float64 `json:"fps"`
	Loop   bool    `json:"loop"`
}

type spriteSpec struct {
	Size   [2]int               `json:"size"`
	Anchor [2]int               `json:"anchor"`
	Top    [2]int               `json:"top"`
	Rest   string               `json:"rest"`
	Tint   *bool                `json:"tint"`
	States map[string]stateSpec `json:"states"`
}

type frameKey struct {
	sprite, state, role string
	i                   int
}

type sprites struct {
	dir    string
	specs  map[string]spriteSpec
	frames map[frameKey]*image.RGBA
}

func loadSprites(dir string) (*sprites, error) {
	b, err := os.ReadFile(filepath.Join(dir, "sprites.json"))
	if err != nil {
		return nil, err
	}
	var f struct {
		Sprites map[string]spriteSpec `json:"sprites"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("sprites.json: %w", err)
	}
	return &sprites{dir: dir, specs: f.Sprites, frames: map[frameKey]*image.RGBA{}}, nil
}

func (s *sprites) prepare(name string, roles map[string]bool) error {
	spec, ok := s.specs[name]
	if !ok {
		return fmt.Errorf("no live character %q", name)
	}
	tint := spec.Tint == nil || *spec.Tint
	for state, st := range spec.States {
		for i := 0; i < st.Frames; i++ {
			src, err := readPNG(filepath.Join(s.dir, name, fmt.Sprintf("%s_%d.png", state, i)))
			if err != nil {
				return err
			}
			for role := range roles {
				key := role
				if !tint {
					key = ""
				}
				k := frameKey{name, state, key, i}
				if _, done := s.frames[k]; done {
					continue
				}
				im := src
				if tint {
					im = tinted(src, roleColor[role])
				}
				s.frames[k] = scaled(im)
			}
		}
	}
	return nil
}

func (s *sprites) frame(name string, states []timedState, t float64, role string) *image.RGBA {
	spec := s.specs[name]
	ts, state, hold := 0.0, spec.Rest, false
	for _, x := range states {
		if t >= x.t {
			ts, state, hold = x.t, x.v, x.hold
		}
	}
	st := spec.States[state]
	i := int((t - ts) * st.FPS)
	switch {
	case st.Loop:
		i %= st.Frames
	case i >= st.Frames && hold:
		i = st.Frames - 1
	case i >= st.Frames:
		state = spec.Rest
		rs := spec.States[state]
		i = int((t-ts)*rs.FPS) % rs.Frames
	}
	if spec.Tint != nil && !*spec.Tint {
		role = ""
	}
	return s.frames[frameKey{name, state, role, i}]
}

func readPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	im, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return im, nil
}

func tinted(src image.Image, c color.RGBA) image.Image {
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

func scaled(src image.Image) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, int(float64(b.Dx())*scale), int(float64(b.Dy())*scale)))
	xdraw.NearestNeighbor.Scale(dst, dst.Bounds(), src, b, xdraw.Src, nil)
	return dst
}
