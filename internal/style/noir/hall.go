package noir

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"sort"
	"strconv"

	"github.com/fogleman/gg"
)

const (
	hallWindow    = 1.3
	hallCounterH  = 1.1
	hallCounterD  = 0.6
	hallClerkZone = 0.9
	hallFirst     = 0.65
	hallStep      = 1.05
	hallMan       = 1.75
	hallTop       = 300.0
	hallBottom    = 1870.0
	hallMaxWindow = 48
	hallMaxRow    = 12
	hallMaxQueue  = 8
)

var hallCoats = []rgb{{52, 60, 78}, {104, 94, 74}, {74, 44, 36}, {78, 82, 88}, {34, 35, 40}, {90, 70, 44}}

type hallCam struct{ f, hc, yh float64 }

func (c hallCam) x(X, Z float64) float64 { return 540 + c.f*X/Z }
func (c hallCam) y(Y, Z float64) float64 { return c.yh + c.f*(c.hc-Y)/Z }
func (c hallCam) z(Y, y float64) float64 { return c.f * (c.hc - Y) / (y - c.yh) }

type hallPiece struct {
	im *image.RGBA
	y  float64
}

type hall struct {
	hallCam
	windows, per, rows int
	queue              []int
	z0, pitch, lane    float64
	zWall              float64
	bg                 *image.RGBA
	pieces             []hallPiece
}

func parseHall(raw json.RawMessage) (*hall, bool, error) {
	var spec struct {
		Room    string
		Windows int
		Queue   json.RawMessage
		PerRow  int `json:"per_row"`
	}
	if len(raw) == 0 || raw[0] != '{' || json.Unmarshal(raw, &spec) != nil || spec.Room == "" {
		return nil, false, nil
	}
	if spec.Room != "post" {
		return nil, true, fmt.Errorf("unknown room %q, known: post", spec.Room)
	}
	n := spec.Windows
	if n < 1 || n > hallMaxWindow {
		return nil, true, fmt.Errorf("windows must be from 1 to %d", hallMaxWindow)
	}
	per := min(n, hallMaxRow)
	if spec.PerRow != 0 {
		if spec.PerRow < 1 || spec.PerRow > hallMaxRow {
			return nil, true, fmt.Errorf("per_row must be from 1 to %d", hallMaxRow)
		}
		per = min(n, spec.PerRow)
	}
	h := &hall{windows: n, per: per, rows: (n + per - 1) / per, queue: make([]int, n)}
	if len(spec.Queue) > 0 {
		var one int
		if json.Unmarshal(spec.Queue, &one) == nil {
			for i := range h.queue {
				h.queue[i] = one
			}
		} else if err := json.Unmarshal(spec.Queue, &h.queue); err != nil || len(h.queue) != n {
			return nil, true, fmt.Errorf("queue must be a number or a list of %d numbers", n)
		}
	}
	for _, q := range h.queue {
		if q < 0 || q > hallMaxQueue {
			return nil, true, fmt.Errorf("a queue holds from 0 to %d people", hallMaxQueue)
		}
	}
	h.fit()
	return h, true, nil
}

func (h *hall) fit() {
	longest := 0
	for _, q := range h.queue {
		longest = max(longest, q)
	}
	h.lane = hallFirst + float64(max(longest-1, 0))*hallStep + 0.5
	h.pitch = hallCounterD + hallClerkZone + h.lane + 0.9
	span := float64(h.per) * hallWindow
	h.z0 = h.lane + 3.2
	if h.rows > 1 {
		h.z0 = math.Max(h.z0, span*0.75)
	}
	near := h.z0 - h.lane + 0.2
	back := h.z0 + float64(h.rows-1)*h.pitch + hallCounterD
	h.hc = 1.15 * h.z0
	byWidth := 1010 * h.z0 / (span + 0.5)
	byHeight := (hallBottom - hallTop) / (h.hc/near - (h.hc-2.9)/back)
	h.f = math.Min(byWidth, byHeight)
	if byWidth < byHeight {
		h.hc = math.Max(h.hc, ((hallBottom-hallTop)/byWidth-2.9/back)/(1/near-1/back))
	}
	h.yh = hallBottom - h.f*h.hc/near
	h.zWall = back + hallClerkZone + 1.2
}

func (h *hall) counter(r int) (zc, x0 float64, first, count int) {
	first = r * h.per
	count = min(h.per, h.windows-first)
	return h.z0 + float64(r)*h.pitch, -float64(count) * hallWindow / 2, first, count
}

func (h *hall) window(i int) (X, zc float64) {
	zc, x0, first, _ := h.counter(i / h.per)
	return x0 + (float64(i-first)+.5)*hallWindow, zc
}

func (h *hall) slot(X, Z float64) slot {
	return slot{pos: point{h.x(X, Z), h.y(0, Z)}, scale: h.f * hallMan / Z / 1017}
}

func (h *hall) clerk(i int) slot {
	X, zc := h.window(i)
	return h.slot(X, zc+hallCounterD+0.45)
}

func (h *hall) inLine(i, j int) slot {
	X, zc := h.window(i)
	return h.slot(X, zc-hallFirst-float64(j)*hallStep)
}

func (h *hall) fill(dst *image.RGBA, r image.Rectangle, c color.RGBA) {
	draw.Draw(dst, r, image.NewUniform(c), image.Point{}, draw.Over)
}

func (h *hall) plane(dst *image.RGBA, Y, z0, z1, x0, x1 float64, col func(X, Z float64) color.RGBA) {
	ya, yb := int(h.y(Y, z1)), int(h.y(Y, z0))
	for y := max(ya, 0); y < min(yb, dst.Rect.Max.Y); y++ {
		z := h.z(Y, float64(y)+.5)
		for x := max(int(h.x(x0, z)), 0); x < min(int(h.x(x1, z)), dst.Rect.Max.X); x++ {
			dst.SetRGBA(x, y, col((float64(x)+.5-540)*z/h.f, z))
		}
	}
}

func (h *hall) front(dst *image.RGBA, Z, y0, y1, x0, x1 float64, c color.RGBA) image.Rectangle {
	r := image.Rect(int(h.x(x0, Z)), int(h.y(y1, Z)), int(h.x(x1, Z)), int(h.y(y0, Z)))
	h.fill(dst, r, c)
	return r
}

func (h *hall) light(X, Z float64) float64 {
	l := 0.0
	for r := range h.rows {
		zc, x0, _, count := h.counter(r)
		i := math.Floor((X - x0) / hallWindow)
		if i < 0 || int(i) >= count {
			continue
		}
		dx := (X - (x0 + (i+.5)*hallWindow)) / (0.34 * hallWindow)
		dz := (Z - (zc - h.lane*0.5 + 0.2)) / (h.lane*0.5 + 0.35)
		fade := 1 - 0.45*clamp((zc-Z)/h.lane)
		l = math.Max(l, clamp((1-math.Hypot(dx, dz))/0.25)*fade)
	}
	return l
}

func (sh *shot) buildHall() {
	h := sh.hall
	key := fmt.Sprintf("hall:%d:%d:%v", h.windows, h.per, h.queue)
	if im := sh.images[key]; im != nil {
		h.bg = im
	} else {
		h.bg = sh.hallBackground()
		sh.images[key] = h.bg
	}
	h.pieces = nil
	for r := range h.rows {
		h.pieces = append(h.pieces, sh.hallCounter(r))
		zc, x0, _, count := h.counter(r)
		for i := 0; i <= count; i++ {
			h.lanePieces(x0+float64(i)*hallWindow, zc)
		}
	}
	sort.SliceStable(h.pieces, func(i, j int) bool { return h.pieces[i].y < h.pieces[j].y })
}

func (sh *shot) hallBackground() *image.RGBA {
	h := sh.hall
	im := image.NewRGBA(image.Rect(0, 0, 1080, 1920))
	base := int(h.y(0, h.zWall))
	for y := 0; y < base; y++ {
		k := 0.7 + 0.3*float64(y)/float64(base)
		h.fill(im, image.Rect(0, y, 1080, y+1), color.RGBA{uint8(36 * k), uint8(42 * k), uint8(52 * k), 255})
	}
	cell := 0.6
	cols := int(float64(h.per)*hallWindow/cell) + 2
	x0 := -float64(cols) * cell / 2
	ceiling := h.hc - (240-h.yh)*h.zWall/h.f
	rows := max(2, min(8, int((ceiling-2.6)/0.5)))
	h.front(im, h.zWall, 2.45, 2.7+float64(rows)*0.5, x0-0.1, x0+float64(cols)*cell+0.1, color.RGBA{24, 16, 11, 255})
	for row := range rows {
		Y := 2.6 + float64(row)*0.5
		for i := range cols {
			X := x0 + float64(i)*cell
			h.front(im, h.zWall, Y+0.04, Y+0.46, X+0.04, X+cell-0.04, color.RGBA{8, 7, 6, 255})
			if (i*7+row*3)%5 < 2 {
				h.front(im, h.zWall, Y+0.1, Y+0.28, X+0.1, X+cell-0.1, color.RGBA{226, 230, 228, 255})
				h.front(im, h.zWall, Y+0.12, Y+0.18, X+cell-0.2, X+cell-0.14, sh.palette.Red.ink())
			}
		}
	}
	h.front(im, h.zWall, 0, 0.15, -100, 100, color.RGBA{14, 11, 9, 255})
	dark := [2][3]float64{{30, 32, 40}, {44, 47, 58}}
	lit := [2][3]float64{{118, 108, 88}, {142, 130, 106}}
	h.plane(im, 0, 0.3, h.zWall, -100, 100, func(X, Z float64) color.RGBA {
		k := (int(math.Floor(X/0.8)) + int(math.Floor(Z/0.8))) & 1
		l := h.light(X, Z)
		var o [3]uint8
		for j := range 3 {
			o[j] = uint8(dark[k][j]*(1-l) + lit[k][j]*l)
		}
		return color.RGBA{o[0], o[1], o[2], 255}
	})
	return im
}

func (sh *shot) hallCounter(r int) hallPiece {
	h := sh.hall
	zc, x0, first, count := h.counter(r)
	zb := zc + hallCounterD
	x1 := x0 + float64(count)*hallWindow
	im := image.NewRGBA(image.Rect(0, 0, 1080, 1920))
	wood := color.RGBA{30, 20, 13, 255}
	for i := 0; i <= count; i++ {
		X := x0 + float64(i)*hallWindow
		h.front(im, zb, hallCounterH, 2.35, X-0.05, X+0.05, wood)
	}
	h.front(im, zb, 2.35, 2.5, x0-0.05, x1+0.05, wood)
	for i := range count {
		X := x0 + (float64(i)+.5)*hallWindow
		sign := h.front(im, zb, 2.55, 2.85, X-0.5, X+0.5, sh.palette.Paper.ink())
		red := sign
		red.Max.X = red.Min.X + max(2, sign.Dx()/12)
		h.fill(im, red, sh.palette.Red.ink())
		if px := float64(sign.Dy()) * 0.8; px >= 11 {
			mid := point{float64(sign.Min.X+sign.Max.X)/2 + float64(red.Dx())/2, float64(sign.Min.Y+sign.Max.Y) / 2}
			sh.text(im, strconv.Itoa(first+i+1), mid, px, float64(sign.Dx()-red.Dx()-4), sh.palette.Ink.ink(), true)
		}
	}
	h.plane(im, 0, zc-0.22, zc, x0-0.05, x1+0.15, func(X, Z float64) color.RGBA { return color.RGBA{10, 10, 13, 255} })
	h.plane(im, hallCounterH, zc, zb, x0-0.05, x1+0.05, func(X, Z float64) color.RGBA {
		k := 1 - 0.2*(Z-zc)/hallCounterD
		return color.RGBA{uint8(204 * k), uint8(158 * k), uint8(104 * k), 255}
	})
	face := h.front(im, zc, 0, hallCounterH, x0-0.05, x1+0.05, color.RGBA{92, 60, 36, 255})
	lip := face
	lip.Max.Y = lip.Min.Y + max(2, face.Dy()/12)
	h.fill(im, lip, color.RGBA{232, 194, 140, 255})
	for i := range count {
		X := x0 + float64(i)*hallWindow
		h.front(im, zc, 0.18, 0.95, X+0.12, X+hallWindow-0.12, color.RGBA{68, 43, 26, 255})
	}
	h.front(im, zc, 0, 0.12, x0-0.05, x1+0.05, color.RGBA{22, 15, 10, 255})
	return hallPiece{crop(im), h.y(0, zc)}
}

func (h *hall) lanePieces(X, zc float64) {
	end := zc - h.lane
	var posts []float64
	for z := zc - 0.3; z >= end-0.01; z -= 1.2 {
		posts = append(posts, z)
	}
	if len(posts) == 0 || posts[len(posts)-1] > end+0.3 {
		posts = append(posts, end)
	}
	for k, z := range posts {
		w := math.Max(2, h.f*0.05/z)
		x, top, bottom := h.x(X, z), h.y(0.95, z), h.y(0, z)
		h.pieces = append(h.pieces, hallPiece{stroke2(x, top, x, bottom, w, color.RGBA{176, 140, 72, 255}, color.RGBA{214, 178, 104, 255}), bottom})
		if k == 0 {
			continue
		}
		const parts = 8
		za := posts[k-1]
		for s := range parts {
			t0, t1 := float64(s)/parts, float64(s+1)/parts
			z0, z1 := za+(z-za)*t0, za+(z-za)*t1
			y0, y1 := 0.88-0.18*math.Sin(math.Pi*t0), 0.88-0.18*math.Sin(math.Pi*t1)
			rw := math.Max(2, h.f*0.035/z)
			im := stroke2(h.x(X, z0), h.y(y0, z0), h.x(X, z1), h.y(y1, z1), rw, color.RGBA{168, 30, 38, 255}, color.RGBA{})
			h.pieces = append(h.pieces, hallPiece{im, h.y(0, (z0+z1)/2)})
		}
	}
}

func stroke2(x0, y0, x1, y1, w float64, c, cap color.RGBA) *image.RGBA {
	r := image.Rect(int(math.Min(x0, x1)-w*2), int(math.Min(y0, y1)-w*2), int(math.Max(x0, x1)+w*2)+1, int(math.Max(y0, y1)+w*2)+1)
	dc := gg.NewContext(r.Dx(), r.Dy())
	dc.Translate(-float64(r.Min.X), -float64(r.Min.Y))
	dc.SetColor(c)
	dc.SetLineWidth(w)
	dc.SetLineCapRound()
	dc.DrawLine(x0, y0, x1, y1)
	dc.Stroke()
	if cap.A > 0 {
		dc.SetColor(cap)
		dc.DrawRectangle(x0-w, y0-w, 2*w, 1.5*w)
		dc.Fill()
	}
	im := dc.Image().(*image.RGBA)
	im.Rect = r
	return im
}

func crop(im *image.RGBA) *image.RGBA {
	b := image.Rectangle{}
	for y := im.Rect.Min.Y; y < im.Rect.Max.Y; y++ {
		for x := im.Rect.Min.X; x < im.Rect.Max.X; x++ {
			if im.Pix[im.PixOffset(x, y)+3] != 0 {
				b = b.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return im.SubImage(b).(*image.RGBA)
}

func (sh *shot) addHallPeople(taken map[int]bool) error {
	h := sh.hall
	for i := range h.windows {
		if !taken[i] {
			p, err := sh.person("", "queue", "stand", "calm", h.clerk(i))
			if err != nil {
				return err
			}
			sh.people = append(sh.people, p)
		}
		for j := range h.queue[i] {
			p, err := sh.person("", "c"+strconv.Itoa((i*5+j*3)%6), "stand", "calm", h.inLine(i, j))
			if err != nil {
				return err
			}
			coat := hallCoats[(i*7+j*5+i/h.per*3)%len(hallCoats)]
			p.coat = &coat
			sh.people = append(sh.people, p)
		}
	}
	return nil
}

func (sh *shot) hallFigures(dst *image.RGBA, t float64) []figure {
	var figures []figure
	for _, p := range sh.people {
		if t >= p.enters {
			figures = append(figures, p.figure(t))
		}
	}
	sort.SliceStable(figures, func(i, j int) bool { return figures[i].feet[1] < figures[j].feet[1] })
	k := 0
	for _, f := range figures {
		for ; k < len(sh.hall.pieces) && sh.hall.pieces[k].y <= f.feet[1]; k++ {
			draw.Draw(dst, sh.hall.pieces[k].im.Rect, sh.hall.pieces[k].im, sh.hall.pieces[k].im.Rect.Min, draw.Over)
		}
		f.shadow(dst, false)
		paste(dst, f.body, f.origin)
	}
	for _, p := range sh.hall.pieces[k:] {
		draw.Draw(dst, p.im.Rect, p.im, p.im.Rect.Min, draw.Over)
	}
	return figures
}

func backCoat(im *image.RGBA, hair int, tone rgb) {
	for y := im.Rect.Min.Y; y < im.Rect.Max.Y; y++ {
		for x := im.Rect.Min.X; x < im.Rect.Max.X; x++ {
			px := im.Pix[im.PixOffset(x, y):][:4]
			if px[3] == 0 {
				continue
			}
			l := (float64(px[0]) + float64(px[1]) + float64(px[2])) / 765
			var c [3]float64
			switch {
			case y < hair-10:
				c = [3]float64{38, 32, 28}
			case y < hair+20:
				c = [3]float64{150, 112, 92}
			case y < 660:
				c = [3]float64{float64(tone[0]), float64(tone[1]), float64(tone[2])}
			default:
				c = [3]float64{30, 30, 34}
			}
			k := (0.82 + 0.36*l) * float64(px[3]) / 255
			for j := range 3 {
				px[j] = uint8(math.Min(255, c[j]*k))
			}
		}
	}
}
