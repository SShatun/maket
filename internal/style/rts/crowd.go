package rts

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"sort"

	"maket/internal/scene"
	spr "maket/internal/style/sprites"
)

type rawCrowd struct {
	Behavior string                `json:"behavior"`
	Kind     string                `json:"kind"`
	Count    int                   `json:"count"`
	From     json.RawMessage       `json:"from"`
	To       json.RawMessage       `json:"to"`
	At       []float64             `json:"at"`
	Around   string                `json:"around"`
	Speed    float64               `json:"speed"`
	Spread   float64               `json:"spread"`
	Depth    float64               `json:"depth"`
	Loop     bool                  `json:"loop"`
	Start    *scene.At             `json:"start"`
	State    string                `json:"state"`
	Cols     int                   `json:"cols"`
	Rows     int                   `json:"rows"`
	Step     []float64             `json:"step"`
	Radius   []float64             `json:"radius"`
	Face     string                `json:"face"`
	Lane     []float64             `json:"lane"`
	Role     string                `json:"role"`
	Badges   []scene.Timed[string] `json:"badges"`
	Part     *rawPart              `json:"part"`
}

type rawPart struct {
	Count  int                   `json:"count"`
	Role   string                `json:"role"`
	Badges []scene.Timed[string] `json:"badges"`
	State  string                `json:"state"`
}

type leg struct {
	t0, t1   float64
	from, to [2]float64
	state    string
}

type actor struct {
	id, kind, label string
	roles           []timedStr
	steps           []spr.Step
	marks           []timedStr
	status          []timedStr
	home            [2]float64
	legs            []leg
	period, shift   float64
	cycle           []leg
	halt, resume    float64
	anim            float64
	appear, gone    float64
	flip            bool
	unit            bool
	jitter          bool
	salt            uint64
}

type crowd struct {
	id       string
	title    string
	behavior string
	kind     string
	units    []*actor
	part     []*actor
	status   []timedStr
	sel      bool
	lanes    [][]*actor
	slot     func(c, i int) [2]float64
	speed    float64
	stepY    float64
}

type spot struct {
	edge string
	at   *actor
	xy   [2]float64
}

func (a *actor) name() string {
	if a.id != "" {
		return "cast " + a.id
	}
	return "crowd of " + a.kind
}

func (a *actor) title() string {
	if a.label != "" {
		return a.label
	}
	return a.id
}

func (a *actor) states() []string {
	var out []string
	for _, st := range a.steps {
		out = append(out, st.V)
	}
	for _, l := range append(append([]leg(nil), a.legs...), a.cycle...) {
		if l.state != "" {
			out = append(out, l.state)
		}
	}
	return out
}

func (a *actor) rest(t float64) (float64, float64) {
	x, y, _, _, _, _ := a.pose(t)
	return x, y
}

func lerp(p, q [2]float64, k float64) (float64, float64) {
	return p[0] + (q[0]-p[0])*k, p[1] + (q[1]-p[1])*k
}

func (a *actor) pose(t float64) (x, y float64, state string, since float64, flip, moving bool) {
	x, y, flip = a.home[0], a.home[1], a.flip
	if a.period > 0 {
		tt, frozen := t, false
		if t >= a.halt {
			if t < a.resume {
				tt, frozen = a.halt, true
			} else {
				tt = t - (a.resume - a.halt)
			}
		}
		u := math.Mod(tt-a.appear+a.shift, a.period)
		if u < 0 {
			u += a.period
		}
		for _, l := range a.cycle {
			if u >= l.t0 && u < l.t1 {
				x, y = lerp(l.from, l.to, (u-l.t0)/(l.t1-l.t0))
				if l.to[0] != l.from[0] {
					flip = l.to[0] < l.from[0]
				}
				if frozen {
					return x, y, "", 0, flip, false
				}
				return x, y, l.state, t - (u - l.t0) - a.anim, flip, true
			}
		}
		return x, y, "", 0, flip, false
	}
	for _, l := range a.legs {
		if t < l.t0 {
			break
		}
		if l.to[0] != l.from[0] {
			flip = l.to[0] < l.from[0]
		}
		if t < l.t1 {
			x, y = lerp(l.from, l.to, (t-l.t0)/(l.t1-l.t0))
			return x, y, l.state, l.t0 - a.anim, flip, true
		}
		x, y = l.to[0], l.to[1]
	}
	return x, y, "", 0, flip, false
}

func (s *Style) spot(sh *shot, raw json.RawMessage, what string) (spot, error) {
	if len(raw) == 0 {
		return spot{}, fmt.Errorf("%s is required", what)
	}
	var str string
	if json.Unmarshal(raw, &str) == nil {
		switch str {
		case "top", "bottom", "left", "right":
			return spot{edge: str}, nil
		}
		a := sh.byID[str]
		if a == nil {
			return spot{}, fmt.Errorf("%s: unknown entity %q", what, str)
		}
		return spot{at: a, xy: a.home}, nil
	}
	var v []float64
	if json.Unmarshal(raw, &v) != nil || len(v) != 2 {
		return spot{}, fmt.Errorf("%s wants an entity id, an edge (top, bottom, left, right) or [x, y]", what)
	}
	return spot{xy: [2]float64{v[0], v[1]}}, nil
}

func (p spot) front() [2]float64 {
	if p.at != nil {
		return [2]float64{p.xy[0], p.xy[1] + 2}
	}
	return p.xy
}

func (s *Style) crowd(sh *shot, c scene.Cast, n int, rng *rand.Rand) (*crowd, error) {
	var rc rawCast
	if err := json.Unmarshal(c.Raw, &rc); err != nil {
		return nil, err
	}
	x := rc.Crowd
	if x == nil {
		x = &rawCrowd{}
	}
	kind := c.Kind
	if kind == "" {
		kind = x.Kind
	}
	spec, ok := s.set.Specs[kind]
	if !ok {
		return nil, fmt.Errorf("unknown kind %q", kind)
	}
	has := func(st string) bool { _, ok := spec.States[st]; return ok }
	role := c.Role
	if role == "" {
		role = x.Role
	}
	if role == "" {
		role = "external"
		if kind == "warrior" {
			role = "error"
		}
	}
	roles := []timedStr{{0, role}}
	for _, r := range c.Roles {
		t, err := s.when(r.At)
		if err != nil {
			return nil, err
		}
		roles = append(roles, timedStr{t, r.V})
	}
	badges := c.Badges
	if badges == nil {
		badges = x.Badges
	}
	marks, err := s.timed(badges)
	if err != nil {
		return nil, err
	}
	status, err := s.timed(c.Status)
	if err != nil {
		return nil, err
	}
	state := x.State
	if state == "" {
		state = c.State
	}
	start := sh.at
	if x.Start != nil {
		if start, err = s.when(*x.Start); err != nil {
			return nil, err
		}
	}
	speed := x.Speed
	title := c.Label
	if title == "" {
		title = c.ID
	}
	cr := &crowd{id: c.ID, title: title, behavior: x.Behavior, kind: kind, status: status, sel: rc.Select}
	unit := func() *actor {
		a := &actor{id: c.ID, kind: kind, roles: append([]timedStr(nil), roles...), marks: append([]timedStr(nil), marks...),
			appear: sh.at, gone: sh.until, unit: true, anim: rng.Float64(), salt: rng.Uint64(), halt: math.Inf(1), resume: math.Inf(1)}
		cr.units = append(cr.units, a)
		sh.actors = append(sh.actors, a)
		return a
	}
	idle := func(a *actor, t float64, st string) {
		if st == "" {
			st = "idle"
		}
		a.steps = []spr.Step{{T: t - a.anim, V: st}}
	}
	center := func() ([2]float64, error) {
		at := c.At
		if len(at) == 0 {
			at = x.At
		}
		if len(at) != 2 {
			return [2]float64{}, fmt.Errorf("%s needs at: [x, y] in map px", x.Behavior)
		}
		return [2]float64{at[0], at[1]}, nil
	}
	if kind == "warrior" {
		sh.horde = math.Min(sh.horde, start)
	}
	switch x.Behavior {
	case "route":
		from, err := s.spot(sh, x.From, "from")
		if err != nil {
			return nil, err
		}
		to, err := s.spot(sh, x.To, "to")
		if err != nil {
			return nil, err
		}
		if from.edge != "" || to.edge != "" {
			return nil, fmt.Errorf("route goes between two entities or points, not an edge")
		}
		if speed == 0 {
			speed = 0.35
		}
		spread := x.Spread
		if spread == 0 {
			spread = 3
		}
		count := x.Count
		if count == 0 {
			count = 6
		}
		pa, pb := from.front(), to.front()
		l := math.Max(1, math.Hypot(pb[0]-pa[0], pb[1]-pa[1]))
		nx, ny := -(pb[1]-pa[1])/l, (pb[0]-pa[0])/l
		back := "walk"
		if has("carry") {
			back = "carry"
		}
		p := 1 / speed
		for i := 0; i < count; i++ {
			u := unit()
			o := (rng.Float64()*2 - 1) * spread
			a := [2]float64{pa[0] + nx*o, pa[1] + ny*o}
			b := [2]float64{pb[0] + nx*o, pb[1] + ny*o}
			u.home, u.period = a, p
			u.shift = (float64(i) + rng.Float64()*0.3) / float64(count) * p
			u.cycle = []leg{{0, p / 2, a, b, "walk"}, {p / 2, p, b, a, back}}
		}
	case "stream":
		from, err := s.spot(sh, x.From, "from")
		if err != nil {
			return nil, err
		}
		to, err := s.spot(sh, x.To, "to")
		if err != nil {
			return nil, err
		}
		if speed == 0 {
			speed = 30
		}
		spread := x.Spread
		if spread == 0 {
			spread = 12
		}
		depth := x.Depth
		if depth == 0 {
			depth = 60
		}
		count := x.Count
		if count == 0 {
			count = 24
		}
		lane := x.Lane
		if len(lane) != 2 {
			lane = []float64{20, mapW - 20}
			if from.edge == "left" || from.edge == "right" {
				lane = []float64{20, mapH - 20}
			}
		}
		for i := 0; i < count; i++ {
			u := unit()
			v := speed * (1 + 0.2*(2*rng.Float64()-1))
			along := lane[0] + rng.Float64()*(lane[1]-lane[0])
			back := rng.Float64() * depth
			var a [2]float64
			delay := 0.0
			switch from.edge {
			case "bottom":
				a = [2]float64{along, mapH + 2 + back}
			case "top":
				a = [2]float64{along, -2 - back}
			case "left":
				a = [2]float64{-10 - back, along}
			case "right":
				a = [2]float64{mapW + 10 + back, along}
			default:
				p := from.front()
				a = [2]float64{p[0] + (rng.Float64()*2-1)*spread/3, p[1] + (rng.Float64()*2-1)*spread/5}
				delay = back / v
			}
			var b [2]float64
			vanish := true
			w := (rng.Float64()*2 - 1) * spread
			switch to.edge {
			case "top":
				b = [2]float64{a[0] + w, -20}
			case "bottom":
				b = [2]float64{a[0] + w, mapH + 20}
			case "left":
				b = [2]float64{-16, a[1] + w}
			case "right":
				b = [2]float64{mapW + 16, a[1] + w}
			default:
				vanish = false
				base := to.xy
				sign := 1.0
				if a[1] < base[1] {
					sign = -1
				}
				r := math.Sqrt(rng.Float64())
				ang := rng.Float64() * math.Pi
				b = [2]float64{base[0] + math.Cos(ang)*r*spread, base[1] + sign*(3+math.Sin(ang)*r*spread*0.6)}
			}
			d := math.Hypot(b[0]-a[0], b[1]-a[1]) / v
			u.home = a
			if x.Loop {
				u.appear = start
				u.period = d
				u.shift = rng.Float64() * d
				u.cycle = []leg{{0, d, a, b, "walk"}}
				continue
			}
			t0 := start + delay
			u.legs = []leg{{t0, t0 + d, a, b, "walk"}}
			if from.edge == "" {
				u.appear = t0
			}
			if vanish {
				u.gone = math.Min(u.gone, t0+d)
			} else {
				idle(u, t0+d, state)
				if kind == "warrior" {
					sh.horn = math.Min(sh.horn, t0+d)
				}
			}
		}
	case "formation":
		at, err := center()
		if err != nil {
			return nil, err
		}
		cols, rows := x.Cols, x.Rows
		if cols == 0 {
			cols = 6
		}
		count := x.Count
		if rows == 0 {
			rows = 5
			if count > 0 {
				rows = (count + cols - 1) / cols
			}
		}
		if count == 0 || count > rows*cols {
			count = rows * cols
		}
		step := [2]float64{11, 12}
		if len(x.Step) == 2 {
			step = [2]float64{x.Step[0], x.Step[1]}
		}
		if speed == 0 {
			speed = 30
		}
		top := true
		if sh.wall != nil && sh.wall.gate != nil && sh.wall.gate.home[1] > at[1] {
			top = false
		}
		half := 0.0
		if rows > 1 {
			half = step[0] / 4
		}
		cr.slot = func(c, i int) [2]float64 {
			r := i
			if !top {
				r = rows - 1 - i
			}
			return [2]float64{
				at[0] - float64(cols-1)*step[0]/2 - half + float64(c)*step[0] + float64(r%2)*step[0]/2,
				at[1] - float64(rows-1)*step[1]/2 + float64(r)*step[1],
			}
		}
		cr.lanes = make([][]*actor, cols)
		cr.speed, cr.stepY = speed, step[1]
		k := 0
		for i := 0; i < rows && k < count; i++ {
			for c := 0; c < cols && k < count; c++ {
				u := unit()
				u.home = cr.slot(c, i)
				u.flip = x.Face == "left"
				idle(u, sh.at, state)
				cr.lanes[c] = append(cr.lanes[c], u)
				k++
			}
		}
	case "swarm":
		var at [2]float64
		inner := 0.0
		if x.Around != "" {
			a := sh.byID[x.Around]
			if a == nil {
				return nil, fmt.Errorf("around: unknown entity %q", x.Around)
			}
			at, inner = a.home, 0.55
		} else if at, err = center(); err != nil {
			return nil, err
		}
		radius := [2]float64{30, 18}
		if len(x.Radius) == 2 {
			radius = [2]float64{x.Radius[0], x.Radius[1]}
		}
		count := x.Count
		if count == 0 {
			count = 24
		}
		st := state
		if st == "" && has("attack") {
			st = "attack"
		}
		for i := 0; i < count; i++ {
			u := unit()
			r := inner + (1-inner)*math.Sqrt(rng.Float64())
			ang := rng.Float64() * 2 * math.Pi
			u.home = [2]float64{at[0] + math.Cos(ang)*r*radius[0], at[1] + math.Sin(ang)*r*radius[1]}
			u.flip = rng.Float64() < 0.5
			u.jitter = true
			idle(u, start, st)
		}
	default:
		return nil, fmt.Errorf("behavior wants route, stream, formation or swarm, got %q", x.Behavior)
	}
	if p := x.Part; p != nil {
		if p.Count < 1 || p.Count > len(cr.units) {
			return nil, fmt.Errorf("part.count wants 1..%d", len(cr.units))
		}
		pm, err := s.timed(p.Badges)
		if err != nil {
			return nil, err
		}
		idx := make([]int, p.Count)
		for i := range idx {
			idx[i] = i
		}
		if x.Behavior != "formation" {
			idx = rng.Perm(len(cr.units))[:p.Count]
			sort.Ints(idx)
		}
		for _, i := range idx {
			u := cr.units[i]
			if p.Role != "" {
				u.roles = []timedStr{{0, p.Role}}
			}
			if p.Badges != nil {
				u.marks = append([]timedStr(nil), pm...)
			}
			if p.State != "" && len(u.steps) > 0 {
				u.steps[0].V = p.State
			}
			cr.part = append(cr.part, u)
		}
	}
	if err := s.crowdTag(sh, cr, c); err != nil {
		return nil, err
	}
	return cr, nil
}

func (cr *crowd) box() (x0, y0, x1, y1 float64) {
	x0, y0, x1, y1 = math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	add := func(p [2]float64) {
		if p[0] < 0 || p[0] > mapW || p[1] < 0 || p[1] > mapH {
			return
		}
		x0, y0, x1, y1 = math.Min(x0, p[0]), math.Min(y0, p[1]), math.Max(x1, p[0]), math.Max(y1, p[1])
	}
	for _, u := range cr.units {
		add(u.home)
		for _, l := range u.cycle {
			add(l.from)
			add(l.to)
		}
		for _, l := range u.legs {
			add(l.to)
		}
	}
	return
}

func (s *Style) admit(sh *shot, ev scene.Event, re rawEvent, t float64) error {
	gate := sh.byID[re.Gate]
	if gate == nil {
		return fmt.Errorf("admit needs gate: id of the lock entity in this shot")
	}
	spec := s.set.Specs[gate.kind]
	for _, st := range []string{"allow", "deny"} {
		if _, ok := spec.States[st]; !ok {
			return fmt.Errorf("gate %s: %s has no state %q", gate.id, gate.kind, st)
		}
	}
	from := ""
	if ev.From != nil {
		from = ev.From.ID
	}
	var cr *crowd
	for _, c := range sh.crowds {
		if c.behavior == "formation" && (from == "" || c.id == from) {
			cr = c
			break
		}
	}
	if cr == nil {
		if from != "" {
			return fmt.Errorf("crowd %q is not a formation of this shot", from)
		}
		return fmt.Errorf("admit needs a formation crowd in this shot")
	}
	rate := re.Rate
	if rate == 0 {
		rate = 2
	}
	if rate < 0 {
		return fmt.Errorf("rate wants units per second > 0")
	}
	until := sh.until
	if ev.Until != nil {
		u, err := s.when(*ev.Until)
		if err != nil {
			return err
		}
		until = math.Min(until, u)
	}
	door := gate.home
	if gate.kind == "lock" {
		door[0] -= 5
	}
	sign := 1.0
	if cr.slot(0, 0)[1] < door[1] {
		sign = -1
	}
	entry := [2]float64{door[0], door[1] + 3*sign}
	exit := [2]float64{door[0], door[1] - 22*sign}
	if ev.To != nil {
		switch {
		case ev.To.ID != "":
			a := sh.byID[ev.To.ID]
			if a == nil {
				return fmt.Errorf("to: unknown entity %q", ev.To.ID)
			}
			exit = [2]float64{a.home[0], a.home[1] + 2}
		case len(ev.To.Coords) == 2:
			exit = [2]float64{ev.To.Coords[0], ev.To.Coords[1]}
		}
	}
	cols := len(cr.lanes)
	order := make([]int, cols)
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool {
		return math.Abs(cr.slot(order[i], 0)[0]-door[0]) < math.Abs(cr.slot(order[j], 0)[0]-door[0])
	})
	free := make([]float64, cols)
	v := cr.speed
	shift := cr.stepY / v
	var passes []float64
	next := 0
	for k := 0; ; k++ {
		tk := t + float64(k)/rate
		if tk >= until {
			break
		}
		c := -1
		for j := 0; j < cols; j++ {
			if l := order[(next+j)%cols]; len(cr.lanes[l]) > 0 {
				c, next = l, (next+j+1)%cols
				break
			}
		}
		if c < 0 {
			break
		}
		dep := math.Max(tk, free[c])
		u := cr.lanes[c][0]
		cr.lanes[c] = cr.lanes[c][1:]
		from := cr.slot(c, 0)
		t1 := dep + math.Hypot(entry[0]-from[0], entry[1]-from[1])/v
		t2 := t1 + math.Hypot(exit[0]-entry[0], exit[1]-entry[1])/v
		u.legs = append(u.legs, leg{dep, t1, from, entry, "walk"}, leg{t1, t2, entry, exit, "walk"})
		u.gone = math.Min(u.gone, t2)
		passes = append(passes, t1-0.3)
		for i, w := range cr.lanes[c] {
			w.legs = append(w.legs, leg{dep, dep + shift, cr.slot(c, i+1), cr.slot(c, i), "walk"})
		}
		free[c] = dep + shift
	}
	sort.Float64s(passes)
	for i, p := range passes {
		gate.steps = append(gate.steps, spr.Step{T: p, V: "allow"})
		if i+1 == len(passes) || p+0.375 < passes[i+1] {
			gate.steps = append(gate.steps, spr.Step{T: p + 0.375, V: "deny"})
		}
	}
	sh.passes = append(sh.passes, passes...)
	return nil
}
