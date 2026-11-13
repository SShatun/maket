package behavior

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"maket/internal/scene"
	spr "maket/internal/style/sprites"
)

type Config struct {
	Seed                           uint64
	At, Until, Width, Height, Unit float64
	Specs                          map[string]spr.Spec
	Resolve                        func(scene.At) (float64, error)
	DefaultRoles                   map[string]string
	GateOffsets                    map[string][2]float64
}

type Stage struct {
	Config
	Actors    []*Actor
	ByID      map[string]*Actor
	Crowds    []*Crowd
	CrowdByID map[string]*Crowd
	Gate      *Actor
	Passes    []float64
	Denies    []float64
	Tallies   []*Tally
}

func NewStage(c Config) *Stage {
	if c.Unit == 0 {
		c.Unit = 1
	}
	return &Stage{Config: c, ByID: map[string]*Actor{}, CrowdByID: map[string]*Crowd{}}
}

func (s *Stage) AddEntity(c scene.Cast) (*Actor, error) {
	if c.ID == "" {
		return nil, fmt.Errorf("entity needs id")
	}
	spec, ok := s.Specs[c.Kind]
	if !ok {
		return nil, fmt.Errorf("cast %s: unknown kind %q", c.ID, c.Kind)
	}
	if len(c.At) != 2 {
		return nil, fmt.Errorf("cast %s needs at: [x, y]", c.ID)
	}
	if s.ByID[c.ID] != nil {
		return nil, fmt.Errorf("duplicate id %q", c.ID)
	}
	role := c.Role
	if role == "" {
		role = s.DefaultRoles[c.Kind]
	}
	if role == "" {
		role = "external"
	}
	size := c.Size
	if size == 0 {
		size = 1
	}
	a := &Actor{ID: c.ID, Kind: c.Kind, Label: c.Label, Home: [2]float64{c.At[0], c.At[1]}, Appear: s.At, Gone: s.Until, Size: size, Roles: []Timed{{V: role}}}
	state := c.State
	if state == "" {
		state = spec.Rest
	}
	a.Steps = []spr.Step{{T: s.At, V: state}}
	if err := a.addTimed(&a.Roles, c.Roles, s.Resolve); err != nil {
		return nil, err
	}
	if err := a.addTimed(&a.Badges, c.Badges, s.Resolve); err != nil {
		return nil, err
	}
	for i := range a.Badges {
		a.Badges[i].V = badgeName(a.Badges[i].V)
	}
	if err := a.addTimed(&a.Status, c.Status, s.Resolve); err != nil {
		return nil, err
	}
	for _, st := range c.States {
		t, err := s.Resolve(st.At)
		if err != nil {
			return nil, err
		}
		a.Steps = append(a.Steps, spr.Step{T: t, V: st.V, Hold: st.Hold})
	}
	s.Actors = append(s.Actors, a)
	s.ByID[a.ID] = a
	return a, nil
}

func (s *Stage) Finish() {
	for _, a := range s.Actors {
		sort.SliceStable(a.Steps, func(i, j int) bool { return a.Steps[i].T < a.Steps[j].T })
		sort.SliceStable(a.Roles, func(i, j int) bool { return a.Roles[i].T < a.Roles[j].T })
		sort.SliceStable(a.Badges, func(i, j int) bool { return a.Badges[i].T < a.Badges[j].T })
		sort.SliceStable(a.Legs, func(i, j int) bool { return a.Legs[i].T0 < a.Legs[j].T0 })
	}
}

func (s *Stage) Event(ev scene.Event, t float64) error {
	target := func() (*Actor, error) {
		if ev.Target == nil || s.ByID[ev.Target.ID] == nil {
			return nil, fmt.Errorf("%s needs target: id of a cast entity in this shot", ev.Do)
		}
		return s.ByID[ev.Target.ID], nil
	}
	switch ev.Do {
	case "state":
		us, err := s.targets(ev)
		if err != nil {
			return err
		}
		moving := ev.State == "walk" || ev.State == "carry"
		for _, a := range us {
			at := a.React(t)
			if a.Period > 0 {
				switch {
				case !moving && math.IsInf(a.Halt, 1):
					a.Halt = at
				case moving && !math.IsInf(a.Halt, 1) && math.IsInf(a.Resume, 1):
					a.Resume = at
				}
			}
			state := ev.State
			if state == "walk" {
				state = s.walkState(a.Kind)
			}
			a.Steps = append(a.Steps, spr.Step{T: at, V: state, Hold: ev.Hold})
		}
		if ev.State == "deny" {
			s.Denies = append(s.Denies, t)
		}
	case "role":
		us, err := s.targets(ev)
		if err != nil {
			return err
		}
		var u float64
		if ev.Until != nil {
			u, err = s.Resolve(*ev.Until)
			if err != nil {
				return err
			}
		}
		for _, a := range us {
			a.Roles = append(a.Roles, Timed{a.React(t), ev.Role})
			if ev.Until != nil {
				a.Roles = append(a.Roles, Timed{a.React(u), a.Roles[0].V})
			}
		}
	case "badge":
		us, err := s.targets(ev)
		if err != nil {
			return err
		}
		var raw struct {
			Badge *string `json:"badge"`
			Part  bool    `json:"part"`
		}
		if err := json.Unmarshal(ev.Raw, &raw); err != nil {
			return err
		}
		v := ""
		if raw.Badge != nil {
			v = *raw.Badge
		}
		if v == "bad" {
			v = "bang"
		}
		var u float64
		if ev.Until != nil {
			u, err = s.Resolve(*ev.Until)
			if err != nil {
				return err
			}
		}
		for _, a := range us {
			prev, _ := Last(a.Badges, t)
			a.Badges = append(a.Badges, Timed{a.React(t), v})
			if ev.Until != nil {
				a.Badges = append(a.Badges, Timed{a.React(u), prev})
			}
			sort.SliceStable(a.Badges, func(i, j int) bool { return a.Badges[i].T < a.Badges[j].T })
		}
	case "walk":
		a, err := target()
		if err != nil {
			return err
		}
		x, y := a.Rest(t)
		st := ""
		if _, ok := s.Specs[a.Kind].States["walk"]; ok {
			st = "walk"
		}
		a.Legs = append(a.Legs, Leg{T0: t, T1: t + ev.Dur, From: [2]float64{x, y}, To: [2]float64{ev.To.Coords[0], ev.To.Coords[1]}, State: st})
	case "admit":
		return s.admit(ev, t)
	case "tries":
		return s.tries(ev, t)
	default:
		return fmt.Errorf("unknown shared behavior event %q", ev.Do)
	}
	return nil
}

func (s *Stage) targets(ev scene.Event) ([]*Actor, error) {
	if ev.Target == nil {
		return nil, fmt.Errorf("needs target: id of a cast entity or crowd in this shot")
	}

	var raw struct {
		Part bool `json:"part"`
	}
	if err := json.Unmarshal(ev.Raw, &raw); err != nil {
		return nil, err
	}
	if a := s.ByID[ev.Target.ID]; a != nil {
		if raw.Part {
			return nil, fmt.Errorf("part is for crowds, %s is one entity", ev.Target.ID)
		}
		return []*Actor{a}, nil
	}
	cr := s.CrowdByID[ev.Target.ID]
	if cr == nil {
		return nil, fmt.Errorf("target %q is not a cast entity or crowd of this shot", ev.Target.ID)
	}
	if !raw.Part {
		return cr.Units, nil
	}
	if len(cr.Part) == 0 {
		return nil, fmt.Errorf("crowd %s has no part", cr.ID)
	}
	return cr.Part, nil
}

func untilValue(s *Stage, at *scene.At, def float64) (float64, error) {
	if at == nil {
		return def, nil
	}
	u, err := s.Resolve(*at)
	if err != nil {
		return 0, err
	}
	return math.Min(def, u), nil
}

func (s *Stage) walkState(kind string) string {
	spec := s.Specs[kind]
	if _, ok := spec.States["walk"]; ok {
		return "walk"
	}
	return spec.Rest
}

func badgeName(name string) string {
	if name == "bad" {
		return "bang"
	}
	return name
}

func (s *Stage) Pose(a *Actor, t float64) (x, y float64, state string, since float64, flip, moving bool) {
	x, y, state, since, flip, moving = a.Pose(t)
	if a.Jitter {
		n := uint64(int(t*30) / 3)
		h := s.Seed*0x9E3779B97F4A7C15 ^ a.Salt*0xBF58476D1CE4E5B9 ^ n*0x94D049BB133111EB ^ uint64(3)*0x2545F4914F6CDD1D
		h ^= h >> 30
		h *= 0xBF58476D1CE4E5B9
		h ^= h >> 27
		h *= 0x94D049BB133111EB
		h ^= h >> 31
		x += float64(int(h%3)-1) * s.Unit
		y += float64(int((h/3)%3)-1) * s.Unit
	}
	return
}
