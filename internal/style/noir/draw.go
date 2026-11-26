package noir

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/math/f64"

	"maket/internal/render"
)

const momentTolerance = 1e-7

type stage struct {
	door                    string
	keys, letters           int
	logs                    [3]*action
	headline, plate, speech *action
}

func (v stage) sub(text string) string {
	text = strings.ReplaceAll(text, "{keys}", strconv.Itoa(v.keys))
	return strings.ReplaceAll(text, "{letters}", strconv.Itoa(v.letters))
}

func (e *action) headline() string {
	if e.Text != "" {
		return e.Text
	}
	if e.State == "stale" {
		return "ВЧЕРА"
	}
	return "СВЕЖАЯ"
}

func (sh *shot) stageAt(t float64) stage {
	v := stage{door: sh.Closed}
	if sh.place == "vault" {
		v.door = sh.Open
	}
	for _, e := range sh.actions {
		if t < e.at || t >= e.end {
			continue
		}
		switch e.Do {
		case "door":
			v.door = sh.Closed
			if e.State == "open" {
				v.door = sh.Open
			}
		case "keys":
			v.keys = e.Count
		case "letters":
			v.letters = e.Count
		case "headline":
			v.headline = e
		case "log":
			v.logs[e.Row-1] = e
		case "plate":
			v.plate = e
		case "say":
			v.speech = e
		}
	}
	return v
}

func (sh *shot) watch(src *image.RGBA, value string) (*image.RGBA, error) {
	if value == "" {
		value = "12:03"
	}
	v, err := time.Parse("15:04", value)
	if err != nil {
		return nil, fmt.Errorf("watch time wants HH:MM")
	}
	im := clone(src)
	centre := point{110, 150}
	m := float64(v.Minute()) / 60
	h := (float64(v.Hour()%12) + m) / 12
	for _, hand := range []struct{ turn, length, width float64 }{{h, 49, 6}, {m, 76, 4}} {
		s, c := math.Sincos(hand.turn * 2 * math.Pi)
		stroke(im, centre, point{centre[0] + s*hand.length, centre[1] - c*hand.length}, hand.width, sh.palette.Ink.ink())
	}
	return im, nil
}

func (sh *shot) miniature(src *image.RGBA) *image.RGBA {
	r := sh.Board
	im := image.NewRGBA(image.Rect(0, 0, r[2], r[3]))
	xdraw.BiLinear.Scale(im, im.Rect, src, src.Rect, draw.Src, nil)
	return im
}

func (sh *shot) caseAt(t float64) *image.RGBA {
	var reveal *action
	for _, e := range sh.actions {
		if e.Do == "reveal" && (reveal == nil || t >= e.at) {
			reveal = e
		}
	}
	b := sh.board
	n := float64(len(b.yarns))
	if reveal != nil && t < min(reveal.end, reveal.at+(n+1)*reveal.Every) {
		dt := max(0., t-reveal.at)
		line := math.Floor(dt / reveal.Every)
		progress := line + clamp((dt-line*reveal.Every)/reveal.Dur)
		im := clone(b.base)
		b.draw(im, progress, t >= reveal.at+n*reveal.Every, false)
		if sh.mini != nil {
			return sh.miniature(im)
		}
		return im
	}
	if sh.mini != nil {
		return sh.mini
	}
	return b.full
}

func (sh *shot) setting(dst *image.RGBA, t float64, v stage) {
	bg := sh.BG
	if bg == "" {
		bg = v.door
	}
	if bg != "" {
		paste(dst, sh.images[bg], image.Point{})
	}
	if sh.board != nil {
		paste(dst, sh.caseAt(t), image.Pt(sh.Board[0], sh.Board[1]))
	}
	for _, pos := range sh.Hooks[:v.keys] {
		im := sh.images[sh.Key]
		paste(dst, im, pos.pixel())
	}
	for _, pos := range sh.Holes[:v.letters] {
		im := sh.images[sh.Letter]
		paste(dst, im, pos.pixel())
	}
	if sh.place == "post" {
		for _, r := range []image.Rectangle{image.Rect(120, 520, 461, 801), image.Rect(620, 520, 961, 801)} {
			draw.Draw(dst, r, sh.images[bg], r.Min, draw.Src)
		}
	}
	if e := v.headline; e != nil {
		col := sh.palette.Ink.ink()
		if e.State == "stale" {
			col = sh.palette.Red.ink()
		}
		text, date := typedLines(e.headline(), e.Time, t-e.at, e.Dur)
		b := sh.Headline
		w := float64(b.Rect[2] - b.Rect[0] - 8)
		sh.text(dst, text, b.Line1.At, b.Line1.Px, w, col, true)
		sh.text(dst, date, b.Line2.At, b.Line2.Px, w, sh.palette.Ink.ink(), true)
	}
	for i, e := range v.logs {
		if e == nil {
			continue
		}
		c := sh.Cards[i]
		col := sh.palette.Ink.ink()
		if c.Red {
			col = sh.palette.Red.ink()
		}
		text, _ := typedLines(e.Text, "", t-e.at, e.Dur)
		sh.text(dst, e.Time, c.Time, sh.Font.Time, float64(c.Rect[2])-c.Time[0]-8, col, false)
		sh.text(dst, text, c.Text, sh.Font.Text, float64(c.Rect[2])-c.Text[0]-8, sh.palette.Ink.ink(), false)
	}
	for _, e := range sh.actions {
		if t < e.at || t >= e.end {
			continue
		}
		switch e.Do {
		case "prop":
			if e.Kind != "flash" {
				paste(dst, e.image, point(e.Pos).pixel())
			}
		case "neon":
			e.neon.draw(dst, t-e.at)
		}
	}
}

func (sh *shot) figures(dst *image.RGBA, t float64) []figure {
	var figures []figure
	var bounds image.Rectangle
	var hands *image.RGBA
	for _, p := range sh.people {
		if t < p.enters {
			continue
		}
		f := p.figure(t)
		if sh.place == "post" && p.atCounter && f.feet[0] >= sh.Clerk[0]-190 && f.feet[0] <= sh.Clerk[0]+210 && f.feet[1] <= sh.Clerk[1] {
			cut := int(sh.Counter)
			r := f.body.Rect
			r.Max.Y = min(r.Max.Y, cut-f.origin.Y)
			if r.Empty() {
				continue
			}
			f.body = f.body.SubImage(r).(*image.RGBA)
		} else {
			f.shadow(dst, sh.place == "interrogation")
		}
		if sh.place == "interrogation" && p.bust && p.kind == "pool" {
			hands = sh.images["fx/pool_hands.png"]
		}
		figures = append(figures, f)
		bounds = bounds.Union(f.body.Rect.Add(f.origin))
	}
	sort.SliceStable(figures, func(i, j int) bool { return figures[i].feet[1] < figures[j].feet[1] })
	bounds = bounds.Intersect(dst.Rect)
	if !bounds.Empty() {
		layer := image.NewRGBA(bounds)
		for _, f := range figures {
			paste(layer, f.body, f.origin)
		}
		sh.Light.apply(layer)
		paste(dst, layer, bounds.Min)
	}
	if sh.Desk.File != "" {
		paste(dst, sh.images[sh.Desk.File], sh.Desk.At.pixel())
	}
	if sh.place == "interrogation" {
		if hands != nil {
			paste(dst, hands, image.Point{})
		}
		paste(dst, sh.images["fx/detective_back.png"], image.Point{})
	}
	return figures
}

func (sh *shot) overlays(dst *image.RGBA, t float64, figures []figure) {
	for _, e := range sh.actions {
		if e.Do == "icon" && t >= e.at && t < e.end {
			for _, f := range figures {
				if f.person.id == e.Target.ID {
					pos := image.Pt(int(f.head[0])-e.image.Rect.Dx()/2, int(f.head[1])-e.image.Rect.Dy())
					paste(dst, e.image, pos)
				}
			}
		}
	}
	if sh.rain {
		i := int((t-sh.start)*12) % len(sh.FX)
		paste(dst, sh.images[sh.FX[i]], image.Point{})
	}
	for _, e := range sh.actions {
		if (e.Do == "flash" || e.Do == "prop" && e.Kind == "flash") && t >= e.at && t < e.end {
			paste(dst, e.image, point(e.Pos).pixel())
		}
	}
}

func (s *Style) Draw(dst *image.RGBA, t float64) {
	t += momentTolerance
	i := sort.Search(len(s.shots), func(i int) bool { return s.shots[i].start > t })
	sh := s.shots[max(0, i-1)]
	v := sh.stageAt(t)
	sh.setting(dst, t, v)
	figures := sh.figures(dst, t)
	sh.overlays(dst, t, figures)
	m := render.LinearCamera(sh.camera, sh.tl, t)
	if m[0] != 1 {
		src := clone(dst)
		xdraw.ApproxBiLinear.Transform(dst, f64.Aff3{m[0], 0, m[1], 0, m[0], m[2]}, src, src.Rect, draw.Src, nil)
	}
	var visible []string
	if e := v.plate; e != nil {
		sub := v.sub(e.Sub)
		sh.art.plate(dst, e.Title, sub, t-e.at, e.Dur)
		visible = append(visible, e.Title, sub)
	}
	if e := v.speech; e != nil {
		r := sh.Speech
		if r == (box{}) {
			r = box{100, 1480, 980, 1690}
		}
		if len(e.Box) == 4 {
			for i := range r {
				r[i] = int(e.Box[i])
			}
		}
		tip := point{float64(r[0] - 80), float64(r[1] + 20)}
		if e.Target != nil {
			for _, f := range figures {
				if f.person.id == e.Target.ID {
					tip = point{m[0]*f.head[0] + m[1], m[0]*f.head[1] + m[2]}
					break
				}
			}
		}
		if e.Tip != nil {
			tip = *e.Tip
		}
		sh.bubble(dst, e.Text, r, tip, e.Target != nil && e.Tip == nil)
		visible = append(visible, e.Text)
	}
	if sh.place == "board" {
		visible = append(visible, sh.board.title, sh.board.sub)
	}
	if i := render.CaptionPhrase(s.sc, s.tl, t); i >= 0 {
		value := caption(s.sc.Phrases[i])
		present := false
		for _, v := range visible {
			if strings.EqualFold(strings.Join(strings.Fields(v), " "), strings.Join(strings.Fields(value), " ")) {
				present = true
				break
			}
		}
		if !present {
			render.Caption(dst, s.sc, s.tl, s.theme, t)
		}
	}
	for _, e := range s.cuts {
		if distance := math.Abs(t - e.at); distance < e.Dur/2 {
			level := uint8(255 * (1 - 2*distance/e.Dur))
			c := color.RGBA{A: level}
			if e.State == "white" {
				c.R, c.G, c.B = level, level, level
			}
			draw.Draw(dst, dst.Rect, image.NewUniform(c), image.Point{}, draw.Over)
		}
	}
}
