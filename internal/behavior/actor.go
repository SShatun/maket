package behavior

import (
	"fmt"
	"math"
	"sort"

	"maket/internal/scene"
	spr "maket/internal/style/sprites"
)

type Timed struct {
	T float64
	V string
}

type Leg struct {
	T0, T1   float64
	From, To [2]float64
	State    string
}

type Actor struct {
	ID, Kind, Label          string
	Roles, Badges, Status    []Timed
	Steps                    []spr.Step
	Home                     [2]float64
	Legs                     []Leg
	Period, Shift            float64
	Cycle                    []Leg
	Halt, Resume             float64
	Anim, Appear, Gone, Size float64
	Flip, Unit, Jitter       bool
	Salt                     uint64
}

func (a *Actor) Name() string {
	if a.ID != "" {
		return "cast " + a.ID
	}
	return "crowd of " + a.Kind
}

func (a *Actor) Title() string {
	if a.Label != "" {
		return a.Label
	}
	return a.ID
}

func (a *Actor) States() []string {
	var out []string
	for _, st := range a.Steps {
		out = append(out, st.V)
	}
	for _, l := range append(append([]Leg(nil), a.Legs...), a.Cycle...) {
		if l.State != "" {
			out = append(out, l.State)
		}
	}
	return out
}

func (a *Actor) Rest(t float64) (float64, float64) {
	x, y, _, _, _, _ := a.Pose(t)
	return x, y
}

func (a *Actor) Pose(t float64) (x, y float64, state string, since float64, flip, moving bool) {
	x, y, flip = a.Home[0], a.Home[1], a.Flip
	if a.Period > 0 {
		tt, frozen := t, false
		if t >= a.Halt {
			if t < a.Resume {
				tt, frozen = a.Halt, true
			} else {
				tt = t - (a.Resume - a.Halt)
			}
		}
		u := math.Mod(tt-a.Appear+a.Shift, a.Period)
		if u < 0 {
			u += a.Period
		}
		for _, l := range a.Cycle {
			if u >= l.T0 && u < l.T1 {
				x, y = lerp(l.From, l.To, (u-l.T0)/(l.T1-l.T0))
				if l.To[0] != l.From[0] {
					flip = l.To[0] < l.From[0]
				}
				if frozen {
					return x, y, "", 0, flip, false
				}
				return x, y, l.State, t - (u - l.T0) - a.Anim, flip, true
			}
		}
		return x, y, "", 0, flip, false
	}
	for _, l := range a.Legs {
		if t < l.T0 {
			break
		}
		if l.To[0] != l.From[0] {
			flip = l.To[0] < l.From[0]
		}
		if t < l.T1 {
			x, y = lerp(l.From, l.To, (t-l.T0)/(l.T1-l.T0))
			return x, y, l.State, l.T0 - a.Anim, flip, true
		}
		x, y = l.To[0], l.To[1]
	}
	return x, y, "", 0, flip, false
}

func (a *Actor) React(t float64) float64 {
	if a.Unit {
		return t + 0.3*a.Anim
	}
	return t
}

func lerp(p, q [2]float64, k float64) (float64, float64) {
	return p[0] + (q[0]-p[0])*k, p[1] + (q[1]-p[1])*k
}

type Crowd struct {
	Start, Arrival            float64
	ID, Title, Behavior, Kind string
	Units, Part               []*Actor
	Status                    []Timed
	Selected                  bool
	Lanes                     [][]*Actor
	Slot                      func(c, i int) [2]float64
	Speed, StepY              float64
}

func (c *Crowd) Box(width, height float64) (x0, y0, x1, y1 float64) {
	x0, y0, x1, y1 = math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	add := func(p [2]float64) {
		if p[0] < 0 || p[0] > width || p[1] < 0 || p[1] > height {
			return
		}
		x0, y0, x1, y1 = math.Min(x0, p[0]), math.Min(y0, p[1]), math.Max(x1, p[0]), math.Max(y1, p[1])
	}
	for _, u := range c.Units {
		add(u.Home)
		for _, l := range u.Cycle {
			add(l.From)
			add(l.To)
		}
		for _, l := range u.Legs {
			add(l.To)
		}
	}
	return
}

func (a *Actor) addTimed(dst *[]Timed, in []scene.Timed[string], resolve func(scene.At) (float64, error)) error {
	for _, x := range in {
		t, err := resolve(x.At)
		if err != nil {
			return err
		}
		*dst = append(*dst, Timed{T: t, V: x.V})
	}
	sort.SliceStable(*dst, func(i, j int) bool { return (*dst)[i].T < (*dst)[j].T })
	return nil
}

func Last(xs []Timed, t float64) (string, float64) {
	v, at := "", 0.0
	for _, x := range xs {
		if t >= x.T {
			v, at = x.V, x.T
		}
	}
	return v, at
}

func ValidateStates(a *Actor, specs map[string]spr.Spec) error {
	spec, ok := specs[a.Kind]
	if !ok {
		return fmt.Errorf("%s: unknown kind %q", a.Name(), a.Kind)
	}
	for _, state := range a.States() {
		if _, ok := spec.States[state]; !ok {
			return fmt.Errorf("%s: %s has no state %q", a.Name(), a.Kind, state)
		}
	}
	return nil
}
