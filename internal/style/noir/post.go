package noir

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"strconv"
)

const (
	postHorizon   = 0.0
	postCounter   = 860
	postEdge      = 884
	postPanel     = 892
	postPlinth    = 1090
	postFloor     = 1110
	postRail      = 556
	postSign      = 570
	postSignH     = 42
	postClerkFeet = 1000
	postFirst     = 1150.0
	postDepth     = 0.4 / 1150
	postStep      = 8.2e-5
	postMaxQueue  = 8
	postMaxWindow = 8
)

type postRoom struct {
	labels  []string
	centres []float64
	bg      *image.RGBA
}

func (sh *shot) newPostRoom(raw json.RawMessage) (*postRoom, error) {
	r := &postRoom{}
	var n int
	if len(raw) == 0 {
		n = 1
	} else if err := json.Unmarshal(raw, &n); err != nil {
		if err := json.Unmarshal(raw, &r.labels); err != nil {
			return nil, fmt.Errorf("windows must be a number or a list of labels")
		}
		n = len(r.labels)
	}
	if n < 1 || n > postMaxWindow {
		return nil, fmt.Errorf("windows must be from 1 to %d", postMaxWindow)
	}
	if len(r.labels) == 0 {
		for i := range n {
			r.labels = append(r.labels, "ОКНО "+strconv.Itoa(i+1))
		}
	}
	w := 1080.0 / float64(n)
	for i := range n {
		r.centres = append(r.centres, w*(float64(i)+.5))
	}
	key := fmt.Sprintf("post:%q", r.labels)
	if sh.images[key] == nil {
		sh.images[key] = sh.drawPostRoom(r)
	}
	r.bg = sh.images[key]
	return r, nil
}

func (r *postRoom) clerk(i int) slot {
	return slot{pos: point{r.centres[i], postClerkFeet}, scale: postDepth * postClerkFeet}
}

func (r *postRoom) queue(i, n int) ([]slot, error) {
	if i < 0 || i >= len(r.centres) {
		return nil, fmt.Errorf("window %d is not in the post (1…%d)", i+1, len(r.centres))
	}
	if n > postMaxQueue {
		return nil, fmt.Errorf("a post queue holds at most %d people", postMaxQueue)
	}
	wx := r.centres[i]
	side := 1.0
	if wx > 540 {
		side = -1
	}
	d0 := postFirst - postHorizon
	var out []slot
	for j := range n {
		d := 1 / (1/d0 - float64(j)*postStep)
		grow := d / d0
		zig := []float64{0, 80, 10, 90, 15, 95, 20, 100}[j] * side
		x := wx + (wx-540)*(grow-1)*.6 + zig*grow
		out = append(out, slot{pos: point{x, postHorizon + d}, scale: postDepth * (postHorizon + d), flip: x < wx})
	}
	return out, nil
}

func (sh *shot) drawPostRoom(r *postRoom) *image.RGBA {
	src := sh.images[sh.BG]
	im := image.NewRGBA(image.Rect(0, 0, 1080, 1920))
	draw.Draw(im, image.Rect(0, 0, 1080, 512), src, image.Point{}, draw.Src)
	wall := src.RGBAAt(60, 600)
	frame := src.RGBAAt(185, 300)
	draw.Draw(im, image.Rect(0, 512, 1080, postCounter), image.NewUniform(wall), image.Point{}, draw.Src)
	for _, y := range []int{476, 594} {
		draw.Draw(im, image.Rect(180, y, 900, y+118), src, image.Pt(180, 358), draw.Src)
	}
	draw.Draw(im, image.Rect(180, 712, 900, 740), image.NewUniform(frame), image.Point{}, draw.Src)

	fill := func(x0, y0, x1, y1 int, c color.RGBA) {
		draw.Draw(im, image.Rect(x0, y0, x1, y1), image.NewUniform(c), image.Point{}, draw.Src)
	}
	dark := color.RGBA{38, 26, 18, 255}
	n := len(r.centres)
	w := 1080 / n
	fill(0, postRail, 1080, postRail+14, dark)
	fill(0, postRail+14, 1080, postRail+16, color.RGBA{70, 50, 34, 255})
	for i := 0; i <= n; i++ {
		x := i * 1080 / n
		fill(x-8, postRail, x+8, postCounter, dark)
		fill(x-8, postRail, x-5, postCounter, color.RGBA{74, 54, 38, 255})
	}

	fill(0, postCounter, 1080, postEdge, color.RGBA{156, 118, 82, 255})
	fill(0, postCounter, 1080, postCounter+3, color.RGBA{186, 148, 108, 255})
	fill(0, postEdge, 1080, postPanel, color.RGBA{98, 68, 44, 255})
	fill(0, postPanel, 1080, postPlinth, color.RGBA{72, 46, 29, 255})
	for i := range n {
		x0, x1 := i*w+18, (i+1)*w-18
		fill(x0, postPanel+20, x1, postPlinth-22, color.RGBA{96, 64, 40, 255})
		fill(x0+5, postPanel+25, x1-5, postPlinth-27, color.RGBA{60, 38, 23, 255})
		fill(x0+5, postPanel+25, x1-5, postPanel+29, color.RGBA{44, 27, 16, 255})
	}
	fill(0, postPlinth, 1080, postFloor, color.RGBA{18, 13, 10, 255})

	const ss = 3
	tile := [2][3]float64{{24, 26, 32}, {66, 68, 76}}
	for y := postFloor; y < 1920; y++ {
		for x := range 1080 {
			var acc [3]float64
			for j := range ss {
				for i := range ss {
					fx := float64(x) + (float64(i)+.5)/ss
					fy := float64(y) + (float64(j)+.5)/ss
					d := fy - postHorizon
					u := (fx - 540) / d * 12.3
					v := 1 / d * 66000
					k := (int(math.Floor(u)) + int(math.Floor(v))) & 1
					for c := range 3 {
						acc[c] += tile[k][c]
					}
				}
			}
			shade := 1 - .4*math.Abs(float64(x)-540)/540
			if dy := float64(y - postFloor); dy < 40 {
				shade *= .55 + .45*dy/40
			}
			im.SetRGBA(x, y, color.RGBA{uint8(acc[0] / ss / ss * shade), uint8(acc[1] / ss / ss * shade), uint8(acc[2] / ss / ss * shade), 255})
		}
	}

	paper, red, ink := sh.palette.Paper.ink(), sh.palette.Red.ink(), sh.palette.Ink.ink()
	for i, label := range r.labels {
		sw := min(w-40, 300)
		x0 := int(r.centres[i]) - sw/2
		fill(x0, postSign, x0+sw, postSign+postSignH, paper)
		fill(x0, postSign, x0+10, postSign+postSignH, red)
		sh.text(im, label, point{r.centres[i] + 5, postSign + postSignH/2}, 30, float64(sw-24), ink, true)
	}
	return im
}

func (sh *shot) addClerks(taken map[int]bool) error {
	for i := range sh.post.centres {
		if taken[i] {
			continue
		}
		p, err := sh.person("", "queue", "stand", "calm", sh.post.clerk(i))
		if err != nil {
			return err
		}
		p.atCounter = true
		sh.people = append(sh.people, p)
	}
	return nil
}
