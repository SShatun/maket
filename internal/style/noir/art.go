package noir

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font/opentype"

	"maket/internal/render"
)

type point [2]float64
type box [4]int
type rgb [3]uint8

func (c rgb) ink() color.RGBA      { return color.RGBA{c[0], c[1], c[2], 255} }
func (p point) pixel() image.Point { return image.Pt(int(p[0]), int(p[1])) }

type portrait struct {
	Pos   point
	Scale float64
}

type lamp struct {
	Map    string
	Floor  float64
	Tint   [3]float64
	Rim    rgb
	Dir    [2]int `json:"rim_dir"`
	Pixels int    `json:"rim_px"`
	mask   *image.Gray
}

type labelPos struct {
	At point
	Px float64
}

type location struct {
	BG          string
	Closed      string `json:"bg_closed"`
	Open        string `json:"bg_open"`
	FX          []string
	Light       *lamp     `json:"people_light"`
	Queue       []point   `json:"queue_feet_head_to_tail"`
	QueueScale  float64   `json:"queue_scale"`
	QueueScales []float64 `json:"queue_scale_head_to_tail"`
	QueueFlip   bool      `json:"queue_flip"`
	Hooks       []point   `json:"key_hooks"`
	Key         string    `json:"key_sprite"`
	Holes       []point   `json:"pigeonholes"`
	Letter      string    `json:"letter_sprite"`
	Board       box       `json:"board_rect"`
	Speech      box       `json:"speech_box"`
	Desk        struct {
		File string
		At   point
	} `json:"desk_overlay"`
	Neon     neonSign
	Headline struct {
		Rect         box
		Line1, Line2 labelPos
	} `json:"headline_board"`
	Cards []struct {
		Rect box
		Time point `json:"time_at"`
		Text point `json:"text_at"`
		Red  bool  `json:"red_tab"`
	}
	Font struct {
		Time float64 `json:"time_px"`
		Text float64 `json:"text_px"`
	}
	Doorman      point     `json:"doorman_feet"`
	DoormanScale float64   `json:"doorman_scale"`
	Workers      []point   `json:"worker_feet"`
	WorkerScales []float64 `json:"worker_scale"`
	Clients      []point   `json:"clients_feet"`
	ClientScale  float64   `json:"client_scale"`
	Crowd        []point   `json:"crowd_feet"`
	CrowdScale   float64   `json:"crowd_scale"`
	Clerk        point     `json:"clerk_feet"`
	Buyer        point     `json:"buyer_feet"`
	Newsboy      point     `json:"newsboy_feet"`
	NewsboyScale float64   `json:"newsboy_scale"`
	Pool         portrait  `json:"pool_bust"`
	Detective    portrait  `json:"detective_bust"`
	Suspect      portrait  `json:"suspect_bust"`
	Accountant   portrait  `json:"accountant_bust"`
}

type sprite struct {
	Head map[string]point `json:"head_at"`
}

type art struct {
	root    string
	images  map[string]*image.RGBA
	places  map[string]*location
	sprites struct {
		Feet            point
		Canvas          [2]int
		People, Clients map[string]sprite
		Emotions        []string
	}
	palette struct {
		Font            string
		Ink, Paper, Red rgb
		Caption         struct {
			Title float64 `json:"title_px"`
			Sub   float64 `json:"sub_px"`
		}
		Speech struct{ Px float64 }
	}
	spec boardSpec
	font *opentype.Font
}

func readJSON(path string, dst any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(data, dst); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func loadArt(root string) (*art, error) {
	a := &art{root: root, images: make(map[string]*image.RGBA)}
	for _, file := range []struct {
		name string
		dst  any
	}{
		{"scenes.json", &a.places}, {"sprites.json", &a.sprites}, {"style.json", &a.palette}, {"board_kit/spec.json", &a.spec},
	} {
		if err := readJSON(filepath.Join(root, file.name), file.dst); err != nil {
			return nil, err
		}
	}
	f, err := render.LoadFont(filepath.Join(root, a.palette.Font))
	a.font = f
	return a, err
}

func (a *art) load(names ...string) error {
	for _, name := range names {
		if name == "" || a.images[name] != nil {
			continue
		}
		if !filepath.IsLocal(name) || strings.Contains(name, "..") {
			return fmt.Errorf("invalid asset %q", name)
		}
		f, err := os.Open(filepath.Join(a.root, name))
		if err != nil {
			return err
		}
		im, err := png.Decode(f)
		f.Close()
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		dst := image.NewRGBA(im.Bounds())
		draw.Draw(dst, dst.Bounds(), im, image.Point{}, draw.Src)
		a.images[name] = dst
	}
	return nil
}

func (a *art) loadPlace(p *location) error {
	if err := a.load(p.BG, p.Closed, p.Open, p.Key, p.Letter, p.Desk.File); err != nil {
		return err
	}
	if err := a.load(p.FX...); err != nil {
		return err
	}
	if p.Suspect.Scale > 0 {
		if err := a.load("fx/pool_hands.png", "fx/detective_back.png"); err != nil {
			return err
		}
	}
	if l := p.Light; l != nil && l.mask == nil {
		if err := a.load(l.Map); err != nil {
			return err
		}
		src := a.images[l.Map]
		l.mask = image.NewGray(src.Bounds())
		draw.Draw(l.mask, l.mask.Bounds(), src, image.Point{}, draw.Src)
		delete(a.images, l.Map)
	}
	return nil
}

func paste(dst *image.RGBA, src *image.RGBA, pos image.Point) {
	draw.Draw(dst, src.Bounds().Add(pos.Sub(src.Rect.Min)), src, src.Rect.Min, draw.Over)
}

func resize(src *image.RGBA, k float64, flip bool) *image.RGBA {
	if k == 1 && !flip {
		return src
	}
	r := image.Rect(0, 0, max(1, int(float64(src.Rect.Dx())*k)), max(1, int(float64(src.Rect.Dy())*k)))
	dst := image.NewRGBA(r)
	xdraw.BiLinear.Scale(dst, r, src, src.Rect, draw.Src, nil)
	if flip {
		for y := 0; y < r.Dy(); y++ {
			row := dst.Pix[y*dst.Stride:][:r.Dx()*4]
			for i, j := 0, len(row)-4; i < j; i, j = i+4, j-4 {
				left, right := (*[4]byte)(row[i:]), (*[4]byte)(row[j:])
				*left, *right = *right, *left
			}
		}
	}
	return dst
}

func clamp(v float64) float64 { return math.Max(0, math.Min(1, v)) }
