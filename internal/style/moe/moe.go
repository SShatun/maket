package moe

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"sort"

	"maket/internal/audio"
	"maket/internal/charkit"
	"maket/internal/render"
	"maket/internal/scene"
	"maket/internal/style/sprites"
	"maket/internal/timeline"
)

var roleColor = map[string]color.RGBA{
	"external": {255, 255, 255, 255},
	"producer": {0, 135, 81, 255},
	"consumer": {131, 118, 156, 255},
	"buffer":   {255, 163, 0, 255},
	"storage":  {41, 173, 255, 255},
	"error":    {255, 0, 77, 255},
	"ok":       {0, 228, 54, 255},
}

type light struct {
	Amb  [3]float64 `json:"amb"`
	AmbK float64    `json:"amb_k"`
	Rim  [4]float64 `json:"rim"`
	Dir  [2]float64 `json:"rim_dir"`
	Glow [4]float64 `json:"backglow"`
}

type backdrop struct {
	Light light `json:"light"`
	image *image.RGBA
}

type backgroundStep struct {
	at     float64
	image  *image.RGBA
	light  light
	bright float64
}

type shot struct {
	at, until   float64
	backgrounds []backgroundStep
	actors      []*actor
	panels      []*panel
	titles      []title
	shocks      []shock
	cuts        []cut
}

type timedString struct {
	at    float64
	value string
}

type actor struct {
	id, kind, motion string
	x, y, size       float64
	flip             bool
	states           []sprites.Step
	roles            []timedString
	visuals          []visual
}

type visual struct {
	at         float64
	body, glow *image.RGBA
}

type shock struct{ at, zoom, shake float64 }
type cut struct {
	at, dur float64
	white   bool
}

type Style struct {
	dir         string
	theme       render.Theme
	look        charkit.Look
	sprites     *sprites.Set
	roles       map[string]color.RGBA
	backgrounds map[string]backdrop
	shots       []shot
	cues        []audio.Cue
	sc          *scene.Scene
	tl          *timeline.Timeline
	shadow      *image.RGBA
}

func New(root string, fonts *render.Fonts) (*Style, error) {
	dir := filepath.Join(root, "assets", "looks", "moe")
	lk, err := charkit.ReadLook(dir)
	if err != nil {
		return nil, err
	}
	roles := make(map[string]color.RGBA, len(roleColor))
	for k, v := range roleColor {
		roles[k] = v
	}
	for k, v := range lk.Roles {
		roles[k] = v
	}
	sp, err := sprites.Load(dir, roles, 1)
	if err != nil {
		return nil, err
	}
	sp.HoldOneShots = true
	th := render.LightTheme(fonts)
	th.BG, th.Ink, th.Card = color.RGBA{14, 18, 36, 255}, color.RGBA{255, 255, 255, 255}, color.RGBA{14, 18, 36, 255}
	th.Accent, th.Error = color.RGBA{255, 236, 120, 255}, color.RGBA{255, 120, 150, 255}
	th.Caption = render.CaptionStyle{Y: 1760, Size: 50, Stroke: 6, Center: 540, Ink: th.Ink, Dim: color.RGBA{150, 150, 165, 255}, Edge: color.RGBA{10, 10, 20, 255}, Karaoke: true}
	th.OwnCaption, th.NoProgress = true, true
	return &Style{dir: dir, theme: th, look: lk, sprites: sp, roles: roles}, nil
}

func (s *Style) Theme() render.Theme { return s.theme }
func (s *Style) Sounds() []audio.Cue { return append([]audio.Cue(nil), s.cues...) }

func raw(data json.RawMessage, v any) error {
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, v)
}

func (s *Style) Layout(sc *scene.Scene, tl *timeline.Timeline) error {
	if len(sc.Shots) == 0 {
		return fmt.Errorf("moe: use shots with place, cast and do")
	}
	if sc.Guess != nil || sc.Answer != nil || sc.Code != nil {
		return fmt.Errorf("moe: use title and holo events instead of guess, answer or code")
	}
	s.sc, s.tl, s.shots, s.cues = sc, tl, nil, nil
	s.backgrounds = map[string]backdrop{}
	b, err := os.ReadFile(filepath.Join(s.dir, "backdrops", "backgrounds.json"))
	if err != nil {
		return err
	}
	var meta struct {
		Backgrounds map[string]backdrop `json:"backgrounds"`
	}
	if err := json.Unmarshal(b, &meta); err != nil {
		return err
	}
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
			return fmt.Errorf("moe shot %d: start must precede the end of the video", i+1)
		}
		sh, err := s.layoutShot(input, starts[i], end, meta.Backgrounds)
		if err != nil {
			return fmt.Errorf("moe shot %d: %w", i+1, err)
		}
		s.shots = append(s.shots, sh)
	}
	s.shadow = makeShadow()
	cache := map[litKey]visual{}
	for i := range s.shots {
		sh := &s.shots[i]
		for _, a := range sh.actors {
			if err := s.prepareActor(sh, a, cache); err != nil {
				return fmt.Errorf("moe shot %d, cast %s: %w", i+1, a.id, err)
			}
		}
	}
	sort.SliceStable(s.cues, func(i, j int) bool { return s.cues[i].T < s.cues[j].T })
	return nil
}

func (s *Style) loadBackdrop(name string, meta map[string]backdrop) (backdrop, error) {
	if b, ok := s.backgrounds[name]; ok {
		return b, nil
	}
	b, ok := meta[name]
	asset, exists := s.look.Backdrops[name]
	if !ok || !exists {
		return b, fmt.Errorf("unknown backdrop %q; use sunset, night, rain, sakura, flash, idea, alert or moe", name)
	}
	im, err := charkit.ReadImage(filepath.Join(s.dir, asset.File))
	if err != nil {
		return b, err
	}
	if im.Bounds().Dx() != render.W || im.Bounds().Dy() != render.H {
		return b, fmt.Errorf("backdrop %s must be 1080x1920", name)
	}
	b.image = rgba(im)
	s.backgrounds[name] = b
	return b, nil
}

func (s *Style) layoutShot(input scene.Shot, start, end float64, meta map[string]backdrop) (shot, error) {
	sh := shot{at: start, until: end}
	name := input.Place.Name
	if name == "" || input.Place.Wall.Name != "" || len(input.Place.Wall.RGB) > 0 || input.Place.Floor != nil || input.Place.Ground.Name != "" || len(input.Place.Ground.RGB) > 0 {
		return sh, fmt.Errorf("place must name a moe backdrop")
	}
	bg, err := s.loadBackdrop(name, meta)
	if err != nil {
		return sh, err
	}
	sh.backgrounds = []backgroundStep{{at: start, image: bg.image, light: bg.Light, bright: brightness(name)}}
	byID := map[string]*actor{}
	for _, c := range input.Cast {
		if c.Crowd != nil {
			return sh, fmt.Errorf("moe does not support crowd; list individual cast members")
		}
		spec, ok := s.sprites.Specs[c.Kind]
		if !ok {
			return sh, fmt.Errorf("cast %s: unknown kind %q", c.ID, c.Kind)
		}
		var extra struct {
			Motion string `json:"motion"`
			Flip   bool   `json:"flip"`
		}
		if err := raw(c.Raw, &extra); err != nil {
			return sh, err
		}
		if extra.Motion == "" {
			extra.Motion = "bob"
		}
		if extra.Motion != "bob" && extra.Motion != "jump" && extra.Motion != "none" {
			return sh, fmt.Errorf("cast %s: motion must be bob, jump or none", c.ID)
		}
		if len(c.Status) > 0 || len(c.Badges) > 0 || c.Label != "" {
			return sh, fmt.Errorf("cast %s: use holo for label, status and badges", c.ID)
		}
		size := c.Size
		if size == 0 {
			size = 1
		}
		role := c.Role
		if role == "" {
			role = "external"
		}
		state := c.State
		if state == "" {
			state = spec.Rest
		}
		a := &actor{id: c.ID, kind: c.Kind, motion: extra.Motion, flip: extra.Flip, x: c.At[0], y: c.At[1], size: size, states: []sprites.Step{{T: start, V: state}}, roles: []timedString{{start, role}}}
		for _, v := range c.States {
			t, err := s.resolve(v.At, start, end)
			if err != nil {
				return sh, fmt.Errorf("cast %s state: %w", c.ID, err)
			}
			a.states = append(a.states, sprites.Step{T: t, V: v.V, Hold: v.Hold})
		}
		for _, v := range c.Roles {
			t, err := s.resolve(v.At, start, end)
			if err != nil {
				return sh, fmt.Errorf("cast %s role: %w", c.ID, err)
			}
			a.roles = append(a.roles, timedString{t, v.V})
		}
		byID[a.id] = a
		sh.actors = append(sh.actors, a)
	}
	sort.SliceStable(sh.actors, func(i, j int) bool { return sh.actors[i].y < sh.actors[j].y })
	events := append([]scene.Event(nil), input.Do...)
	sort.SliceStable(events, func(i, j int) bool { return s.tl.MustResolve(events[i].At) < s.tl.MustResolve(events[j].At) })
	panels := map[string]*panel{}
	padAt, padName := start, name
	for i, ev := range events {
		t, err := s.resolve(ev.At, start, end)
		if err != nil {
			return sh, fmt.Errorf("event %d: %w", i+1, err)
		}
		until := end
		if ev.Until != nil {
			until, err = s.tl.Resolve(*ev.Until)
			if err != nil {
				return sh, err
			}
			if until <= t {
				return sh, fmt.Errorf("event %d: until must follow at", i+1)
			}
			until = math.Min(until, end)
		}
		var x struct {
			Name  string   `json:"name"`
			Color string   `json:"color"`
			Shake *float64 `json:"shake"`
		}
		if err := raw(ev.Raw, &x); err != nil {
			return sh, err
		}
		switch ev.Do {
		case "bg":
			next, err := s.loadBackdrop(x.Name, meta)
			if err != nil {
				return sh, err
			}
			dur := ev.Dur
			if dur == 0 {
				dur = 0.4
			}
			if dur < 0 {
				return sh, fmt.Errorf("bg dur must be positive")
			}
			prev := backgroundAt(&sh, t)
			keep := sort.Search(len(sh.backgrounds), func(i int) bool { return sh.backgrounds[i].at >= t })
			sh.backgrounds = sh.backgrounds[:keep]
			steps := max(1, int(math.Ceil(dur*render.FPS)))
			for j := 0; j <= steps; j++ {
				k := float64(j) / float64(steps)
				if t+k*dur >= end {
					break
				}
				sh.backgrounds = append(sh.backgrounds, backgroundStep{at: t + k*dur, image: mixImage(prev.image, next.image, k), light: mixLight(prev.light, next.Light, k), bright: lerp(prev.bright, brightness(x.Name), k)})
			}
			s.pad(padAt, t, padName)
			padAt, padName = t, x.Name
		case "state":
			if ev.Target == nil || byID[ev.Target.ID] == nil {
				return sh, fmt.Errorf("%s needs a cast target", ev.Do)
			}
			a := byID[ev.Target.ID]
			a.states = append(a.states, sprites.Step{T: t, V: ev.State, Hold: ev.Hold})
		case "holo":
			p, err := s.makePanel(ev, t, until, panels)
			if err != nil {
				return sh, fmt.Errorf("event %d: %w", i+1, err)
			}
			sh.panels = append(sh.panels, p)
			s.cue(t, audio.Blip, 0.6, 0)
		case "title":
			v, err := s.makeTitle(ev, t, until)
			if err != nil {
				return sh, err
			}
			sh.titles = append(sh.titles, v)
			s.cue(t, audio.MusicBox, 0.6, 0)
		case "shock":
			z, shake := ev.Zoom, 20.0
			if z == 0 {
				z = 1.14
			}
			if x.Shake != nil {
				shake = *x.Shake
			}
			if z < 1 || z > 2 || shake < 0 || shake > 100 {
				return sh, fmt.Errorf("shock needs zoom in 1..2 and shake in 0..100")
			}
			sh.shocks = append(sh.shocks, shock{t, z, shake})
			s.cue(t, audio.Stab, 1, 0)
			s.cue(t, audio.Thud, 0.6, 0)
		case "cut":
			dur := ev.Dur
			if dur == 0 {
				dur = 0.3
			}
			if dur <= 0 || dur > 2 || x.Color != "" && x.Color != "black" && x.Color != "white" {
				return sh, fmt.Errorf("cut needs color black or white and dur in (0, 2]")
			}
			sh.cuts = append(sh.cuts, cut{at: t, dur: dur, white: x.Color == "white"})
		case "sfx":
			if !soundNames[x.Name] {
				return sh, fmt.Errorf("unknown moe sfx %q", x.Name)
			}
			if ev.Dur < 0 {
				return sh, fmt.Errorf("sfx needs nonnegative dur")
			}
			dur := ev.Dur
			if dur == 0 {
				dur = end - t
			}
			s.cue(t, x.Name, 1, math.Min(dur, end-t))
		default:
			return sh, fmt.Errorf("event %d: moe does not support %q", i+1, ev.Do)
		}
	}
	s.pad(padAt, end, padName)
	return sh, nil
}

func (s *Style) resolve(a scene.At, start, end float64) (float64, error) {
	t, err := s.tl.Resolve(a)
	if err != nil {
		return 0, err
	}
	if t < start || t >= end {
		return 0, fmt.Errorf("time %.3f outside shot [%.3f, %.3f)", t, start, end)
	}
	return t, nil
}

func roleAt(a *actor, t float64) string {
	v := a.roles[0].value
	for _, r := range a.roles {
		if r.at > t {
			break
		}
		v = r.value
	}
	return v
}

func backgroundAt(sh *shot, t float64) backgroundStep {
	i := sort.Search(len(sh.backgrounds), func(i int) bool { return sh.backgrounds[i].at > t }) - 1
	return sh.backgrounds[max(0, i)]
}

func brightness(name string) float64 {
	switch name {
	case "sakura", "moe", "idea", "flash":
		return 1
	}
	return 0
}

func clamp(v float64) float64      { return math.Max(0, math.Min(1, v)) }
func lerp(a, b, k float64) float64 { return a + (b-a)*k }
