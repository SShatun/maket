package behavior

import (
	"encoding/json"
	"fmt"
	"maket/internal/scene"
	spr "maket/internal/style/sprites"
	"math"
	"math/rand"
	"sort"
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

type rawCast struct {
	Select bool      `json:"select"`
	Crowd  *rawCrowd `json:"crowd"`
}

func (s *Stage) AddCrowd(c scene.Cast, rng *rand.Rand) (*Crowd, error) {
	var rc rawCast
	if err := json.Unmarshal(c.Raw, &rc); err != nil {
		return nil, err
	}
	x := rc.Crowd
	if x == nil {
		return nil, fmt.Errorf("cast %s: crowd behavior is required", c.ID)
	}
	if x.Count < 0 || x.Cols < 0 || x.Rows < 0 || x.Speed < 0 || x.Spread < 0 || x.Depth < 0 {
		return nil, fmt.Errorf("crowd counts, speed, spread and depth must not be negative")
	}
	for name, values := range map[string][]float64{"step": x.Step, "radius": x.Radius, "lane": x.Lane} {
		if values != nil && len(values) != 2 {
			return nil, fmt.Errorf("crowd %s wants two coordinates", name)
		}
		if name != "lane" && len(values) == 2 && (values[0] <= 0 || values[1] <= 0) {
			return nil, fmt.Errorf("crowd %s must be positive", name)
		}
	}
	kind := c.Kind
	if kind == "" {
		kind = x.Kind
	}
	spec, ok := s.Specs[kind]
	if !ok {
		return nil, fmt.Errorf("unknown kind %q", kind)
	}
	has := func(st string) bool { _, ok := spec.States[st]; return ok }
	role := c.Role
	if role == "" {
		role = x.Role
	}
	if role == "" {
		role = s.DefaultRoles[kind]
	}
	if role == "" {
		role = "external"
	}
	roles := []Timed{{V: role}}
	for _, r := range c.Roles {
		t, err := s.Resolve(r.At)
		if err != nil {
			return nil, err
		}
		roles = append(roles, Timed{T: t, V: r.V})
	}
	badges := c.Badges
	if badges == nil {
		badges = x.Badges
	}
	marks := []Timed{}
	for _, b := range badges {
		t, err := s.Resolve(b.At)
		if err != nil {
			return nil, err
		}
		marks = append(marks, Timed{T: t, V: badgeName(b.V)})
	}
	status := []Timed{}
	for _, v := range c.Status {
		t, err := s.Resolve(v.At)
		if err != nil {
			return nil, err
		}
		status = append(status, Timed{T: t, V: v.V})
	}
	state := x.State
	if state == "" {
		state = c.State
	}
	start := s.At
	var err error
	if x.Start != nil {
		start, err = s.Resolve(*x.Start)
		if err != nil {
			return nil, err
		}
	}
	speed := x.Speed
	title := c.Label
	if title == "" {
		title = c.ID
	}
	sort.SliceStable(marks, func(i, j int) bool { return marks[i].T < marks[j].T })
	sort.SliceStable(status, func(i, j int) bool { return status[i].T < status[j].T })
	cr := &Crowd{Start: start, Arrival: math.Inf(1), ID: c.ID, Title: title, Behavior: x.Behavior, Kind: kind, Status: status, Selected: rc.Select}
	size := c.Size
	if size == 0 {
		size = 1
	}
	unit := func() *Actor {
		a := &Actor{ID: c.ID, Kind: kind, Roles: append([]Timed(nil), roles...), Badges: append([]Timed(nil), marks...), Appear: s.At, Gone: s.Until, Unit: true, Anim: rng.Float64(), Salt: rng.Uint64(), Halt: math.Inf(1), Resume: math.Inf(1), Size: size}
		cr.Units = append(cr.Units, a)
		s.Actors = append(s.Actors, a)
		return a
	}
	idle := func(a *Actor, t float64, st string) {
		if st == "" {
			st = spec.Rest
		}
		a.Steps = []spr.Step{{T: t - a.Anim, V: st}}
	}
	center := func() ([2]float64, error) {
		at := c.At
		if len(at) == 0 {
			at = x.At
		}
		if len(at) != 2 {
			return [2]float64{}, fmt.Errorf("%s needs at: [x, y]", x.Behavior)
		}
		return [2]float64{at[0], at[1]}, nil
	}
	switch x.Behavior {
	case "route":
		from, e := s.spot(x.From, "from")
		if e != nil {
			return nil, e
		}
		to, e := s.spot(x.To, "to")
		if e != nil {
			return nil, e
		}
		if from.edge != "" || to.edge != "" {
			return nil, fmt.Errorf("route goes between two entities or points, not an edge")
		}
		if speed == 0 {
			speed = .35
		}
		spread := x.Spread
		if spread == 0 {
			spread = 3 * s.Unit
		}
		count := x.Count
		if count == 0 {
			count = 6
		}
		pa, pb := from.front(s.Unit), to.front(s.Unit)
		l := math.Max(1, math.Hypot(pb[0]-pa[0], pb[1]-pa[1]))
		nx, ny := -(pb[1]-pa[1])/l, (pb[0]-pa[0])/l
		walk := s.walkState(kind)
		back := walk
		if has("carry") {
			back = "carry"
		}
		period := 1 / speed
		for i := 0; i < count; i++ {
			u := unit()
			o := (rng.Float64()*2 - 1) * spread
			a := [2]float64{pa[0] + nx*o, pa[1] + ny*o}
			b := [2]float64{pb[0] + nx*o, pb[1] + ny*o}
			u.Home, u.Period = a, period
			u.Shift = (float64(i) + rng.Float64()*.3) / float64(count) * period
			u.Cycle = []Leg{{0, period / 2, a, b, walk}, {period / 2, period, b, a, back}}
		}
	case "stream":
		walk := s.walkState(kind)
		from, e := s.spot(x.From, "from")
		if e != nil {
			return nil, e
		}
		to, e := s.spot(x.To, "to")
		if e != nil {
			return nil, e
		}
		if speed == 0 {
			speed = 30 * s.Unit
		}
		spread := x.Spread
		if spread == 0 {
			spread = 12 * s.Unit
		}
		depth := x.Depth
		if depth == 0 {
			depth = 60 * s.Unit
		}
		count := x.Count
		if count == 0 {
			count = 24
		}
		lane := x.Lane
		if len(lane) != 2 {
			lane = []float64{20 * s.Unit, s.Width - 20*s.Unit}
			if from.edge == "left" || from.edge == "right" {
				lane = []float64{20 * s.Unit, s.Height - 20*s.Unit}
			}
		}
		for i := 0; i < count; i++ {
			u := unit()
			v := speed * (1 + .2*(2*rng.Float64()-1))
			along := lane[0] + rng.Float64()*(lane[1]-lane[0])
			back := rng.Float64() * depth
			var a [2]float64
			delay := 0.0
			switch from.edge {
			case "bottom":
				a = [2]float64{along, s.Height + 2*s.Unit + back}
			case "top":
				a = [2]float64{along, -2*s.Unit - back}
			case "left":
				a = [2]float64{-10*s.Unit - back, along}
			case "right":
				a = [2]float64{s.Width + 10*s.Unit + back, along}
			default:
				p := from.front(s.Unit)
				a = [2]float64{p[0] + (rng.Float64()*2-1)*spread/3, p[1] + (rng.Float64()*2-1)*spread/5}
				delay = back / v
			}
			var b [2]float64
			vanish := true
			w := (rng.Float64()*2 - 1) * spread
			switch to.edge {
			case "top":
				b = [2]float64{a[0] + w, -20 * s.Unit}
			case "bottom":
				b = [2]float64{a[0] + w, s.Height + 20*s.Unit}
			case "left":
				b = [2]float64{-16 * s.Unit, a[1] + w}
			case "right":
				b = [2]float64{s.Width + 16*s.Unit, a[1] + w}
			default:
				vanish = false
				base := to.xy
				sign := 1.0
				if a[1] < base[1] {
					sign = -1
				}
				r := math.Sqrt(rng.Float64())
				ang := rng.Float64() * math.Pi
				b = [2]float64{base[0] + math.Cos(ang)*r*spread, base[1] + sign*(3*s.Unit+math.Sin(ang)*r*spread*.6)}
			}
			d := math.Hypot(b[0]-a[0], b[1]-a[1]) / v
			u.Home = a
			if x.Loop {
				u.Appear = start
				u.Period = d
				u.Shift = rng.Float64() * d
				u.Cycle = []Leg{{0, d, a, b, walk}}
				continue
			}
			t0 := start + delay
			u.Legs = []Leg{{t0, t0 + d, a, b, walk}}
			if from.edge == "" {
				u.Appear = t0
			}
			if vanish {
				u.Gone = math.Min(u.Gone, t0+d)
			} else {
				idle(u, t0+d, state)
				cr.Arrival = math.Min(cr.Arrival, t0+d)
			}
		}
	case "formation":
		at, e := center()
		if e != nil {
			return nil, e
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
		step := [2]float64{11 * s.Unit, 12 * s.Unit}
		if len(x.Step) == 2 {
			step = [2]float64{x.Step[0], x.Step[1]}
		}
		if speed == 0 {
			speed = 30 * s.Unit
		}
		top := true
		if cr.Behavior == "formation" && s.Gate != nil && s.Gate.Home[1] > at[1] {
			top = false
		}
		half := 0.0
		if rows > 1 {
			half = step[0] / 4
		}
		cr.Slot = func(c, i int) [2]float64 {
			r := i
			if !top {
				r = rows - 1 - i
			}
			return [2]float64{at[0] - float64(cols-1)*step[0]/2 - half + float64(c)*step[0] + float64(r%2)*step[0]/2, at[1] - float64(rows-1)*step[1]/2 + float64(r)*step[1]}
		}
		cr.Lanes = make([][]*Actor, cols)
		cr.Speed, cr.StepY = speed, step[1]
		k := 0
		for i := 0; i < rows && k < count; i++ {
			for c := 0; c < cols && k < count; c++ {
				u := unit()
				u.Home = cr.Slot(c, i)
				u.Flip = x.Face == "left"
				idle(u, s.At, state)
				cr.Lanes[c] = append(cr.Lanes[c], u)
				k++
			}
		}
	case "swarm":
		var at [2]float64
		inner := 0.0
		if x.Around != "" {
			a := s.ByID[x.Around]
			if a == nil {
				return nil, fmt.Errorf("around: unknown entity %q", x.Around)
			}
			at, inner = a.Home, .55
		} else if at, err = center(); err != nil {
			return nil, err
		}
		radius := [2]float64{30 * s.Unit, 18 * s.Unit}
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
			u.Home = [2]float64{at[0] + math.Cos(ang)*r*radius[0], at[1] + math.Sin(ang)*r*radius[1]}
			u.Flip = rng.Float64() < .5
			u.Jitter = true
			idle(u, start, st)
		}
	default:
		return nil, fmt.Errorf("behavior wants route, stream, formation or swarm, got %q", x.Behavior)
	}
	if p := x.Part; p != nil {
		if p.Count < 1 || p.Count > len(cr.Units) {
			return nil, fmt.Errorf("part.count wants 1..%d", len(cr.Units))
		}
		pm := []Timed{}
		for _, b := range p.Badges {
			t, e := s.Resolve(b.At)
			if e != nil {
				return nil, e
			}
			pm = append(pm, Timed{T: t, V: badgeName(b.V)})
		}
		sort.SliceStable(pm, func(i, j int) bool { return pm[i].T < pm[j].T })
		idx := make([]int, p.Count)
		for i := range idx {
			idx[i] = i
		}
		if x.Behavior != "formation" {
			idx = rng.Perm(len(cr.Units))[:p.Count]
			sort.Ints(idx)
		}
		for _, i := range idx {
			u := cr.Units[i]
			if p.Role != "" {
				u.Roles = []Timed{{V: p.Role}}
			}
			if p.Badges != nil {
				u.Badges = append([]Timed(nil), pm...)
			}
			if p.State != "" && len(u.Steps) > 0 {
				u.Steps[0].V = p.State
			}
			cr.Part = append(cr.Part, u)
		}
	}
	s.Crowds = append(s.Crowds, cr)
	if cr.ID != "" {
		s.CrowdByID[cr.ID] = cr
	}
	return cr, nil
}

type spot struct {
	edge  string
	actor *Actor
	xy    [2]float64
}

func (p spot) front(unit float64) [2]float64 {
	if p.actor != nil {
		return [2]float64{p.xy[0], p.xy[1] + 2*unit}
	}
	return p.xy
}
func (s *Stage) spot(raw json.RawMessage, what string) (spot, error) {
	if len(raw) == 0 {
		return spot{}, fmt.Errorf("%s is required", what)
	}
	var str string
	if json.Unmarshal(raw, &str) == nil {
		switch str {
		case "top", "bottom", "left", "right":
			return spot{edge: str}, nil
		}
		a := s.ByID[str]
		if a == nil {
			return spot{}, fmt.Errorf("%s: unknown entity %q", what, str)
		}
		return spot{actor: a, xy: a.Home}, nil
	}
	var v []float64
	if json.Unmarshal(raw, &v) != nil || len(v) != 2 {
		return spot{}, fmt.Errorf("%s wants an entity id, an edge (top, bottom, left, right) or [x, y]", what)
	}
	return spot{xy: [2]float64{v[0], v[1]}}, nil
}

func (s *Stage) admit(ev scene.Event, t float64) error {
	var re struct {
		Gate string  `json:"gate"`
		Rate float64 `json:"rate"`
	}
	if err := json.Unmarshal(ev.Raw, &re); err != nil {
		return err
	}
	gate := s.ByID[re.Gate]
	if gate == nil {
		return fmt.Errorf("admit needs gate: id of the gate entity in this shot")
	}
	for _, st := range []string{"allow", "deny"} {
		if _, ok := s.Specs[gate.Kind].States[st]; !ok {
			return fmt.Errorf("gate %s: %s has no state %q", gate.ID, gate.Kind, st)
		}
	}
	from := ""
	if ev.From != nil {
		from = ev.From.ID
	}
	var cr *Crowd
	for _, c := range s.Crowds {
		if c.Behavior == "formation" && (from == "" || c.ID == from) {
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
	until, err := untilValue(s, ev.Until, s.Until)
	if err != nil {
		return err
	}
	door := gate.Home
	offset := s.GateOffsets[gate.Kind]
	door[0] += offset[0] * gate.Size
	door[1] += offset[1] * gate.Size
	sign := 1.0
	if cr.Slot(0, 0)[1] < door[1] {
		sign = -1
	}
	entry := [2]float64{door[0], door[1] + 3*s.Unit*sign}
	exit := [2]float64{door[0], door[1] - 22*s.Unit*sign}
	if ev.To != nil {
		switch {
		case ev.To.ID != "":
			a := s.ByID[ev.To.ID]
			if a == nil {
				return fmt.Errorf("to: unknown entity %q", ev.To.ID)
			}
			exit = [2]float64{a.Home[0], a.Home[1] + 2*s.Unit}
		case len(ev.To.Coords) == 2:
			exit = [2]float64{ev.To.Coords[0], ev.To.Coords[1]}
		}
	}
	cols := len(cr.Lanes)
	order := make([]int, cols)
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool {
		return math.Abs(cr.Slot(order[i], 0)[0]-door[0]) < math.Abs(cr.Slot(order[j], 0)[0]-door[0])
	})
	free := make([]float64, cols)
	v := cr.Speed
	shift := cr.StepY / v
	var passes []float64
	next := 0
	for k := 0; ; k++ {
		tk := t + float64(k)/rate
		if tk >= until {
			break
		}
		c := -1
		for j := 0; j < cols; j++ {
			if l := order[(next+j)%cols]; len(cr.Lanes[l]) > 0 {
				c, next = l, (next+j+1)%cols
				break
			}
		}
		if c < 0 {
			break
		}
		dep := math.Max(tk, free[c])
		u := cr.Lanes[c][0]
		cr.Lanes[c] = cr.Lanes[c][1:]
		from := cr.Slot(c, 0)
		t1 := dep + math.Hypot(entry[0]-from[0], entry[1]-from[1])/v
		t2 := t1 + math.Hypot(exit[0]-entry[0], exit[1]-entry[1])/v
		u.Legs = append(u.Legs, Leg{dep, t1, from, entry, s.walkState(u.Kind)}, Leg{t1, t2, entry, exit, s.walkState(u.Kind)})
		u.Gone = math.Min(u.Gone, t2)
		passes = append(passes, t1-.3)
		for i, w := range cr.Lanes[c] {
			w.Legs = append(w.Legs, Leg{dep, dep + shift, cr.Slot(c, i+1), cr.Slot(c, i), s.walkState(w.Kind)})
		}
		free[c] = dep + shift
	}
	sort.Float64s(passes)
	for i, p := range passes {
		gate.Steps = append(gate.Steps, spr.Step{T: p, V: "allow"})
		if i+1 == len(passes) || p+.375 < passes[i+1] {
			gate.Steps = append(gate.Steps, spr.Step{T: p + .375, V: "deny"})
		}
	}
	s.Passes = append(s.Passes, passes...)
	return nil
}
