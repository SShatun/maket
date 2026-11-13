package behavior

import (
	"encoding/json"
	"fmt"
	"maket/internal/scene"
	"math"
	"strconv"
	"strings"
)

type Tally struct {
	Who      *Actor
	T, Until float64
	Steps    []float64
	Values   []string
	Badge    string
}

func (t *Tally) Index(at float64) int {
	i := 0
	for i+1 < len(t.Steps) && at >= t.Steps[i+1] {
		i++
	}
	return i
}
func (t *Tally) Remaining(at float64) float64 {
	i := t.Index(at)
	if i+1 >= len(t.Steps) || t.Steps[i+1] <= t.Steps[i] {
		return 1
	}
	return 1 - math.Max(0, math.Min(1, (at-t.Steps[i])/(t.Steps[i+1]-t.Steps[i])))
}
func (s *Stage) TallyAt(a *Actor, t float64) *Tally {
	for i := len(s.Tallies) - 1; i >= 0; i-- {
		x := s.Tallies[i]
		if x.Who == a && t >= x.T && t < x.Until {
			return x
		}
	}
	return nil
}

func (s *Stage) tries(ev scene.Event, t float64) error {
	if ev.Target == nil || s.ByID[ev.Target.ID] == nil {
		return fmt.Errorf("tries needs target: id of one cast entity in this shot")
	}
	a := s.ByID[ev.Target.ID]
	var re struct {
		Count []float64 `json:"count"`
		Badge *string   `json:"badge"`
	}
	if err := json.Unmarshal(ev.Raw, &re); err != nil {
		return err
	}
	badge := "wait"
	if re.Badge != nil {
		badge = *re.Badge
	}
	if badge == "bad" {
		badge = "bang"
	}
	text := ev.Text
	if text == "" {
		text = "{n}"
	}
	var vals []string
	var steps []float64
	if c := re.Count; c != nil {
		if len(c) != 3 || c[2] < 0 {
			return fmt.Errorf("tries: count wants [from, to, seconds]")
		}
		if !strings.Contains(text, "{n}") {
			text = "{n}" + text
		}
		from, to := int(math.Round(c[0])), int(math.Round(c[1]))
		k, dir := to-from, 1
		if k < 0 {
			k, dir = -k, -1
		}
		for i := 0; i <= k; i++ {
			ti := t
			if k > 0 {
				ti = t + float64(i)*c[2]/float64(k)
			}
			vals = append(vals, strings.ReplaceAll(text, "{n}", strconv.Itoa(from+i*dir)))
			steps = append(steps, ti)
		}
	} else if strings.Contains(text, "{n}") {
		return fmt.Errorf("tries: {n} in text needs count")
	} else {
		vals, steps = []string{text}, []float64{t}
	}
	u, err := untilValue(s, ev.Until, s.Until)
	if err != nil {
		return err
	}
	s.Tallies = append(s.Tallies, &Tally{Who: a, T: t, Until: u, Steps: steps, Values: vals, Badge: badge})
	return nil
}
