package noir

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"path/filepath"
	"unicode/utf8"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/math/f64"
)

type yarnColor struct{ Base, Hi, Lo rgb }
type boardSpec struct {
	Yarn struct {
		Twine, Red yarnColor
		Shadow     struct {
			Color         rgb
			Dx, Dy, Width float64
		}
		Outer, Core float64
		TwistEvery  float64 `json:"twist_every"`
		TwistLen    float64 `json:"twist_len"`
		TwistWidth  float64 `json:"twist_w"`
		ArrowAt     float64 `json:"arrow_at"`
	}
	Card struct {
		Size  point
		Icon  point `json:"icon_at"`
		Label struct{ At point }
		Sub   struct {
			labelPos
			Colour rgb
		}
		Shadow point
		Shade  rgb `json:"shadow_colour"`
	}
	Title struct {
		At     point
		Rot    float64
		T1, T2 labelPos
	}
	Note struct {
		T1, T2 labelPos
		Shadow point
	}
	Pin    struct{ Anchor point }
	Marker struct{ File string }
}

type card struct {
	ID, Icon, Label, Sub string
	Centre               point
	Rot                  float64
}

type note struct {
	T1, T2, Colour string
	Centre         point
	Rot            float64
}

type link struct {
	from, to string
	red      bool
}

func (l *link) UnmarshalJSON(b []byte) error {
	var v []json.RawMessage
	var flag int
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	if len(v) != 3 || json.Unmarshal(v[0], &l.from) != nil || json.Unmarshal(v[1], &l.to) != nil || json.Unmarshal(v[2], &flag) != nil || (flag != 0 && flag != 1) {
		return fmt.Errorf("link requires [from, to, 0 or 1]")
	}
	l.red = flag == 1
	return nil
}

type caseFile struct {
	Title, Sub, Culprit string
	Cards               []card
	Links               []link
	Notes               []note
}

type yarn struct {
	points []point
	arrow  *image.RGBA
	index  int
	red    bool
}

type paper struct {
	im             *image.RGBA
	centre, shadow point
}

type board struct {
	*art
	title, sub string
	base, full *image.RGBA
	marker     *point
	yarns      []yarn
	notes      []paper
}

func rotate(im *image.RGBA, degrees float64) *image.RGBA {
	if degrees == 0 {
		return im
	}
	s, c := math.Sincos(-degrees * math.Pi / 180)
	hw, hh := float64(im.Rect.Dx())/2, float64(im.Rect.Dy())/2
	w := int(math.Ceil(math.Abs(c)*hw+math.Abs(s)*hh)) * 2
	h := int(math.Ceil(math.Abs(c)*hh+math.Abs(s)*hw)) * 2
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	m := f64.Aff3{c, -s, float64(w)/2 - c*hw + s*hh, s, c, float64(h)/2 - s*hw - c*hh}
	xdraw.BiLinear.Transform(out, m, im, im.Rect, draw.Src, nil)
	return out
}

func centered(im *image.RGBA, pt point) image.Point {
	return image.Pt(int(pt[0]-float64(im.Rect.Dx())/2), int(pt[1]-float64(im.Rect.Dy())/2))
}

func (p paper) draw(dst *image.RGBA, shade color.RGBA) {
	at := centered(p.im, p.centre)
	draw.DrawMask(dst, p.im.Rect.Add(at.Add(p.shadow.pixel())), image.NewUniform(shade), image.Point{}, p.im, image.Point{}, draw.Over)
	paste(dst, p.im, at)
}

func clone(im *image.RGBA) *image.RGBA {
	out := image.NewRGBA(im.Rect)
	copy(out.Pix, im.Pix)
	return out
}

func (a *art) newBoard(raw json.RawMessage) (*board, error) {
	var file caseFile
	var name string
	if json.Unmarshal(raw, &name) == nil {
		if name != "429" && name != "502" {
			return nil, fmt.Errorf("unknown case %q: define it in cases or supply a case object", name)
		}
		if err := readJSON(filepath.Join(a.root, "board_kit", "case_"+name+".json"), &file); err != nil {
			return nil, err
		}
	} else if err := json.Unmarshal(raw, &file); err != nil {
		return nil, err
	}
	if len(file.Cards) < 1 || len(file.Cards) > 7 {
		return nil, fmt.Errorf("a case needs 1..7 cards")
	}
	b := &board{art: a, title: file.Title, sub: file.Sub}
	const kit = "board_kit/"
	for _, n := range []string{"cork", "card_blank", "title_blank", "pin_gold", "pin_red", "arrow_twine", "arrow_red", "note_white", "note_yellow"} {
		if err := a.load(kit + n + ".png"); err != nil {
			return nil, err
		}
	}
	if err := a.load(kit + a.spec.Marker.File); err != nil {
		return nil, err
	}
	b.base = clone(a.images[kit+"cork.png"])
	spec := &a.spec
	ink, red, shade := a.palette.Ink.ink(), a.palette.Red.ink(), spec.Card.Shade.ink()
	label := func(dst *image.RGBA, text string, pos labelPos, col color.RGBA) {
		width := 2*min(pos.At[0], float64(dst.Rect.Dx())-pos.At[0]) - 8
		a.text(dst, text, pos.At, pos.Px, width, col, true)
	}
	title := clone(a.images[kit+"title_blank.png"])
	label(title, file.Title, spec.Title.T1, ink)
	label(title, file.Sub, spec.Title.T2, red)
	paper{rotate(title, spec.Title.Rot), spec.Title.At, spec.Card.Shadow}.draw(b.base, shade)
	centres := map[string]point{}
	for _, c := range file.Cards {
		if _, ok := centres[c.ID]; ok || c.ID == "" {
			return nil, fmt.Errorf("missing or duplicate card id %q", c.ID)
		}
		centres[c.ID] = c.Centre
		im := clone(a.images[kit+"card_blank.png"])
		path := kit + "icons/" + c.Icon + ".png"
		if err := a.load(path); err != nil {
			return nil, err
		}
		paste(im, a.images[path], spec.Card.Icon.pixel())
		px := 26.
		if n := utf8.RuneCountInString(c.Label); n <= 6 {
			px = 40
		} else if n <= 9 {
			px = 34
		}
		col := ink
		if c.ID == file.Culprit {
			col = red
			pt := c.Centre
			b.marker = &pt
		}
		label(im, c.Label, labelPos{spec.Card.Label.At, px}, col)
		label(im, c.Sub, spec.Card.Sub.labelPos, spec.Card.Sub.Colour.ink())
		paper{rotate(im, c.Rot), c.Centre, spec.Card.Shadow}.draw(b.base, shade)
	}
	if file.Culprit != "" && b.marker == nil {
		return nil, fmt.Errorf("unknown culprit %q", file.Culprit)
	}
	for _, ln := range file.Links {
		from, ok1 := centres[ln.from]
		to, ok2 := centres[ln.to]
		if !ok1 || !ok2 || from == to {
			return nil, fmt.Errorf("invalid link %s to %s", ln.from, ln.to)
		}
		b.yarns = append(b.yarns, b.yarn(from, to, ln.red))
	}
	for _, n := range file.Notes {
		if n.Colour == "" {
			n.Colour = "yellow"
		}
		if n.Colour != "yellow" && n.Colour != "white" {
			return nil, fmt.Errorf("unknown note colour %q", n.Colour)
		}
		im := clone(a.images[kit+"note_"+n.Colour+".png"])
		label(im, n.T1, spec.Note.T1, ink)
		label(im, n.T2, spec.Note.T2, red)
		b.notes = append(b.notes, paper{rotate(im, n.Rot), n.Centre, spec.Note.Shadow})
	}
	b.full = clone(b.base)
	b.draw(b.full, float64(len(b.yarns)), true, true)
	return b, nil
}

func pinPoint(a, b, size point) point {
	dx, dy := b[0]-a[0], b[1]-a[1]
	if math.Abs(dx) < 10 {
		return point{a[0] + size[0]/2 - 30, a[1] + math.Copysign(size[1]/2-28, dy)}
	}
	k := min((size[0]/2-24)/math.Abs(dx), (size[1]/2-24)/math.Abs(dy))
	return point{a[0] + dx*k, a[1] + dy*k}
}

func (b *board) yarn(a, z point, red bool) yarn {
	start, end := pinPoint(a, z, b.spec.Card.Size), pinPoint(z, a, b.spec.Card.Size)
	dx, dy := end[0]-start[0], end[1]-start[1]
	length := math.Hypot(dx, dy)
	n := max(2, int(length/3))
	y := yarn{red: red, points: make([]point, n+1)}
	for i := range y.points {
		t := float64(i) / float64(n)
		p := point{start[0] + dx*t, start[1] + dy*t}
		if math.Abs(dx) < .3*math.Abs(dy) {
			p[0] += 12 * math.Sin(math.Pi*t)
		} else {
			p[1] += (16 + .05*length) * math.Sin(math.Pi*t)
		}
		y.points[i] = p
	}
	y.index = int(float64(len(y.points)) * b.spec.Yarn.ArrowAt)
	prev, next := y.points[max(0, y.index-3)], y.points[min(n, y.index+3)]
	asset := "arrow_twine"
	if red {
		asset = "arrow_red"
	}
	y.arrow = rotate(b.images["board_kit/"+asset+".png"], -math.Atan2(next[1]-prev[1], next[0]-prev[0])*180/math.Pi)
	return y
}

func stroke(dst *image.RGBA, a, b point, width float64, ink color.RGBA) {
	dx, dy := b[0]-a[0], b[1]-a[1]
	l := dx*dx + dy*dy
	r := width / 2
	for y := max(dst.Rect.Min.Y, int(min(a[1], b[1])-r)); y < min(dst.Rect.Max.Y, int(max(a[1], b[1])+r)+1); y++ {
		for x := max(dst.Rect.Min.X, int(min(a[0], b[0])-r)); x < min(dst.Rect.Max.X, int(max(a[0], b[0])+r)+1); x++ {
			xx, yy := float64(x)-a[0], float64(y)-a[1]
			t := clamp((xx*dx + yy*dy) / max(l, 1e-12))
			xx -= dx * t
			yy -= dy * t
			if xx*xx+yy*yy <= r*r {
				dst.SetRGBA(x, y, ink)
			}
		}
	}
}

func (b *board) draw(dst *image.RGBA, progress float64, mark, notes bool) {
	type pin struct {
		pos point
		red bool
	}
	var pins []pin
	spec := b.spec.Yarn
	for j, y := range b.yarns {
		u := clamp(progress - float64(j))
		if u <= 0 {
			break
		}
		last := int(float64(len(y.points)-1) * u)
		col := spec.Twine
		if y.red {
			col = spec.Red
		}
		for _, layer := range []struct {
			dx, dy, w float64
			c         rgb
		}{
			{spec.Shadow.Dx, spec.Shadow.Dy, spec.Shadow.Width, spec.Shadow.Color}, {0, 0, spec.Outer, col.Lo}, {0, 0, spec.Core, col.Base},
		} {
			for i := 0; i < last; i++ {
				a, z := y.points[i], y.points[i+1]
				stroke(dst, point{a[0] + layer.dx, a[1] + layer.dy}, point{z[0] + layer.dx, z[1] + layer.dy}, layer.w, layer.c.ink())
			}
		}
		step := max(1, int(spec.TwistEvery/3))
		for i := 0; i < last-1; i += step {
			a, z := y.points[i], y.points[i+1]
			tx, ty := z[0]-a[0], z[1]-a[1]
			l := max(math.Hypot(tx, ty), 1e-8)
			tx, ty = tx/l, ty/l
			dx, dy := -ty*spec.Core/2+tx*spec.TwistLen/2, tx*spec.Core/2+ty*spec.TwistLen/2
			c := col.Hi
			if i/step%2 == 1 {
				c = col.Lo
			}
			stroke(dst, point{a[0] - dx, a[1] - dy}, point{a[0] + dx, a[1] + dy}, spec.TwistWidth, c.ink())
		}
		if last >= y.index {
			paste(dst, y.arrow, centered(y.arrow, y.points[y.index]))
		}
		pins = append(pins, pin{y.points[0], y.red})
		if u == 1 {
			pins = append(pins, pin{y.points[last], y.red})
		}
	}
	for _, p := range pins {
		key := "board_kit/pin_gold.png"
		if p.red {
			key = "board_kit/pin_red.png"
		}
		paste(dst, b.images[key], p.pos.pixel().Sub(b.spec.Pin.Anchor.pixel()))
	}
	if mark && b.marker != nil {
		im := b.images["board_kit/"+b.spec.Marker.File]
		paste(dst, im, centered(im, *b.marker))
	}
	if notes {
		for _, n := range b.notes {
			n.draw(dst, b.spec.Card.Shade.ink())
			paste(dst, b.images["board_kit/pin_red.png"], n.centre.pixel().Sub(image.Pt(15, 67)))
		}
	}
}
