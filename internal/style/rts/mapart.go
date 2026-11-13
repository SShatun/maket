package rts

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"math/rand"
	"sort"

	xdraw "golang.org/x/image/draw"
)

type rawMap struct {
	Seed   *int64          `json:"seed"`
	Forest []rawForest     `json:"forest"`
	Path   json.RawMessage `json:"path"`
	Wall   *rawWall        `json:"wall"`
	Rocks  [][]float64     `json:"rocks"`
	Dirt   [][]float64     `json:"dirt"`
}

type rawForest struct {
	Rect  []float64 `json:"rect"`
	Count int       `json:"count"`
}

type rawWall struct {
	Gate string    `json:"gate"`
	Y    *float64  `json:"y"`
	X    []float64 `json:"x"`
}

type wall struct {
	im   *image.RGBA
	y    int
	gate *actor
}

var (
	speckDark  = color.RGBA{60, 104, 40, 255}
	speckLight = color.RGBA{98, 150, 66, 255}
)

func box4(v []float64, what string) (image.Rectangle, error) {
	if len(v) != 4 || v[0] >= v[2] || v[1] >= v[3] {
		return image.Rectangle{}, fmt.Errorf("%s wants [x0, y0, x1, y1] in map px 180x229", what)
	}
	return image.Rect(int(v[0]), int(v[1]), int(v[2]), int(v[3])), nil
}

func (s *Style) paint(m *rawMap, rng *rand.Rand, place func(json.RawMessage) ([2]float64, error)) (*image.RGBA, error) {
	a := s.art
	lg := image.NewRGBA(image.Rect(0, 0, mapW, mapH))
	for y := 0; y < mapH; y += 16 {
		for x := 0; x < mapW; x += 16 {
			draw.Draw(lg, image.Rect(x, y, x+16, y+16), a.grass[rng.Intn(4)], image.Point{}, draw.Src)
		}
	}
	for y := 0; y < mapH; y++ {
		for x := 0; x < mapW; x++ {
			switch r := rng.Float64(); {
			case r < 0.02:
				lg.SetRGBA(x, y, speckDark)
			case r < 0.035:
				lg.SetRGBA(x, y, speckLight)
			}
		}
	}
	if m == nil {
		return lg, nil
	}
	for i, d := range m.Dirt {
		r, err := box4(d, fmt.Sprintf("map.dirt %d", i+1))
		if err != nil {
			return nil, err
		}
		cells := map[image.Point]int{}
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				c := image.Pt(x/16, y/16)
				v, ok := cells[c]
				if !ok {
					v = rng.Intn(2)
					cells[c] = v
				}
				if image.Pt(x, y).In(lg.Bounds()) {
					lg.SetRGBA(x, y, a.dirt[v].RGBAAt(x%16, y%16))
				}
			}
		}
	}
	strokes, err := strokes(m.Path, place)
	if err != nil {
		return nil, err
	}
	mask := make([]bool, mapW*mapH)
	for _, st := range strokes {
		for i := 0; i+1 < len(st); i++ {
			p, q := st[i], st[i+1]
			n := int(math.Ceil(math.Hypot(q[0]-p[0], q[1]-p[1])*2)) + 1
			for k := 0; k <= n; k++ {
				u := float64(k) / float64(n)
				stamp(mask, p[0]+(q[0]-p[0])*u, p[1]+(q[1]-p[1])*u)
			}
		}
		if len(st) == 1 {
			stamp(mask, st[0][0], st[0][1])
		}
	}
	for i, on := range mask {
		if !on {
			continue
		}
		c := s.palette["path"]
		switch r := rng.Float64(); {
		case r < 0.14:
			c = s.palette["path_dark"]
		case r < 0.22:
			c = s.palette["path_light"]
		}
		lg.SetRGBA(i%mapW, i/mapW, c)
	}
	for i, r := range m.Rocks {
		if len(r) != 2 {
			return nil, fmt.Errorf("map.rocks %d wants [x, y]", i+1)
		}
		put(lg, a.rock, int(r[0]), int(r[1]))
	}
	var trees [][2]int
	for i, f := range m.Forest {
		r, err := box4(f.Rect, fmt.Sprintf("map.forest %d rect", i+1))
		if err != nil {
			return nil, err
		}
		if f.Count < 1 {
			return nil, fmt.Errorf("map.forest %d needs count >= 1", i+1)
		}
		for k := 0; k < f.Count; k++ {
			trees = append(trees, [2]int{r.Min.X + rng.Intn(r.Dx()), r.Min.Y + rng.Intn(r.Dy())})
		}
	}
	sort.SliceStable(trees, func(i, j int) bool { return trees[i][1] < trees[j][1] })
	for _, t := range trees {
		put(lg, a.tree, t[0], t[1])
	}
	return lg, nil
}

func put(dst, im *image.RGBA, x, y int) {
	w, h := im.Bounds().Dx(), im.Bounds().Dy()
	draw.Draw(dst, image.Rect(x-w/2, y-h+2, x-w/2+w, y+2), im, image.Point{}, draw.Over)
}

func stamp(mask []bool, cx, cy float64) {
	for y := int(cy) - 3; y <= int(cy)+3; y++ {
		for x := int(cx) - 4; x <= int(cx)+4; x++ {
			if x < 0 || y < 0 || x >= mapW || y >= mapH {
				continue
			}
			dx, dy := (float64(x)+0.5-cx)/3, (float64(y)+0.5-cy)/2.5
			if dx*dx+dy*dy <= 1 {
				mask[y*mapW+x] = true
			}
		}
	}
}

func strokes(raw json.RawMessage, place func(json.RawMessage) ([2]float64, error)) ([][][2]float64, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil || len(items) == 0 {
		return nil, fmt.Errorf("map.path wants a list of points [x, y] or \"ID\", or a list of such lists")
	}
	single := isPoint(items[0])
	var lines [][]json.RawMessage
	if single {
		lines = [][]json.RawMessage{items}
	} else {
		for _, it := range items {
			var l []json.RawMessage
			if err := json.Unmarshal(it, &l); err != nil {
				return nil, fmt.Errorf("map.path: %s is not a list of points", it)
			}
			lines = append(lines, l)
		}
	}
	var out [][][2]float64
	for i, l := range lines {
		var st [][2]float64
		for _, p := range l {
			v, err := place(p)
			if err != nil {
				return nil, fmt.Errorf("map.path %d: %w", i+1, err)
			}
			st = append(st, v)
		}
		out = append(out, st)
	}
	return out, nil
}

func isPoint(raw json.RawMessage) bool {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return true
	}
	var v []float64
	return json.Unmarshal(raw, &v) == nil
}

func (s *Style) wall(w *rawWall, byID map[string]*actor) (*wall, error) {
	if w == nil {
		return nil, nil
	}
	out := &wall{}
	x0, x1 := 0, mapW
	if len(w.X) == 2 {
		x0, x1 = int(w.X[0]), int(w.X[1])
	} else if len(w.X) != 0 {
		return nil, fmt.Errorf("map.wall.x wants [x0, x1]")
	}
	gap0, gap1 := 0, 0
	switch {
	case w.Gate != "":
		g := byID[w.Gate]
		if g == nil {
			return nil, fmt.Errorf("map.wall.gate: unknown entity %q", w.Gate)
		}
		out.gate = g
		out.y = int(math.Round(g.Home[1]))
		spec := s.set.Specs[g.Kind]
		ax := int(math.Round(g.Home[0]))
		gap0 = ax - spec.Anchor[0]/12
		gap1 = ax + (s.boxes12(g.Kind) - spec.Anchor[0]/12)
	case w.Y != nil:
		out.y = int(math.Round(*w.Y))
	default:
		return nil, fmt.Errorf("map.wall needs gate (id of the lock) or y")
	}
	lg := image.NewRGBA(image.Rect(0, 0, mapW, 14))
	for x := max(0, x0); x < min(mapW, x1); x++ {
		if x >= gap0 && x < gap1 {
			continue
		}
		c := (x - gap0) % 3
		if c < 0 {
			c += 3
		}
		for r := 0; r < 14; r++ {
			lg.SetRGBA(x, r, s.art.wallTile[r][c])
		}
	}
	out.im = image.NewRGBA(image.Rect(0, 0, mapW*px, 14*px))
	xdraw.NearestNeighbor.Scale(out.im, out.im.Bounds(), lg, lg.Bounds(), xdraw.Src, nil)
	return out, nil
}

func (s *Style) boxes12(kind string) int {
	spec := s.set.Specs[kind]
	b := s.box(kind)
	if b.Empty() {
		return spec.Size[0] / 12
	}
	return (b.Max.X*2 + 11) / 12
}

func minimap(lg *image.RGBA) *image.RGBA {
	mw := int(math.Round(float64(mapW) * 62 / mapH))
	out := image.NewRGBA(image.Rect(0, 0, mw, 62))
	sx, sy := float64(mapW)/float64(mw), float64(mapH)/62
	for j := 0; j < 62; j++ {
		for i := 0; i < mw; i++ {
			var r, g, b, n int
			for y := int(float64(j) * sy); y < int(math.Ceil(float64(j+1)*sy)) && y < mapH; y++ {
				for x := int(float64(i) * sx); x < int(math.Ceil(float64(i+1)*sx)) && x < mapW; x++ {
					c := lg.RGBAAt(x, y)
					if fogged(x, y) {
						c = black
					}
					r, g, b, n = r+int(c.R), g+int(c.G), b+int(c.B), n+1
				}
			}
			out.SetRGBA(i, j, color.RGBA{uint8(r / n), uint8(g / n), uint8(b / n), 255})
		}
	}
	return out
}
