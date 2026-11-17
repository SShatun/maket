package paper

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"sort"

	"golang.org/x/image/font/opentype"

	"maket/internal/audio"
	"maket/internal/charkit"
	"maket/internal/render"
	"maket/internal/scene"
	"maket/internal/style/pixel"
	"maket/internal/style/sprites"
	"maket/internal/timeline"
)

type backdrop struct {
	File   string  `json:"file"`
	StandY float64 `json:"stand_y"`
	Board  struct {
		Rect  [4]int   `json:"rect"`
		Chalk [3]uint8 `json:"chalk"`
	} `json:"board"`
	image image.Image
}

type background struct {
	at   float64
	name string
}

type transition struct {
	at, dur float64
	white   bool
}

type motion struct {
	at, dur  float64
	from, to [2]float64
}

type actor struct {
	id, kind string
	size     float64
	home     [2]float64
	flip     bool
	label    *image.RGBA
	states   []sprites.Step
	roles    []role
	moves    []motion
}

type role struct {
	at   float64
	name string
}

type lettering struct {
	at, until, cps float64
	lines          []string
	image          *image.RGBA
	pos            [2]float64
	sound          bool
}

type shot struct {
	at, until   float64
	backgrounds []background
	actors      []*actor
	byID        map[string]*actor
	chalk       []lettering
	titles      []lettering
	labels      []lettering
}

type Style struct {
	dir         string
	theme       render.Theme
	font        *opentype.Font
	set         *sprites.Set
	roles       map[string]color.RGBA
	backgrounds map[string]backdrop
	shots       []shot
	cuts        []transition
	cues        []audio.Cue
	sc          *scene.Scene
	tl          *timeline.Timeline
}

func New(root string, fonts *render.Fonts) (*Style, error) {
	dir := filepath.Join(root, "assets", "looks", "paper")
	roles, _, err := pixel.Colors(dir)
	if err != nil {
		return nil, err
	}
	set, err := sprites.Load(dir, roles, 1)
	if err != nil {
		return nil, err
	}
	ft, err := render.LoadFont(filepath.Join(root, "assets", "fonts", "Pangolin-Regular.ttf"))
	if err != nil {
		return nil, err
	}
	th := render.LightTheme(fonts)
	th.OwnCaption, th.NoProgress = true, true
	th.Caption = render.CaptionStyle{Y: 1760, Size: 50, Stroke: 6, Center: 540, Ink: color.RGBA{255, 255, 255, 255}, Dim: color.RGBA{170, 170, 180, 255}, Edge: color.RGBA{10, 10, 14, 255}, Karaoke: true}
	return &Style{dir: dir, theme: th, font: ft, set: set, roles: roles}, nil
}

func (s *Style) Theme() render.Theme { return s.theme }
func (s *Style) Sounds() []audio.Cue { return append([]audio.Cue(nil), s.cues...) }

func (s *Style) Layout(sc *scene.Scene, tl *timeline.Timeline) error {
	if len(sc.Shots) == 0 {
		return fmt.Errorf("paper: use shots with place, cast and do")
	}
	if sc.Guess != nil || sc.Answer != nil || sc.Code != nil {
		return fmt.Errorf("paper: use title, text and chalk inside shots")
	}
	s.sc, s.tl, s.shots, s.cuts, s.cues = sc, tl, nil, nil, nil
	b, err := os.ReadFile(filepath.Join(s.dir, "bg", "backgrounds.json"))
	if err != nil {
		return err
	}
	var meta struct {
		Backgrounds map[string]backdrop `json:"backgrounds"`
	}
	if err := json.Unmarshal(b, &meta); err != nil {
		return err
	}
	s.backgrounds = meta.Backgrounds
	starts, err := tl.ShotStarts(sc)
	if err != nil {
		return err
	}
	for i, input := range sc.Shots {
		end := tl.Duration
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		if starts[i] >= end {
			return fmt.Errorf("paper shot %d: start must precede the video end", i+1)
		}
		sh, err := s.layoutShot(input, starts[i], end, i)
		if err != nil {
			return fmt.Errorf("paper shot %d: %w", i+1, err)
		}
		s.shots = append(s.shots, sh)
	}
	sort.SliceStable(s.cues, func(i, j int) bool { return s.cues[i].T < s.cues[j].T })
	return nil
}

func (s *Style) loadBackground(name string) error {
	bg, ok := s.backgrounds[name]
	if !ok {
		return fmt.Errorf("unknown paper background %q", name)
	}
	if bg.image != nil {
		return nil
	}
	im, err := charkit.ReadImage(filepath.Join(s.dir, bg.File))
	if err != nil {
		return err
	}
	if im.Bounds().Dx() != render.W || im.Bounds().Dy() != render.H {
		return fmt.Errorf("background %s must be 1080x1920", name)
	}
	bg.image = im
	s.backgrounds[name] = bg
	return nil
}

func (s *Style) resolve(at scene.At, start, end float64) (float64, error) {
	t, err := s.tl.Resolve(at)
	if err != nil {
		return 0, err
	}
	if t < start || t >= end {
		return 0, fmt.Errorf("time %.3f outside shot [%.3f, %.3f)", t, start, end)
	}
	return t, nil
}

func (s *Style) layoutShot(input scene.Shot, start, end float64, index int) (shot, error) {
	sh := shot{at: start, until: end, byID: map[string]*actor{}}
	if input.Place.Name == "" || input.Place.Wall.Name != "" || len(input.Place.Wall.RGB) > 0 || input.Place.Floor != nil || input.Place.Ground.Name != "" || len(input.Place.Ground.RGB) > 0 {
		return sh, fmt.Errorf("place must name a paper background")
	}
	if err := s.loadBackground(input.Place.Name); err != nil {
		return sh, err
	}
	sh.backgrounds = []background{{start, input.Place.Name}}
	s.cues = append(s.cues, audio.Cue{T: start, Kind: audio.Strum, Gain: 1, Chord: index % 4})
	if index > 0 {
		s.cuts = append(s.cuts, transition{start, 0.3, false})
	}
	if input.Place.Name == "classroom" && index > 0 {
		s.cuts[len(s.cuts)-1] = transition{start, 0.15, true}
	}
	for _, c := range input.Cast {
		if c.Crowd != nil {
			return sh, fmt.Errorf("paper: list individual cast members without crowd")
		}
		spec, ok := s.set.Specs[c.Kind]
		if !ok {
			return sh, fmt.Errorf("cast %s: unknown object %q", c.ID, c.Kind)
		}
		if len(c.Badges) > 0 || len(c.Status) > 0 {
			return sh, fmt.Errorf("cast %s: use text; state badges are already drawn", c.ID)
		}
		var extra struct {
			Enter string `json:"enter"`
			Flip  bool   `json:"flip"`
		}
		if err := json.Unmarshal(c.Raw, &extra); err != nil {
			return sh, err
		}
		size := c.Size
		if size == 0 {
			size = 1.1
			if len(input.Cast) == 1 {
				size = 1.3
			}
		}
		a := &actor{id: c.ID, kind: c.Kind, size: size, home: [2]float64{c.At[0], c.At[1]}, flip: extra.Flip}
		state := c.State
		if state == "" {
			state = spec.Rest
		}
		a.states = []sprites.Step{{T: start, V: state}}
		r := c.Role
		if r == "" {
			r = spec.DefaultRole()
		}
		a.roles = []role{{start, r}}
		for _, v := range c.States {
			t, err := s.resolve(v.At, start, end)
			if err != nil {
				return sh, err
			}
			a.states = append(a.states, sprites.Step{T: t, V: v.V, Hold: v.Hold})
		}
		for _, v := range c.Roles {
			t, err := s.resolve(v.At, start, end)
			if err != nil {
				return sh, err
			}
			a.roles = append(a.roles, role{t, v.V})
		}
		if extra.Enter != "none" {
			from, err := s.edge(a, extra.Enter)
			if err != nil {
				return sh, err
			}
			a.moves = append(a.moves, motion{start, 0.8, from, a.home})
		}
		if c.Label != "" {
			im, err := s.textImage(c.Label, 40, s.theme.Ink)
			if err != nil {
				return sh, err
			}
			a.label = im
		}
		sh.actors = append(sh.actors, a)
		sh.byID[a.id] = a
	}
	events := append([]scene.Event(nil), input.Do...)
	sort.SliceStable(events, func(i, j int) bool { return s.tl.MustResolve(events[i].At) < s.tl.MustResolve(events[j].At) })
	for i, ev := range events {
		if err := s.event(&sh, ev); err != nil {
			return sh, fmt.Errorf("event %d (%s): %w", i+1, ev.Do, err)
		}
	}
	for _, a := range sh.actors {
		sort.SliceStable(a.states, func(i, j int) bool { return a.states[i].T < a.states[j].T })
		sort.SliceStable(a.roles, func(i, j int) bool { return a.roles[i].at < a.roles[j].at })
		spec := s.set.Specs[a.kind]
		states, roles := map[string]bool{}, map[string]bool{}
		for _, st := range a.states {
			v, ok := spec.States[st.V]
			if !ok || v.Frames < 1 || v.FPS <= 0 {
				return sh, fmt.Errorf("cast %s: %s has no valid state %q", a.id, a.kind, st.V)
			}
			states[st.V] = true
		}
		states[s.talkingState(a)] = true
		for _, r := range a.roles {
			if _, ok := s.roles[r.name]; !ok {
				return sh, fmt.Errorf("cast %s: unknown role %q", a.id, r.name)
			}
			roles[r.name] = true
		}
		if err := s.set.Prepare(a.kind, roles, states); err != nil {
			return sh, err
		}
	}
	for _, text := range sh.chalk {
		if !text.sound {
			continue
		}
		count := 0
		for _, line := range text.lines {
			count += len([]rune(line))
		}
		dur := math.Min(float64(count)/text.cps, text.until-text.at)
		if dur > 0 {
			s.cues = append(s.cues, audio.Cue{T: text.at, Kind: audio.Chalk, Gain: 1, Dur: dur})
		}
	}
	for i, p := range s.sc.Phrases[:min(len(s.sc.Phrases), len(s.tl.Phrases))] {
		if p.Speaker == "" {
			continue
		}
		pt := s.tl.Phrases[i]
		until := math.Min(pt.End+0.35, end)
		if i+1 < len(s.tl.Phrases) {
			until = math.Min(until, s.tl.Phrases[i+1].Start)
		}
		at := math.Max(pt.Start, start)
		if at >= until {
			continue
		}
		a := sh.byID[p.Speaker]
		if a == nil {
			return sh, fmt.Errorf("phrase p%d: speaker %q is not in this shot", i+1, p.Speaker)
		}
		pitch := 180.0
		if a.kind == "person" || a.kind == "baby" || s.set.Specs[a.kind].Group == "people" {
			pitch = 260
		}
		s.cues = append(s.cues, audio.Cue{T: at, Kind: audio.Babble, Gain: 1, Dur: until - at, Pitch: pitch})
	}
	return sh, nil
}

func (s *Style) talkingState(a *actor) string {
	if _, ok := s.set.Specs[a.kind].States["talk"]; ok {
		return "talk"
	}
	return "work"
}

func (s *Style) edge(a *actor, side string) ([2]float64, error) {
	if side == "" {
		side = "left"
		if a.home[0] > render.W/2 {
			side = "right"
		}
	}
	spec := s.set.Specs[a.kind]
	x := -float64(spec.Size[0]-spec.Anchor[0]) * a.size
	if side == "right" {
		x = render.W + float64(spec.Anchor[0])*a.size
	} else if side != "left" {
		return [2]float64{}, fmt.Errorf("enter side must be left, right or none")
	}
	return [2]float64{x, a.home[1]}, nil
}

func (sh *shot) backgroundAt(t float64) string {
	name := sh.backgrounds[0].name
	for _, bg := range sh.backgrounds {
		if bg.at <= t {
			name = bg.name
		}
	}
	return name
}

func (a *actor) position(t float64) (float64, float64) {
	x, y := a.home[0], a.home[1]
	for _, m := range a.moves {
		if t < m.at {
			break
		}
		k := math.Max(0, math.Min(1, (t-m.at)/m.dur))
		x, y = m.from[0]+(m.to[0]-m.from[0])*k, m.from[1]+(m.to[1]-m.from[1])*k
		if k < 1 && int(math.Floor((t-m.at)*12+1e-8))%2 == 1 {
			y -= 12
		}
	}
	return x, y
}
