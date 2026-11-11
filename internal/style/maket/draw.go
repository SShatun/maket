package maket

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"sort"
	"unicode/utf8"

	"github.com/fogleman/gg"
	"golang.org/x/image/draw"
	"golang.org/x/image/font"

	"maket/internal/render"
	"maket/internal/scene"
)

type tokenState struct {
	pos             point
	at              *scene.Point
	move            *event
	moving, waiting bool
}

func (m *motion) state(t float64) *tokenState {
	var last *event
	for i := range m.moves {
		ev := &m.moves[i]
		if t < ev.at {
			break
		}
		end := ev.at + ev.Dur
		if t <= end {
			k, arc := prog(t, ev.at, end), sendArc
			if ev.From.ID == ev.To.ID {
				arc = shiftArc
			}
			if ev.Arc != nil {
				arc = *ev.Arc
			}
			return &tokenState{
				pos:  point{lerp(ev.from.x, ev.to.x, k), lerp(ev.from.y, ev.to.y, k), lerp(ev.from.z, ev.to.z, k) + math.Sin(math.Pi*k)*arc},
				move: ev, moving: true,
			}
		}
		last = ev
	}
	if last == nil || last.Consume {
		return nil
	}
	return &tokenState{pos: last.to, at: last.To, move: last, waiting: last.Waiting}
}

func (s *Style) dim(id string, t float64) float64 {
	if s.focus == nil {
		return 1
	}
	for _, p := range s.focus.Targets {
		if p.ID == id {
			return 1
		}
	}
	return 1 - (1-focusDim)*prog(t, s.focus.at, s.focus.at+focusDur)
}

func valueAt(rows []timed, t float64, fallback string) string {
	for _, row := range rows {
		if row.at <= t {
			fallback = row.value
		}
	}
	return fallback
}

type item struct {
	mesh       *mesh
	pos        point
	rise, dx   float64
	depth      float64
	color, ink color.RGBA
	label      string
}

type label struct {
	entity      *entity
	x, top, bot float64
	opacity     float64
	blocked     bool
}

func (s *Style) Draw(dst *image.RGBA, t float64) {
	dc := gg.NewContext(dst.Bounds().Dx()*2, dst.Bounds().Dy()*2)
	dc.Scale(2, 2)
	dc.SetColor(ground)
	dc.Clear()
	c := s.camera(t)
	dc.SetColor(grid)
	dc.SetLineWidth(3)
	for _, line := range s.grid {
		x0, y0 := c.pt(line[0])
		x1, y1 := c.pt(line[1])
		dc.MoveTo(x0, y0)
		dc.LineTo(x1, y1)
		dc.Stroke()
	}
	labelFace, statusFace := render.Face(s.fonts.Semi, 60), render.Face(s.fonts.Mono, 44)
	tokenFace, smallFace := render.Face(s.fonts.MonoBold, 68), render.Face(s.fonts.MonoBold, 48)
	defer labelFace.Close()
	defer statusFace.Close()
	defer tokenFace.Close()
	defer smallFace.Close()
	var items []item
	var labels []label
	for i := range s.entities {
		e := &s.entities[i]
		r := rise((t - e.appear) / riseDur)
		if r <= 0 {
			continue
		}
		d := s.dim(e.ID, t)
		role := valueAt(e.roles, t, e.Role)
		blocked, blockStart := false, 0.0
		for _, ev := range s.events {
			if ev.Target == nil || ev.Target.ID != e.ID || t < ev.at || t >= ev.until {
				continue
			}
			if ev.Do == "state" {
				role = ev.Role
			}
			if ev.Do == "block" && !blocked {
				blocked, blockStart = true, ev.at
			}
		}
		if blocked {
			role = "error"
		}
		col := mix(roles[role], ground, d)
		fl := 0.0
		for _, f := range s.flashes {
			if f.id == e.ID && f.at-0.15 < t && t < f.at+0.35 {
				fl = math.Max(fl, 1-math.Abs(t-(f.at+0.1))/0.25)
			}
		}
		col = mix(color.RGBA{255, 255, 255, 255}, col, 0.35*fl)
		dx := 0.0
		if blocked && e.Kind == "block" && t < blockStart+shakeDur {
			dx = math.Sin(t*60) * shakePX * (1 - prog(t, blockStart, blockStart+shakeDur))
		}
		k := clamp(r) * d
		for _, p := range e.positions {
			polygon(dc, c, e.mesh.shadow, p, 1, 0, mix(shadow, ground, k))
			items = append(items, item{mesh: &e.mesh, pos: p, rise: r, dx: dx, depth: 2 * p.y, color: col})
		}
		x, bot := c.pab(point{e.At[0], e.At[1], 0})
		_, top := c.pab(point{e.At[0], e.At[1], e.height})
		half := e.width / 2 * c.u * c.zoom
		if e.Kind == "cylinder" {
			half *= 0.7071
		}
		labels = append(labels, label{e, x + dx, top - half, bot + half, k, blocked})
	}
	occupancy := map[string]int{}
	for i := range s.tokens {
		m := &s.tokens[i]
		st := m.state(t)
		if st == nil {
			continue
		}
		host := m.moves[0].From.ID
		if st.at != nil {
			host = st.at.ID
			if st.at.ID != "" && st.at.Slot >= 0 {
				occupancy[st.at.ID]++
			}
		}
		d := 1.0
		if !st.moving {
			d = s.dim(host, t)
		}
		col, textColor, shape := card, ink, &s.token
		if st.move.Role != "" {
			col, textColor = roles[st.move.Role], card
		}
		if st.waiting {
			col = mix(roles["error"], ground, 0.75+0.25*math.Sin(t*2*math.Pi*1.1))
			textColor, shape = card, &s.waiting
		}
		col = mix(col, ground, d)
		textColor = mix(textColor, col, d)
		items = append(items, item{mesh: shape, pos: st.pos, rise: 1, depth: 2*st.pos.y + 0.75*st.pos.z + 0.01, color: col, ink: textColor, label: m.label})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].depth < items[j].depth })
	for _, it := range items {
		polygon(dc, c, it.mesh.right, it.pos, it.rise, it.dx, shade(it.color, 0.80))
		polygon(dc, c, it.mesh.left, it.pos, it.rise, it.dx, shade(it.color, 0.64))
		for _, band := range it.mesh.bands {
			path(dc, c, band, it.pos, it.rise, it.dx)
			dc.SetColor(shade(it.color, 0.55))
			dc.SetLineWidth(4)
			dc.Stroke()
		}
		polygon(dc, c, it.mesh.top, it.pos, it.rise, it.dx, it.color)
		if it.label != "" {
			x, y := c.pab(point{it.pos.x, it.pos.y, it.pos.z + it.mesh.height*it.rise})
			face := tokenFace
			if utf8.RuneCountInString(it.label) > 3 {
				face = smallFace
			}
			text(dc, face, it.label, x+it.dx, y+2, 0.5, it.ink)
		}
	}
	for _, l := range labels {
		e := l.entity
		nameColor, statusColor := ink, muted
		if l.blocked {
			nameColor, statusColor = errorText, errorText
		}
		status := valueAt(e.status, t, "")
		if e.Kind == "slots" {
			n := occupancy[e.ID]
			full := n >= e.Count
			if full {
				statusColor = errorText
			}
			if status == "" {
				status = fmt.Sprintf("%d/%d", n, e.Count)
				if full && (e.FullWord == nil || *e.FullWord) {
					status += " забит"
				}
			}
		}
		nameColor, statusColor = mix(nameColor, ground, l.opacity), mix(statusColor, ground, l.opacity)
		x, y, anchor := l.x, l.bot+30, 0.5
		switch e.LabelPos {
		case "left", "right":
			half := e.width * 0.866 * c.u * c.zoom
			if e.Kind == "slots" {
				half += float64(e.Count-1) / 2 * e.Spacing * 2 * 0.866 * c.u * c.zoom
			}
			sign := 1.0
			anchor = 0
			if e.LabelPos == "left" {
				sign, anchor = -1, 1
			}
			x, y = x+sign*(half+24), (l.top+l.bot)/2-8-labelGap/2
		case "above":
			y = l.top - 40 - labelGap
		}
		text(dc, labelFace, e.Label, x, y, anchor, nameColor)
		if status != "" {
			text(dc, statusFace, status, x, y+labelGap, anchor, statusColor)
		}
	}
	draw.ApproxBiLinear.Scale(dst, dst.Bounds(), dc.Image(), dc.Image().Bounds(), draw.Src, nil)
}

func path(dc *gg.Context, c camera, points []point, pos point, rise, dx float64) {
	for i, p := range points {
		p.x, p.y, p.z = p.x+pos.y+pos.x, p.y+pos.y-pos.x, p.z*rise+pos.z
		x, y := c.pt(p)
		if i == 0 {
			dc.MoveTo(x+dx, y)
		} else {
			dc.LineTo(x+dx, y)
		}
	}
}

func polygon(dc *gg.Context, c camera, points []point, pos point, rise, dx float64, col color.RGBA) {
	path(dc, c, points, pos, rise, dx)
	dc.ClosePath()
	dc.SetColor(col)
	dc.Fill()
}

func text(dc *gg.Context, face font.Face, label string, x, y, anchor float64, col color.RGBA) {
	dc.Push()
	dc.Scale(0.5, 0.5)
	dc.SetFontFace(face)
	dc.SetColor(col)
	dc.DrawStringAnchored(label, x*2, y*2, anchor, 0.35)
	dc.Pop()
}
