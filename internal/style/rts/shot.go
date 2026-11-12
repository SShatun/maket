package rts

import (
	"encoding/json"
	"fmt"
	"image"
	"math"
	"math/rand"
	"sort"
	"strings"

	xdraw "golang.org/x/image/draw"

	"maket/internal/audio"
	"maket/internal/scene"
	spr "maket/internal/style/sprites"
)

type rawShot struct {
	Map *rawMap         `json:"map"`
	HUD *rawHUD         `json:"hud"`
	In  json.RawMessage `json:"in"`
}

type rawHUD struct {
	Res     []rawRes `json:"res"`
	Buttons []string `json:"buttons"`
	Pressed []string `json:"pressed"`
}

type rawRes struct {
	Key   string    `json:"key"`
	Text  string    `json:"text"`
	Icon  string    `json:"icon"`
	Alert bool      `json:"alert"`
	Count []float64 `json:"count"`
}

type rawCast struct {
	Flip   bool      `json:"flip"`
	Select bool      `json:"select"`
	HP     *float64  `json:"hp"`
	Crowd  *rawCrowd `json:"crowd"`
}

type rawEvent struct {
	Key     string    `json:"key"`
	Alert   bool      `json:"alert"`
	Count   []float64 `json:"count"`
	Name    *string   `json:"name"`
	Status  *string   `json:"status"`
	HP      *float64  `json:"hp"`
	Pressed *[]string `json:"pressed"`
	Buttons []string  `json:"buttons"`
	Gate    string    `json:"gate"`
	Rate    float64   `json:"rate"`
	Px      float64   `json:"px"`
	Shake   *float64  `json:"shake"`
	Icon    string    `json:"icon"`
	Badge   *string   `json:"badge"`
	Part    bool      `json:"part"`
}

type timedStr struct {
	t float64
	v string
}

type resStep struct {
	t     float64
	text  string
	icon  string
	alert bool
	count []float64
}

type fade struct {
	color string
	dur   float64
}

type message struct {
	t, until float64
	lines    []string
}

type shake struct {
	t, until float64
	px       int
}

type focus struct {
	t, until float64
	targets  map[*actor]bool
}

type pick struct {
	t       float64
	who     *actor
	group   map[*actor]bool
	crowd   *crowd
	part    bool
	src     []timedStr
	name    string
	status  string
	fixed   bool
	tone    string
	hp      float64
	pressed map[string]bool
	buttons []string
}

type shot struct {
	at, until float64
	seed      uint64
	in        fade
	bg        *image.RGBA
	wall      *wall
	actors    []*actor
	byID      map[string]*actor
	crowds    []*crowd
	crowdByID map[string]*crowd
	tags      []tag
	tallies   []tally
	elems     []element
	keys      []string
	picks     []pick
	msgs      []message
	shakes    []shake
	shocks    []shake
	focuses   []focus
	passes    []float64
	denies    []float64
	sfx       []audio.Cue
	horde     float64
	horn      float64
}

var defaultButtons = []string{"move", "stop", "wait", "retry", "cancel", "attack"}

var sfxKinds = map[string]bool{
	audio.Whoosh: true, audio.Land: true, audio.Rise: true, audio.Thud: true, audio.Buzz: true, audio.Pop: true, audio.Chime: true,
	audio.PadSunset: true, audio.PadNight: true, audio.PadIdea: true, audio.PadSakura: true, audio.MusicBox: true, audio.Meow: true,
	audio.Heartbeat: true, audio.Purr: true, audio.Stab: true, audio.Blip: true, audio.Sparkle: true, audio.Drone: true,
	audio.DroneLow: true, audio.Pluck: true, audio.Drums: true, audio.Horn: true, audio.Crackle: true, audio.Clunk: true,
}

func (s *Style) when(a scene.At) (float64, error) { return s.tl.Resolve(a) }

func (s *Style) buildShot(i int, sc scene.Shot, at, until float64, keys []string, sceneSeed int64) (*shot, error) {
	var rs rawShot
	if err := json.Unmarshal(sc.Raw, &rs); err != nil {
		return nil, err
	}
	seed := sceneSeed
	if rs.Map != nil && rs.Map.Seed != nil {
		seed = *rs.Map.Seed
	}
	sh := &shot{at: at, until: until, seed: uint64(seed), byID: map[string]*actor{}, crowdByID: map[string]*crowd{}, horde: math.Inf(1), horn: math.Inf(1)}
	in, err := parseFade(rs.In, i == 0)
	if err != nil {
		return nil, err
	}
	sh.in = in
	var crowds []scene.Cast
	for _, c := range sc.Cast {
		var rc rawCast
		if err := json.Unmarshal(c.Raw, &rc); err != nil {
			return nil, err
		}
		if c.Crowd != nil || rc.Crowd != nil {
			crowds = append(crowds, c)
			continue
		}
		a, err := s.entity(sh, c, rc)
		if err != nil {
			return nil, err
		}
		if rc.Select {
			hp := 1.0
			if rc.HP != nil {
				hp = *rc.HP
			}
			sh.picks = append(sh.picks, pick{t: at, who: a, src: a.status, name: a.title(), tone: "ok", hp: hp})
		}
	}
	rng := rand.New(rand.NewSource(seed))
	place := func(raw json.RawMessage) ([2]float64, error) {
		var id string
		if json.Unmarshal(raw, &id) == nil {
			a := sh.byID[id]
			if a == nil {
				return [2]float64{}, fmt.Errorf("unknown entity %q", id)
			}
			return [2]float64{a.home[0], a.home[1] + 2}, nil
		}
		var v []float64
		if json.Unmarshal(raw, &v) != nil || len(v) != 2 {
			return [2]float64{}, fmt.Errorf("point wants [x, y] in map px or an entity id: %s", raw)
		}
		return [2]float64{v[0], v[1]}, nil
	}
	var mp *rawMap = rs.Map
	lg, err := s.paint(mp, rng, place)
	if err != nil {
		return nil, err
	}
	if mp != nil {
		if sh.wall, err = s.wall(mp.Wall, sh.byID); err != nil {
			return nil, err
		}
	}
	for j, c := range crowds {
		cr, err := s.crowd(sh, c, j, rng)
		if err != nil {
			return nil, fmt.Errorf("crowd %d: %w", j+1, err)
		}
		sh.crowds = append(sh.crowds, cr)
		if cr.id != "" {
			sh.crowdByID[cr.id] = cr
		}
		if cr.sel {
			sh.picks = append(sh.picks, groupPick(pick{t: at}, cr, false, cr.units))
		}
	}
	sh.bg = image.NewRGBA(s.art.chrome.Bounds())
	copy(sh.bg.Pix, s.art.chrome.Pix)
	xdraw.NearestNeighbor.Scale(sh.bg, mapRect, lg, lg.Bounds(), xdraw.Src, nil)
	mm := minimap(lg)
	x0 := minimapIn.Min.X + (minimapIn.Dx()-mm.Bounds().Dx()*4)/2
	mr := image.Rect(x0, minimapIn.Min.Y, x0+mm.Bounds().Dx()*4, minimapIn.Max.Y)
	xdraw.NearestNeighbor.Scale(sh.bg, mr, mm, mm.Bounds(), xdraw.Src, nil)
	outline(sh.bg, mr, 4, white)

	buttons, pressed := defaultButtons, map[string]bool{}
	if h := rs.HUD; h != nil {
		if h.Buttons != nil {
			buttons = h.Buttons
		}
		for _, p := range h.Pressed {
			pressed[p] = true
		}
		if h.Res != nil {
			keys = nil
			for _, r := range h.Res {
				if err := s.addRes(sh, &keys, at, r.Key, r.Text, r.Icon, r.Alert, r.Count); err != nil {
					return nil, fmt.Errorf("hud.res: %w", err)
				}
			}
		}
	}
	if err := checkButtons(buttons, pressed); err != nil {
		return nil, fmt.Errorf("hud: %w", err)
	}
	sh.keys = append([]string(nil), keys...)
	base := pick{t: at, buttons: buttons, pressed: pressed}
	if len(sh.picks) > 0 {
		p := sh.picks[len(sh.picks)-1]
		p.buttons, p.pressed = buttons, pressed
		base = p
	}
	sh.picks = []pick{base}

	events := append([]scene.Event(nil), sc.Do...)
	times := make([]float64, len(events))
	for j, ev := range events {
		if times[j], err = s.when(ev.At); err != nil {
			return nil, err
		}
	}
	idx := make([]int, len(events))
	for j := range idx {
		idx[j] = j
	}
	sort.SliceStable(idx, func(a, b int) bool { return times[idx[a]] < times[idx[b]] })
	for _, j := range idx {
		if err := s.event(sh, events[j], times[j]); err != nil {
			return nil, fmt.Errorf("event %d (%s): %w", j+1, events[j].Do, err)
		}
	}
	for _, a := range sh.actors {
		sort.SliceStable(a.steps, func(i, j int) bool { return a.steps[i].T < a.steps[j].T })
		sort.SliceStable(a.roles, func(i, j int) bool { return a.roles[i].t < a.roles[j].t })
		sort.SliceStable(a.marks, func(i, j int) bool { return a.marks[i].t < a.marks[j].t })
		sort.SliceStable(a.legs, func(i, j int) bool { return a.legs[i].t0 < a.legs[j].t0 })
	}
	return sh, nil
}

func groupPick(p pick, cr *crowd, part bool, units []*actor) pick {
	p.who, p.crowd, p.part, p.src, p.name = units[0], cr, part, cr.status, cr.title
	if !part {
		in := map[*actor]bool{}
		for _, u := range cr.part {
			in[u] = true
		}
		for _, u := range units {
			if !in[u] {
				p.who = u
				break
			}
		}
	}
	p.group = map[*actor]bool{}
	for _, u := range units {
		p.group[u] = true
	}
	p.status, p.fixed, p.hp, p.tone = "", false, 1, "ok"
	return p
}

func (s *Style) targets(sh *shot, ev scene.Event, re rawEvent) ([]*actor, error) {
	if ev.Target == nil {
		return nil, fmt.Errorf("needs target: id of a cast entity or crowd in this shot")
	}
	id := ev.Target.ID
	if a := sh.byID[id]; a != nil {
		if re.Part {
			return nil, fmt.Errorf("part is for crowds, %s is one entity", id)
		}
		return []*actor{a}, nil
	}
	cr := sh.crowdByID[id]
	if cr == nil {
		return nil, fmt.Errorf("target %q is not a cast entity or crowd of this shot", id)
	}
	if !re.Part {
		return cr.units, nil
	}
	if len(cr.part) == 0 {
		return nil, fmt.Errorf("crowd %s has no part", id)
	}
	return cr.part, nil
}

func (a *actor) react(t float64) float64 {
	if a.unit {
		return t + 0.3*a.anim
	}
	return t
}

func parseFade(raw json.RawMessage, first bool) (fade, error) {
	f := fade{color: "black", dur: 0.2}
	if first {
		f.color = "none"
	}
	if len(raw) == 0 {
		return f, nil
	}
	var o struct {
		Color string  `json:"color"`
		Dur   float64 `json:"dur"`
	}
	if json.Unmarshal(raw, &o.Color) != nil {
		if err := json.Unmarshal(raw, &o); err != nil {
			return f, fmt.Errorf("in wants \"black\", \"white\", \"none\" or {\"color\": ..., \"dur\": s}")
		}
	}
	switch o.Color {
	case "black":
		f = fade{"black", 0.2}
	case "white":
		f = fade{"white", 0.15}
	case "none":
		return fade{color: "none"}, nil
	default:
		return f, fmt.Errorf("in: unknown colour %q, want black, white or none", o.Color)
	}
	if o.Dur > 0 {
		f.dur = o.Dur
	}
	return f, nil
}

func checkButtons(buttons []string, pressed map[string]bool) error {
	if len(buttons) > 6 {
		return fmt.Errorf("at most 6 buttons, got %d", len(buttons))
	}
	known := map[string]bool{}
	for _, c := range commands {
		known[c] = true
	}
	for _, b := range buttons {
		if !known[b] {
			return fmt.Errorf("unknown button %q, known: %s", b, strings.Join(commands, ", "))
		}
	}
	for b := range pressed {
		if !known[b] {
			return fmt.Errorf("unknown pressed button %q, known: %s", b, strings.Join(commands, ", "))
		}
	}
	return nil
}

func (s *Style) addRes(sh *shot, keys *[]string, t float64, key, text, icon string, alert bool, count []float64) error {
	if icon == "" {
		if steps := s.res[key]; len(steps) > 0 {
			icon = steps[len(steps)-1].icon
		} else if s.art.res[key] != nil {
			icon = "res-" + key
		}
	}
	if key == "" || icon == "" {
		return fmt.Errorf("unknown resource %q, known: %s; any other key needs icon", key, strings.Join(resKeys, ", "))
	}
	if s.art.glyphs[icon] == nil {
		var names []string
		for k := range s.art.glyphs {
			names = append(names, k)
		}
		sort.Strings(names)
		return fmt.Errorf("res %s: unknown icon %q, known: %s", key, icon, strings.Join(names, ", "))
	}
	if count != nil && (len(count) != 3 || count[2] < 0) {
		return fmt.Errorf("res %s: count wants [from, to, seconds]", key)
	}
	if count != nil && !strings.Contains(text, "{n}") {
		text = "{n}" + text
	}
	found := false
	for _, k := range *keys {
		found = found || k == key
	}
	if !found {
		if len(*keys) >= 4 {
			return fmt.Errorf("at most 4 resources in the bar")
		}
		*keys = append(*keys, key)
	}
	s.res[key] = append(s.res[key], resStep{t: t, text: text, icon: icon, alert: alert, count: count})
	return nil
}

func (s *Style) entity(sh *shot, c scene.Cast, rc rawCast) (*actor, error) {
	spec, ok := s.set.Specs[c.Kind]
	if !ok {
		return nil, fmt.Errorf("cast %s: unknown kind %q", c.ID, c.Kind)
	}
	if c.At[0] < 0 || c.At[0] > mapW || c.At[1] < 0 || c.At[1] > mapH {
		return nil, fmt.Errorf("cast %s: at wants map px inside 180x229, got %v", c.ID, c.At)
	}
	a := &actor{id: c.ID, kind: c.Kind, label: c.Label, home: [2]float64{c.At[0], c.At[1]}, appear: sh.at, gone: sh.until, flip: rc.Flip}
	role := c.Role
	if role == "" {
		role = "external"
	}
	a.roles = []timedStr{{0, role}}
	for _, r := range c.Roles {
		t, err := s.when(r.At)
		if err != nil {
			return nil, err
		}
		a.roles = append(a.roles, timedStr{t, r.V})
	}
	state := c.State
	if state == "" {
		state = spec.Rest
	}
	a.steps = []spr.Step{{T: sh.at, V: state}}
	for _, st := range c.States {
		t, err := s.when(st.At)
		if err != nil {
			return nil, err
		}
		a.steps = append(a.steps, spr.Step{T: t, V: st.V, Hold: st.Hold})
	}
	var err error
	if a.marks, err = s.timed(c.Badges); err != nil {
		return nil, err
	}
	if a.status, err = s.timed(c.Status); err != nil {
		return nil, err
	}
	if err := s.entityTag(sh, a, c); err != nil {
		return nil, err
	}
	sh.actors = append(sh.actors, a)
	sh.byID[c.ID] = a
	return a, nil
}

func (s *Style) timed(l []scene.Timed[string]) ([]timedStr, error) {
	var out []timedStr
	for _, x := range l {
		t, err := s.when(x.At)
		if err != nil {
			return nil, err
		}
		v := x.V
		if v == "bad" {
			v = "bang"
		}
		out = append(out, timedStr{t, v})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].t < out[j].t })
	return out, nil
}

func (s *Style) event(sh *shot, ev scene.Event, t float64) error {
	var re rawEvent
	if err := json.Unmarshal(ev.Raw, &re); err != nil {
		return err
	}
	until := func(def float64) (float64, error) {
		if ev.Until == nil {
			return math.Min(def, sh.until), nil
		}
		u, err := s.when(*ev.Until)
		return math.Min(u, sh.until), err
	}
	target := func() (*actor, error) {
		if ev.Target == nil || sh.byID[ev.Target.ID] == nil {
			return nil, fmt.Errorf("needs target: id of a cast entity in this shot")
		}
		return sh.byID[ev.Target.ID], nil
	}
	switch ev.Do {
	case "state":
		us, err := s.targets(sh, ev, re)
		if err != nil {
			return err
		}
		moving := ev.State == "walk" || ev.State == "carry"
		for _, a := range us {
			at := a.react(t)
			if a.period > 0 {
				switch {
				case !moving && math.IsInf(a.halt, 1):
					a.halt = at
				case moving && !math.IsInf(a.halt, 1) && math.IsInf(a.resume, 1):
					a.resume = at
				}
			}
			a.steps = append(a.steps, spr.Step{T: at, V: ev.State, Hold: ev.Hold})
		}
		if ev.State == "deny" {
			sh.denies = append(sh.denies, t)
		}
	case "role":
		us, err := s.targets(sh, ev, re)
		if err != nil {
			return err
		}
		var u float64
		if ev.Until != nil {
			if u, err = s.when(*ev.Until); err != nil {
				return err
			}
		}
		for _, a := range us {
			a.roles = append(a.roles, timedStr{a.react(t), ev.Role})
			if ev.Until != nil {
				a.roles = append(a.roles, timedStr{a.react(u), a.roles[0].v})
			}
		}
	case "badge":
		us, err := s.targets(sh, ev, re)
		if err != nil {
			return err
		}
		v := ""
		if re.Badge != nil {
			v = *re.Badge
		}
		if v == "bad" {
			v = "bang"
		}
		var u float64
		if ev.Until != nil {
			if u, err = s.when(*ev.Until); err != nil {
				return err
			}
		}
		for _, a := range us {
			prev, _ := last(a.marks, t)
			a.marks = append(a.marks, timedStr{a.react(t), v})
			if ev.Until != nil {
				a.marks = append(a.marks, timedStr{a.react(u), prev})
			}
			sort.SliceStable(a.marks, func(i, j int) bool { return a.marks[i].t < a.marks[j].t })
		}
	case "tries":
		return s.tries(sh, ev, re, t)
	case "text", "panel", "line":
		u, err := until(sh.until)
		if err != nil {
			return err
		}
		return s.element(sh, ev, t, u)
	case "walk":
		a, err := target()
		if err != nil {
			return err
		}
		x, y := a.rest(t)
		st := ""
		if _, ok := s.set.Specs[a.kind].States["walk"]; ok {
			st = "walk"
		}
		a.legs = append(a.legs, leg{t0: t, t1: t + ev.Dur, from: [2]float64{x, y}, to: [2]float64{ev.To.Coords[0], ev.To.Coords[1]}, state: st})
	case "shake":
		p := int(math.Round(re.Px))
		if p == 0 {
			p = 1
		}
		if p < 1 || p > 2 {
			return fmt.Errorf("px wants 1 or 2 map px")
		}
		u, err := until(t + 0.6)
		if err != nil {
			return err
		}
		sh.shakes = append(sh.shakes, shake{t: t, until: u, px: p})
	case "shock":
		p := 2
		if re.Shake != nil {
			p = int(math.Round(*re.Shake))
		}
		sh.shocks = append(sh.shocks, shake{t: t, until: t + 0.7, px: p})
	case "message":
		var lines []string
		for _, l := range ev.Lines {
			lines = append(lines, l.Text)
		}
		if len(lines) == 0 && ev.Text != "" {
			lines = []string{ev.Text}
		}
		if len(lines) == 0 || len(lines) > 2 {
			return fmt.Errorf("message wants 1 or 2 lines")
		}
		u, err := until(t + 3)
		if err != nil {
			return err
		}
		sh.msgs = append(sh.msgs, message{t: t, until: u, lines: lines})
	case "res":
		return s.addRes(sh, &sh.keys, t, re.Key, ev.Text, re.Icon, re.Alert, re.Count)
	case "select":
		return s.selectEvent(sh, ev, re, t)
	case "admit":
		return s.admit(sh, ev, re, t)
	case "sfx":
		var o struct {
			Name string `json:"name"`
		}
		json.Unmarshal(ev.Raw, &o)
		if !sfxKinds[o.Name] {
			var names []string
			for k := range sfxKinds {
				names = append(names, k)
			}
			sort.Strings(names)
			return fmt.Errorf("unknown sound %q, known: %s", o.Name, strings.Join(names, ", "))
		}
		sh.sfx = append(sh.sfx, audio.Cue{T: t, Kind: o.Name, Gain: 1, Dur: ev.Dur})
	case "focus":
		f := focus{t: t, targets: map[*actor]bool{}}
		var err error
		if f.until, err = until(sh.until); err != nil {
			return err
		}
		pts := append([]scene.Point(nil), ev.Targets...)
		if ev.Target != nil {
			pts = append(pts, *ev.Target)
		}
		for _, p := range pts {
			if sh.byID[p.ID] == nil {
				return fmt.Errorf("focus target %q is not a cast entity", p.ID)
			}
			f.targets[sh.byID[p.ID]] = true
		}
		sh.focuses = append(sh.focuses, f)
	default:
		return fmt.Errorf("style rts does not know %q; known: state, role, badge, walk, tries, shake, shock, message, text, panel, line, res, select, admit, sfx, focus", ev.Do)
	}
	return nil
}

func (s *Style) selectEvent(sh *shot, ev scene.Event, re rawEvent, t float64) error {
	p := sh.picks[len(sh.picks)-1]
	p.t = t
	pressed := map[string]bool{}
	for k, v := range p.pressed {
		pressed[k] = v
	}
	p.pressed = pressed
	if ev.Target != nil {
		if a := sh.byID[ev.Target.ID]; a != nil {
			if re.Part {
				return fmt.Errorf("part is for crowds, %s is one entity", a.id)
			}
			if a != p.who || p.group != nil {
				p.who, p.group, p.crowd, p.part, p.src = a, nil, nil, false, a.status
				p.name, p.status, p.fixed, p.hp, p.tone = a.title(), "", false, 1, "ok"
			}
		} else if cr := sh.crowdByID[ev.Target.ID]; cr != nil {
			us, err := s.targets(sh, ev, re)
			if err != nil {
				return err
			}
			if cr != p.crowd || re.Part != p.part {
				p = groupPick(p, cr, re.Part, us)
			}
		} else {
			return fmt.Errorf("select target %q is not a cast entity or crowd", ev.Target.ID)
		}
	}
	if p.who == nil {
		return fmt.Errorf("select needs target first")
	}
	if re.Name != nil {
		p.name = *re.Name
	}
	if re.Status != nil {
		p.status, p.fixed = *re.Status, true
	}
	if ev.Tone != "" {
		if _, ok := toneInk[ev.Tone]; !ok {
			return fmt.Errorf("tone wants ok, warn or bad, got %q", ev.Tone)
		}
		p.tone = ev.Tone
	}
	if re.HP != nil {
		if *re.HP < 0 || *re.HP > 1 {
			return fmt.Errorf("hp wants 0..1")
		}
		p.hp = *re.HP
	}
	if re.Pressed != nil {
		p.pressed = map[string]bool{}
		for _, b := range *re.Pressed {
			p.pressed[b] = true
		}
	}
	if re.Buttons != nil {
		p.buttons = re.Buttons
	}
	if err := checkButtons(p.buttons, p.pressed); err != nil {
		return err
	}
	sh.picks = append(sh.picks, p)
	return nil
}
