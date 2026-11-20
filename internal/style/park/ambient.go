package park

import (
	"encoding/json"
	"fmt"
	"strings"
)

type ambient struct {
	From       string
	SkipAt     []point  `json:"skip_at"`
	SkipQueues []string `json:"skip_queues"`
	Guests     []int
	Walkers    []struct {
		Obj, From, To string
		Speed, Offset float64
		Shift         point
	}
}

func (sh *shot) addAmbient(cfg ambient) error {
	if cfg.From != "reference" {
		return fmt.Errorf("ambient.from wants reference")
	}
	occupied := map[point]bool{}
	for _, at := range cfg.SkipAt {
		occupied[at] = true
	}
	for _, o := range sh.objects {
		occupied[o.at] = true
	}
	for _, name := range cfg.SkipQueues {
		q := sh.art.queues[name]
		if q == nil {
			return fmt.Errorf("unknown ambient queue %q", name)
		}
		for _, s := range q.Slots {
			occupied[point{s.X, s.Y}] = true
		}
	}
	guest := 0
	for _, r := range sh.world.Reference {
		var name string
		var args []any
		if err := json.Unmarshal(r.Call[0], &name); err != nil {
			return err
		}
		if err := json.Unmarshal(r.Call[1], &args); err != nil {
			return err
		}
		if r.Kind == "person" {
			var props struct {
				View, Role string
				Kid        bool
			}
			if err := json.Unmarshal(r.Call[2], &props); err != nil {
				return err
			}
			kind := props.Role
			if kind == "" {
				var err error
				kind, err = sh.guestKind(sh.guestsByAge[props.Kid], cfg.Guests, guest)
				if err != nil {
					return err
				}
				guest++
			}
			if occupied[r.Anchor] {
				continue
			}
			if props.View == "" && len(args) > 0 {
				props.View = fmt.Sprint(args[0])
			}
			if _, err := sh.newPerson(kind, r.Anchor, props.View, r.Flip); err != nil {
				return err
			}
			continue
		}
		if occupied[r.Anchor] {
			continue
		}
		if err := sh.referenceObject(name, args, r.Anchor); err != nil {
			return err
		}
	}
	if len(cfg.Guests) > guest {
		return fmt.Errorf("ambient.guests has %d entries for %d reference guests", len(cfg.Guests), guest)
	}
	for _, w := range cfg.Walkers {
		if _, ok := sh.paths.Nodes[w.From]; !ok {
			return fmt.Errorf("unknown walker node %q", w.From)
		}
		if _, ok := sh.paths.Nodes[w.To]; !ok {
			return fmt.Errorf("unknown walker node %q", w.To)
		}
		route := sh.route(w.From, w.To)
		if len(route) < 2 || w.Offset < 0 || w.Speed < 0 {
			return fmt.Errorf("invalid ambient walk %s to %s", w.From, w.To)
		}
		speed := w.Speed
		if speed == 0 {
			speed = 180
		}
		for i := range route {
			route[i][0] += w.Shift[0]
			route[i][1] += w.Shift[1]
		}
		p, err := sh.newPerson(w.Obj, route[0], "front", false)
		if err != nil {
			return err
		}
		t := sh.at - w.Offset
		for t < sh.end {
			for i := 1; i < len(route); i++ {
				end := t + route[i-1].distance(route[i])/speed
				p.move(t, end, route[i-1], route[i], "", false)
				t = end
			}

		}
	}
	return nil
}

func (sh *shot) referenceObject(name string, args []any, at point) error {
	arg := func(i int) string {
		if i >= len(args) {
			return ""
		}
		return fmt.Sprint(args[i])
	}
	toggle := func(on, off string) string {
		if arg(0) == "true" {
			return on
		}
		return off
	}
	kind, state, value := name, "", ""
	switch name {
	case "ferris_v", "carousel_v":
		kind = strings.TrimSuffix(name, "_v")
	case "led_board":
		for k, b := range sh.sprites.Boards {
			if b.Title == arg(0) {
				kind = k
				break
			}
		}
		value, state = arg(1), arg(2)
	case "seat_meter":
		kind, value = "seats"+arg(0), arg(1)
	case "traffic_light":
		kind, state = "light", arg(0)
	case "junction":
		state = toggle("on", "off")
	case "loudspeaker":
		kind, state = "speaker", toggle("on", "off")
	case "validator":
		state = toggle("ok", "bad")
	case "plate":
		kind = "plate_" + strings.ReplaceAll(arg(0), " ", "_")
	case "smoke", "spark":
		kind = name + "_" + arg(0)
	case "bubble", "icon":
		return nil
	}
	o, err := sh.newObject(kind, at, state)
	if err != nil {
		return err
	}
	if value != "" {
		o.values = []mark{{sh.at, sh.end, value}}
	}
	if o.seats > 0 && len(args) > 2 {
		n, _ := args[2].(float64)
		for range int(n) {
			o.trips = append(o.trips, &trip{start: sh.at, boarded: sh.at, end: sh.end})
		}
	}
	return nil
}
