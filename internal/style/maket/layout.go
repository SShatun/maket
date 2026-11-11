package maket

import (
	"fmt"
	"math"
	"sort"

	"maket/internal/audio"
	"maket/internal/render"
	"maket/internal/scene"
	"maket/internal/style"
	"maket/internal/timeline"
)

type point struct{ x, y, z float64 }

type timed struct {
	at    float64
	value string
}

type entity struct {
	scene.Entity
	appear        float64
	width, height float64
	positions     []point
	status, roles []timed
	mesh          mesh
}

type event struct {
	scene.Event
	at, until        float64
	from, to, target point
}

type motion struct {
	label string
	moves []event
}

type flash struct {
	at float64
	id string
}

type Style struct {
	fonts          *render.Fonts
	theme          render.Theme
	duration       float64
	entities       []entity
	ids            map[string]int
	events         []event
	tokens         []motion
	flashes        []flash
	focus          *event
	cues           []audio.Cue
	u, ox, oy      float64
	fx, fy, cx, cy float64
	grid           [][2]point
	token, waiting mesh
}

func New(_ string, fonts *render.Fonts) (*Style, error) {
	return &Style{fonts: fonts, theme: render.LightTheme(fonts)}, nil
}

func (s *Style) Layout(sc *scene.Scene, tl *timeline.Timeline) error {
	s.duration, s.ids = tl.Duration, make(map[string]int)
	resolve := func(rows []scene.Timed[string]) []timed {
		out := make([]timed, len(rows))
		for i, row := range rows {
			out[i] = timed{tl.MustResolve(row.At), row.V}
		}
		return out
	}
	for i, source := range sc.Entities {
		e := entity{Entity: source, appear: 0.05 + float64(i)*0.1}
		if e.Appear != nil {
			e.appear = tl.MustResolve(*e.Appear)
		}
		if len(e.At) != 2 {
			return fmt.Errorf("entity %s: maket style wants at: [a, b]", e.ID)
		}
		if e.Kind == "" {
			e.Kind = "block"
		}
		if e.Role == "" {
			e.Role = "producer"
		}
		if e.LabelPos == "" {
			e.LabelPos = "above"
			if e.Kind == "slots" || e.Kind == "cylinder" || e.Role == "consumer" {
				e.LabelPos = "below"
			}
		}
		switch e.Kind {
		case "block":
			e.width, e.height = blockSize, blockH
			e.mesh = cube(blockSize, blockH)
		case "slots":
			if e.Count == 0 {
				e.Count = 3
			}
			if e.Spacing == 0 {
				e.Spacing = 2.5
			}
			e.width, e.height = slotSize, slotH
			e.mesh = cube(slotSize, slotH)
		case "cylinder":
			e.width, e.height = 2*cylinderR, cylinderH
			e.mesh = cylinder(cylinderR, cylinderH)
		default:
			return fmt.Errorf("entity %s: unknown kind %q", e.ID, e.Kind)
		}
		e.positions = []point{{e.At[0], e.At[1], 0}}
		if e.Kind == "slots" {
			e.positions = make([]point, e.Count)
			for j := range e.positions {
				e.positions[j] = point{e.At[0] + (float64(j)-float64(e.Count-1)/2)*e.Spacing, e.At[1], 0}
			}
		}
		e.status, e.roles = resolve(e.Status), resolve(e.Roles)
		s.ids[e.ID] = len(s.entities)
		s.entities = append(s.entities, e)
		s.cues = append(s.cues, audio.Cue{T: e.appear + 0.18, Kind: audio.Rise, Gain: 1})
	}
	for i, source := range sc.Events {
		switch source.Do {
		case "send", "block", "state", "flash", "shot":
		default:
			if !style.Common(source.Do) {
				return fmt.Errorf("event %d: style maket does not know %q", i+1, source.Do)
			}
		}
		for _, pt := range []*scene.Point{source.From, source.To, source.Target} {
			if pt != nil && pt.ID == "" && len(pt.Coords) < 2 {
				return fmt.Errorf("event %d: point wants [a, b] or [a, b, z]", i+1)
			}
		}
		ev := event{Event: source, at: tl.MustResolve(source.At), until: math.Inf(1)}
		if source.Until != nil {
			ev.until = tl.MustResolve(*source.Until)
		}
		if ev.From != nil {
			ev.from = s.anchor(*ev.From)
		}
		if ev.To != nil {
			ev.to = s.anchor(*ev.To)
		}
		if ev.Target != nil {
			ev.target = s.anchor(*ev.Target)
		}
		if ev.Dur == 0 {
			if ev.Do == "send" {
				ev.Dur = sendDur
			} else if ev.Do == "shot" {
				ev.Dur = 0.6
			}
		}
		if ev.Do == "shot" && ev.Zoom == 0 {
			ev.Zoom = 1.3
		}
		s.events = append(s.events, ev)
	}
	sort.SliceStable(s.events, func(i, j int) bool { return s.events[i].at < s.events[j].at })
	tokens := map[string]int{}
	for i := range s.events {
		ev := &s.events[i]
		if ev.Do == "focus" && s.focus == nil {
			s.focus = ev
		}
		if ev.Do == "flash" {
			s.flashes = append(s.flashes, flash{ev.at, ev.Target.ID})
		}
		if ev.Do == "send" {
			idx, ok := tokens[ev.Token]
			if !ok {
				idx = len(s.tokens)
				tokens[ev.Token] = idx
				label := ev.Label
				if label == "" {
					label = ev.Token
				}
				s.tokens = append(s.tokens, motion{label: label})
			}
			s.tokens[idx].moves = append(s.tokens[idx].moves, *ev)
			if ev.Consume {
				s.flashes = append(s.flashes, flash{ev.at + ev.Dur, ev.To.ID})
			}
		}
		s.sound(*ev)
	}
	sort.SliceStable(s.cues, func(i, j int) bool { return s.cues[i].T < s.cues[j].T })
	s.token, s.waiting = cube(tokenSize, tokenSize), cube(0.9, 0.9)
	for i := -20; i < 32; i++ {
		s.grid = append(s.grid, [2]point{{float64(i), -20, 0}, {float64(i), 32, 0}}, [2]point{{-20, float64(i), 0}, {32, float64(i), 0}})
	}
	s.fit(sc.Code != nil)
	return nil
}

func (s *Style) anchor(p scene.Point) point {
	if p.ID == "" {
		v := point{x: p.Coords[0], y: p.Coords[1]}
		if len(p.Coords) == 3 {
			v.z = p.Coords[2]
		}
		return v
	}
	e := &s.entities[s.ids[p.ID]]
	if p.Slot >= 0 {
		return point{e.At[0] + (float64(p.Slot)-float64(e.Count-1)/2)*e.Spacing, e.At[1], slotH}
	}
	return point{e.At[0], e.At[1], e.height}
}

func (s *Style) fit(code bool) {
	xmin, ymin, xmax, ymax := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, e := range s.entities {
		h := e.height
		if e.Kind == "slots" {
			h += tokenSize
		}
		r := e.width / 2
		for _, p := range e.positions {
			for _, d := range []point{{x: -r}, {x: r}, {y: -r}, {y: r}} {
				x, y := 2*(p.x+d.x)*0.866, p.y+d.y
				xmin, xmax = math.Min(xmin, x), math.Max(xmax, x)
				ymin, ymax = math.Min(ymin, y-h), math.Max(ymax, y)
			}
		}
	}
	top, bottom := 322.0, 1278.0
	if code {
		bottom = 998
	}
	s.cx, s.cy = 505, (top+bottom)/2
	s.u = math.Min(math.Min(750/math.Max(1e-6, xmax-xmin), (bottom-top)/math.Max(1e-6, ymax-ymin)), 66) / (1 + (driftZoom+focusZoom)*0.6)
	mx, my := (xmin+xmax)/2, (ymin+ymax)/2
	fx, fy := 0.0, 0.0
	if s.focus != nil {
		p := s.anchor(s.focus.Targets[0])
		fx, fy = 2*p.x*0.866-mx, p.y-my
	}
	for _, ev := range s.events {
		if ev.Do != "shot" {
			continue
		}
		px, py := 2*ev.target.x*0.866-mx, ev.target.y-ev.target.z-my
		for _, axis := range []struct {
			low, high, focus, target, before, after float64
		}{
			{xmin - mx, xmax - mx, fx, px, s.cx - 70 - shakePX, 940 - s.cx - shakePX},
			{ymin - my, ymax - my, fy, py, s.cy - 230 - swayPX, bottom + 92 - s.cy - swayPX},
		} {
			for _, edge := range []float64{axis.low, axis.high} {
				for _, z := range []float64{1, 1 + driftZoom + focusZoom} {
					q, d := ev.Zoom-1, axis.target-axis.focus
					a := -z * q * d
					b := -axis.focus + z*(q*(edge-axis.focus)-d)
					c := axis.focus + z*(edge-axis.focus)
					ks := []float64{0, 1}
					if a != 0 {
						k := -b / (2 * a)
						if k > 0 && k < 1 {
							ks = append(ks, k)
						}
					}
					for _, k := range ks {
						distance := a*k*k + b*k + c
						if distance < 0 {
							s.u = math.Min(s.u, axis.before/-distance)
						} else if distance > 0 {
							s.u = math.Min(s.u, axis.after/distance)
						}
					}
				}
			}
		}
	}
	s.ox, s.oy = s.cx-mx*s.u, s.cy-my*s.u
	s.fx, s.fy = s.cx, s.cy
	if s.focus != nil {
		s.fx, s.fy = s.cx+fx*s.u, s.cy+fy*s.u
	}
}

func (s *Style) sound(ev event) {
	add := func(t float64, kind string, gain float64) {
		s.cues = append(s.cues, audio.Cue{T: t, Kind: kind, Gain: gain})
	}
	switch ev.Do {
	case "send":
		gain := 1.0
		if ev.From.ID == ev.To.ID {
			gain = 0.6
		}
		add(ev.at, audio.Whoosh, gain)
		if ev.Consume {
			add(ev.at+ev.Dur-0.02, audio.Pop, 1)
		} else if !ev.Waiting {
			add(ev.at+ev.Dur-0.01, audio.Land, 1)
		}
	case "block":
		add(ev.at-0.02, audio.Thud, 1)
		add(ev.at+0.1, audio.Buzz, 1)
		add(ev.at+0.4, audio.Buzz, 0.8)
	case "flash":
		add(ev.at, audio.Pop, 1)
	case "focus":
		add(ev.at, audio.Chime, 1)
	case "shot":
		add(ev.at, audio.Whoosh, 1.4)
	}
}

func (s *Style) Sounds() []audio.Cue { return s.cues }

func (s *Style) Theme() render.Theme { return s.theme }
