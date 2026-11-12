package charkit

import (
	"bufio"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	cell  = 12
	gridW = 32
	gridH = 40
)

type gridState struct {
	name   string
	spec   State
	frames [][]string
}

type Grid struct {
	legend map[byte]color.NRGBA
	anchor [2]int
	top    [2]int
	rest   string
	tint   bool
	states []*gridState
}

func ParseGrid(r io.Reader) (*Grid, error) {
	g := &Grid{legend: map[byte]color.NRGBA{}, tint: true}
	sc := bufio.NewScanner(r)
	var cur *gridState
	var rows []string
	n := 0
	flush := func() error {
		if rows == nil {
			return nil
		}
		if len(rows) != gridH {
			return fmt.Errorf("state %s frame %d: %d rows, want %d", cur.name, len(cur.frames), len(rows), gridH)
		}
		cur.frames = append(cur.frames, rows)
		rows = nil
		return nil
	}
	for sc.Scan() {
		n++
		line := strings.TrimRight(sc.Text(), " \t\r")
		if rows != nil && len(rows) < gridH {
			if len(line) != gridW {
				return nil, fmt.Errorf("line %d: %d symbols, want %d", n, len(line), gridW)
			}
			for i := 0; i < len(line); i++ {
				if _, ok := g.legend[line[i]]; !ok {
					return nil, fmt.Errorf("line %d: symbol %q not in palette", n, line[i])
				}
			}
			rows = append(rows, line)
			continue
		}
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		var err error
		switch f[0] {
		case "palette":
			err = g.parsePalette(f[1:])
		case "anchor":
			g.anchor, err = pair(f[1:])
		case "top":
			g.top, err = pair(f[1:])
		case "rest":
			if len(f) != 2 {
				err = fmt.Errorf("rest NAME")
			} else {
				g.rest = f[1]
			}
		case "tint":
			if len(f) != 2 {
				err = fmt.Errorf("tint true|false")
			} else {
				g.tint, err = strconv.ParseBool(f[1])
			}
		case "state":
			if err = flush(); err == nil {
				cur, err = parseState(f[1:])
				for _, s := range g.states {
					if err == nil && s.name == cur.name {
						err = fmt.Errorf("state %s twice", cur.name)
					}
				}
				if err == nil {
					g.states = append(g.states, cur)
				}
			}
		case "frame":
			if cur == nil {
				err = fmt.Errorf("frame before state")
			} else if err = flush(); err == nil {
				rows = []string{}
			}
		default:
			err = fmt.Errorf("unknown line %q", f[0])
		}
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", n, err)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if err := flush(); err != nil {
		return nil, err
	}
	for _, s := range g.states {
		if len(s.frames) == 0 {
			return nil, fmt.Errorf("state %s: no frames", s.name)
		}
		s.spec.Frames = len(s.frames)
	}
	return g, nil
}

func (g *Grid) parsePalette(f []string) error {
	if len(f)%2 != 0 {
		return fmt.Errorf("palette: symbol and colour in pairs")
	}
	for i := 0; i < len(f); i += 2 {
		if len(f[i]) != 1 {
			return fmt.Errorf("palette: symbol %q is not one character", f[i])
		}
		if f[i+1] == "transparent" {
			g.legend[f[i][0]] = color.NRGBA{}
			continue
		}
		p := strings.Split(f[i+1], ",")
		if len(p) != 3 {
			return fmt.Errorf("palette: colour %q, want r,g,b", f[i+1])
		}
		var c [3]uint8
		for j, s := range p {
			v, err := strconv.Atoi(s)
			if err != nil || v < 0 || v > 255 {
				return fmt.Errorf("palette: colour %q", f[i+1])
			}
			c[j] = uint8(v)
		}
		g.legend[f[i][0]] = color.NRGBA{c[0], c[1], c[2], 255}
	}
	return nil
}

func pair(f []string) ([2]int, error) {
	if len(f) != 2 {
		return [2]int{}, fmt.Errorf("want two numbers")
	}
	a, err1 := strconv.Atoi(f[0])
	b, err2 := strconv.Atoi(f[1])
	if err1 != nil || err2 != nil {
		return [2]int{}, fmt.Errorf("want two numbers")
	}
	return [2]int{a, b}, nil
}

func parseState(f []string) (*gridState, error) {
	if len(f) != 4 || f[1] != "fps" || (f[3] != "loop" && f[3] != "once") {
		return nil, fmt.Errorf("state NAME fps N loop|once")
	}
	fps, err := strconv.ParseFloat(f[2], 64)
	if err != nil || fps <= 0 {
		return nil, fmt.Errorf("state %s: fps %q", f[0], f[2])
	}
	return &gridState{name: f[0], spec: State{FPS: fps, Loop: f[3] == "loop"}}, nil
}

func (g *Grid) Entry() Entry {
	e := Entry{
		Size: [2]int{gridW * cell, gridH * cell}, Anchor: g.anchor, Top: g.top, Pixel: cell,
		Rest: g.rest, Tint: &g.tint, States: map[string]State{},
	}
	for _, s := range g.states {
		e.States[s.name] = s.spec
	}
	return e
}

func (g *Grid) Write(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, s := range g.states {
		for i, rows := range s.frames {
			im := image.NewNRGBA(image.Rect(0, 0, gridW*cell, gridH*cell))
			for y, row := range rows {
				for x := 0; x < gridW; x++ {
					c := g.legend[row[x]]
					for dy := 0; dy < cell; dy++ {
						for dx := 0; dx < cell; dx++ {
							im.SetNRGBA(x*cell+dx, y*cell+dy, c)
						}
					}
				}
			}
			if err := writePNG(filepath.Join(dir, fmt.Sprintf("%s_%d.png", s.name, i)), im); err != nil {
				return err
			}
		}
	}
	b, err := json.MarshalIndent(g.Entry(), "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "entry.json"), append(b, '\n'), 0o644)
}

func GridFile(c *Catalog, name string, only string, frame int) (string, error) {
	e, ok := c.Entries[name]
	if !ok {
		return "", fmt.Errorf("no character %q", name)
	}
	names := e.StateNames()
	if only != "" {
		if _, ok := e.States[only]; !ok {
			return "", fmt.Errorf("%s has no state %q", name, only)
		}
		names = []string{only}
	}
	symbols := map[color.NRGBA]byte{{}: '.'}
	pool := "kWSMGabcdefghijlmnopqrstuvwxyzABCDEFHIJKLNOPQRTUVXYZ0123456789#$%&*+=@?"
	var body strings.Builder
	for _, st := range names {
		spec := e.States[st]
		mode := "once"
		if spec.Loop {
			mode = "loop"
		}
		fmt.Fprintf(&body, "state %s fps %s %s\n", st, strconv.FormatFloat(spec.FPS, 'f', -1, 64), mode)
		for i := 0; i < spec.Frames; i++ {
			if frame >= 0 && i != frame {
				continue
			}
			im, err := ReadPNG(filepath.Join(c.Dir, name, fmt.Sprintf("%s_%d.png", st, i)))
			if err != nil {
				return "", err
			}
			fmt.Fprintf(&body, "frame %d\n", i)
			for y := 0; y < gridH; y++ {
				for x := 0; x < gridW; x++ {
					p := opaque(im.At(x*cell+cell/2, y*cell+cell/2))
					s, ok := symbols[p]
					if !ok {
						if len(symbols)-1 >= len(pool) {
							return "", fmt.Errorf("%s: more colours than symbols", name)
						}
						s = pool[len(symbols)-1]
						symbols[p] = s
					}
					body.WriteByte(s)
				}
				body.WriteByte('\n')
			}
		}
	}
	type sym struct {
		s byte
		c color.NRGBA
	}
	var legend []sym
	for c, s := range symbols {
		legend = append(legend, sym{s, c})
	}
	sort.Slice(legend, func(i, j int) bool {
		return strings.IndexByte("."+pool, legend[i].s) < strings.IndexByte("."+pool, legend[j].s)
	})
	var head strings.Builder
	head.WriteString("palette")
	for _, l := range legend {
		if l.c.A == 0 {
			fmt.Fprintf(&head, " %c transparent", l.s)
		} else {
			fmt.Fprintf(&head, " %c %d,%d,%d", l.s, l.c.R, l.c.G, l.c.B)
		}
	}
	tint := e.Tint == nil || *e.Tint
	fmt.Fprintf(&head, "\nanchor %d %d\ntop %d %d\nrest %s\ntint %t\n", e.Anchor[0], e.Anchor[1], e.Top[0], e.Top[1], e.Rest, tint)
	return head.String() + body.String(), nil
}

func opaque(c color.Color) color.NRGBA {
	p := color.NRGBAModel.Convert(c).(color.NRGBA)
	if p.A == 0 {
		return color.NRGBA{}
	}
	return p
}

func ReadPNG(path string) (image.Image, error) {
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

func writePNG(path string, im image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	err = png.Encode(f, im)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}
