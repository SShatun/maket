package park

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"maket/internal/audio"
	"maket/internal/scene"
)

type action struct {
	scene.Event
	t, end float64
	parameters
}

type parameters struct {
	Count                     int
	Every, Speed, Dy          float64
	Name, Preset, Item, Color string
	Value                     json.RawMessage
	On                        *bool
	Episode                   int
}

func (sh *shot) events(events []scene.Event) error {
	var actions []action
	for _, ev := range events {
		a := action{Event: ev, parameters: parameters{Count: 1}, end: sh.end}
		var err error
		if a.t, err = sh.tl.Resolve(ev.At); err != nil {
			return err
		}
		if ev.Until != nil {
			if a.end, err = sh.tl.Resolve(*ev.Until); err != nil {
				return err
			}
		}
		if err = json.Unmarshal(ev.Raw, &a.parameters); err != nil {
			return err
		}
		if (a.Do == "admit" || a.Do == "release") && a.Until != nil {
			if err := a.fitTransfers(); err != nil {
				return err
			}
		}
		actions = append(actions, a)
	}
	sort.SliceStable(actions, func(i, j int) bool { return actions[i].t < actions[j].t })
	for len(actions) > 0 {
		a := actions[0]
		actions = actions[1:]
		if a.t >= sh.end {
			continue
		}
		if err := sh.apply(a); err != nil {
			return fmt.Errorf("%s at %.3f: %w", a.Do, a.t, err)
		}
		if a.Do == "release" && a.Count > 1 {
			a.t += a.Every
			a.Count--
			i := sort.Search(len(actions), func(i int) bool { return actions[i].t > a.t })
			actions = append(actions, action{})
			copy(actions[i+1:], actions[i:])
			actions[i] = a
		}
	}
	sh.advance(sh.end)
	return nil
}

func (a *action) fitTransfers() error {
	if a.Count < 1 || a.Dur < 0 || a.Every < 0 || (a.Count > 1 && a.Every == 0) || a.end <= a.t {
		return fmt.Errorf("%s needs a positive transfer interval", a.Do)
	}
	if a.Do == "release" {
		if a.Count > 1 {
			a.Every = (a.end - a.t) / float64(a.Count-1)
		}
		return nil
	}
	if a.Dur == 0 {
		a.Dur = boardingDuration
	}
	duration := float64(a.Count-1)*a.Every + a.Dur
	factor := (a.end - a.t) / duration
	a.Every *= factor
	a.Dur *= factor
	return nil
}

func id(p *scene.Point) string {
	if p == nil {
		return ""
	}
	return p.ID
}

func (sh *shot) apply(a action) error {
	sh.advance(a.t)
	o := sh.byID[id(a.Target)]
	switch a.Do {
	case "admit", "release":
		if a.Count < 1 || a.Every < 0 || a.Count > 1 && a.Every == 0 || a.Dur < 0 {
			return fmt.Errorf("invalid count, every or dur")
		}
		if a.Do == "release" {
			return sh.release(o, a)
		}
		q, ride := sh.queues[id(a.From)], sh.byID[id(a.To)]
		if q == nil || ride == nil || ride.seats == 0 && ride.kind != "gate" {
			return fmt.Errorf("admit needs a queue and a ride or gate")
		}
		sh.pending = append(sh.pending, &admission{q, ride, a.Count, a.t, a.Every, a.Dur})
	case "ride":
		if o == nil || o.seats == 0 || !rideState(a.State) {
			return fmt.Errorf("invalid ride or state %q", a.State)
		}
		o.states = append(o.states, mark{a.t, sh.end, a.State})
	case "board":
		if o == nil {
			return fmt.Errorf("unknown board")
		}
		return sh.setBoard(o, a.Value, a.State, a.t)
	case "power":
		if o == nil || o.kind != "junction" || a.On == nil {
			return fmt.Errorf("power needs a junction and on")
		}
		state := "off"
		if *a.On {
			state = "on"
		}
		o.states = append(o.states, mark{a.t, sh.end, state})
	case "announce":
		if o == nil || o.kind != "speaker" {
			return fmt.Errorf("announce needs a speaker")
		}
		o.states = append(o.states, mark{a.t, a.end, "on"})
	case "emote", "carry", "bubble", "icon", "walk":
		ps, err := sh.targets(a.Target, a.Count)
		if err != nil {
			return err
		}
		for _, p := range ps {
			switch a.Do {
			case "walk":
				if err := sh.walk(p, a); err != nil {
					return err
				}
			case "carry":
				if err := sh.carry(p, a.Item, a.t); err != nil {
					return err
				}
			case "emote":
				if err := sh.emote(p, a.State, a.t, a.end); err != nil {
					return err
				}
			case "icon":
				if sh.images["fx/icon_"+a.Kind+"_0.png"] == nil {
					return fmt.Errorf("unknown icon %q", a.Kind)
				}
				p.icons = append(p.icons, mark{a.t, a.end, a.Kind})
			case "bubble":
				if strings.TrimSpace(a.Text) == "" {
					return fmt.Errorf("empty bubble")
				}
				sh.prepareBubble(a.Text)
				p.bubbles = append(p.bubbles, bubbleMark{mark{a.t, a.end, a.Text}, point{a.Dx, a.Dy}})
			}
		}
	case "camera":
		pos, ok := sh.cameras.Presets[a.Preset]
		if !ok || a.Dur <= 0 {
			return fmt.Errorf("camera needs preset and positive dur")
		}
		sh.camera = append(sh.camera, leg{at: a.t, end: a.t + a.Dur, from: sh.cameraAt(a.t), to: pos})
		sh.tl.AddActivity(a.t, min(a.t+a.Dur, sh.end))
	case "title":
		line := a.Text
		if len(a.Lines) == 1 {
			line = a.Lines[0].Text
		}
		if len(a.Lines) > 1 {
			return fmt.Errorf("title needs one line")
		}
		im, err := titleImage(sh.theme.Fonts, a.Episode, line)
		if err != nil {
			return err
		}
		sh.titles = append(sh.titles, title{a.t, a.end, im})
	case "cut", "flash":
		if a.Color != "" && a.Color != "black" && a.Color != "white" || a.Dur < 0 {
			return fmt.Errorf("invalid cut")
		}
		d := a.Dur
		if d == 0 {
			d = .3
		}
		c := cut{a.t, d, a.Color == "white" || a.Do == "flash"}
		if a.t == sh.at {
			sh.cuts[0] = c
		} else {
			sh.cuts = append(sh.cuts, c)
		}
	case "sfx":
		switch a.Name {
		case audio.Crowd, audio.Click, audio.Chord, audio.Pop, audio.Whoosh, audio.Chime, audio.Buzz, audio.Rise, audio.Land, audio.Thud:
		default:
			return fmt.Errorf("unknown sound %q", a.Name)
		}
		dur := a.Dur
		if dur < 0 {
			return fmt.Errorf("negative sfx duration")
		}
		if a.Name == audio.Crowd && dur == 0 {
			dur = 1.2
		}
		sh.cue(a.t, a.Name, min(dur, sh.end-a.t))
	default:
		return fmt.Errorf("unknown park event %q", a.Do)
	}
	sh.advance(a.t)
	return nil
}

func (sh *shot) targets(target *scene.Point, count int) ([]*person, error) {
	if p := sh.actors[id(target)]; p != nil {
		return []*person{p}, nil
	}
	q := sh.queues[id(target)]
	if q == nil {
		return nil, fmt.Errorf("unknown person or queue %q", id(target))
	}
	i := max(0, target.Slot)
	if count < 1 || i > len(q.people) || count > len(q.people)-i {
		return nil, fmt.Errorf("target outside queue %s", target.ID)
	}
	return q.people[i : i+count], nil
}

func (sh *shot) carry(p *person, item string, t float64) error {
	if item != "" && sh.sprites.Items[item].PNG == "" {
		return fmt.Errorf("unknown item %q", item)
	}
	p.items = append(p.items, mark{t, sh.end, item})
	return nil
}

func (sh *shot) emote(p *person, state string, t, end float64) error {
	if state == "walk" || sh.sprites.People[p.kind].Frames["front_"+state+"_0"].PNG == "" {
		return fmt.Errorf("unknown pose %q", state)
	}
	p.poses = append(p.poses, mark{t, end, state})
	return nil
}

func (sh *shot) walk(p *person, a action) error {
	if a.To == nil || a.Speed < 0 || a.Dur < 0 {
		return fmt.Errorf("walk needs a destination and positive speed")
	}
	pos := func(ref *scene.Point) (point, error) {
		if len(ref.Coords) == 2 {
			return point(ref.Coords), nil
		}
		if n, ok := sh.paths.Nodes[ref.ID]; ok {
			return n, nil
		}
		return point{}, fmt.Errorf("unknown path node %q", ref.ID)
	}
	_, from := p.pose(a.t)
	var err error
	if a.From != nil {
		if from, err = pos(a.From); err != nil {
			return err
		}
	}
	to, err := pos(a.To)
	if err != nil {
		return err
	}
	var route []point
	if a.To.ID != "" || id(a.From) != "" {
		route = sh.route(sh.nearest(from), sh.nearest(to))
	}
	route = append(append([]point{from}, route...), to)
	speed := a.Speed
	if speed == 0 {
		speed = 180
	}
	if a.Dur > 0 {
		dist := 0.
		for i := 1; i < len(route); i++ {
			dist += route[i-1].distance(route[i])
		}
		speed = dist / a.Dur
	}
	t := a.t
	for i := 1; i < len(route); i++ {
		if route[i] == route[i-1] {
			continue
		}
		end := t + route[i-1].distance(route[i])/speed
		p.move(t, end, route[i-1], route[i], "", false)
		t = end
	}
	return nil
}

func (sh *shot) setBoard(o *object, raw json.RawMessage, state string, t float64) error {
	value := valueAt(o.values, t)
	if len(raw) > 0 {
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			var n int
			if err = json.Unmarshal(raw, &n); err != nil {
				return fmt.Errorf("board value needs text or integer")
			}
			text = strconv.Itoa(n)
		}
		value = text
	}
	if err := sh.checkBoard(o.kind, value, state); err != nil {
		return err
	}
	if len(raw) > 0 {
		o.values = append(o.values, mark{t, sh.end, value})
	}
	if state != "" {
		o.states = append(o.states, mark{t, sh.end, state})
	}
	return nil
}

func (sh *shot) checkBoard(kind, value, state string) error {
	if b := sh.sprites.Boards[kind]; b.PNG != "" {
		if _, ok := b.Colors[state]; !ok && state != "" {
			return fmt.Errorf("unknown board colour %q", state)
		}
		return nil
	}
	switch kind {
	case "light", "validator", "junction", "speaker":
		if state == "" {
			return fmt.Errorf("%s needs a state", kind)
		}
		if sh.images["boards/"+kind+"_"+state+".png"] != nil || kind == "speaker" && state == "on" {
			return nil
		}
	case "seats6", "seats8":
		if sh.images["boards/"+kind+"_"+value+".png"] != nil {
			return nil
		}
	}
	return fmt.Errorf("invalid %s board state/value %q/%q", kind, state, value)
}
