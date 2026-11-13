package rts

import (
	"fmt"
	"math"
	"math/rand"

	"maket/internal/behavior"
	"maket/internal/scene"
)

func (s *Style) crowd(sh *shot, c scene.Cast, _ int, rng *rand.Rand) (*crowd, error) {
	c.Size = 1
	cr, err := sh.behavior.AddCrowd(c, rng)
	if err != nil {
		return nil, err
	}
	if cr.Kind == "warrior" {
		sh.horde = math.Min(sh.horde, cr.Start)
		sh.horn = math.Min(sh.horn, cr.Arrival)
	}
	if err := s.crowdTag(sh, cr, c); err != nil {
		return nil, err
	}
	return cr, nil
}

func (s *Style) behaviorStage(at, until float64) *behavior.Stage {
	return behavior.NewStage(behavior.Config{
		At:           at,
		Until:        until,
		Width:        mapW,
		Height:       mapH,
		Unit:         1,
		Specs:        s.set.Specs,
		Resolve:      s.when,
		DefaultRoles: map[string]string{"warrior": "error"},
		GateOffsets:  map[string][2]float64{"lock": {-5, 0}},
	})
}

func (s *Style) addBehaviorEntity(sh *shot, c scene.Cast, rc rawCast) (*actor, error) {
	if len(c.At) != 2 || c.At[0] < 0 || c.At[0] > mapW || c.At[1] < 0 || c.At[1] > mapH {
		return nil, fmt.Errorf("cast %s: at wants map px inside 180x229, got %v", c.ID, c.At)
	}
	c.Size = 1
	if c.Role == "" {
		c.Role = "external"
	}
	a, err := sh.behavior.AddEntity(c)
	if err != nil {
		return nil, err
	}
	a.Flip = rc.Flip
	if err := s.entityTag(sh, a, c); err != nil {
		return nil, err
	}
	return a, nil
}
