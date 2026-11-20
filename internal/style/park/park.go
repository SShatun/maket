package park

import (
	"encoding/json"
	"fmt"
	"image"
	"math/rand"
	"os"
	"path/filepath"
	"sort"

	"maket/internal/audio"
	"maket/internal/render"
	"maket/internal/scene"
	"maket/internal/timeline"
)

type Style struct {
	*art
	theme    render.Theme
	shots    []*shot
	captions []caption
	cues     []audio.Cue
	duration float64
	labels   map[label]*image.RGBA
}

type title struct {
	at, until float64
	im        *image.RGBA
}
type cut struct {
	at, dur float64
	white   bool
}

type shot struct {
	*Style
	at, end float64
	camera  []leg
	objects []*object
	people  []*person
	byID    map[string]*object
	actors  map[string]*person
	queues  map[string]*queue
	titles  []title
	cuts    []cut
	signals []signal
	pending []*admission
	clock   float64
	rng     *rand.Rand
	tl      *timeline.Timeline
}

type signal struct {
	at   float64
	gate point
}

type queue struct {
	*line
	people []*person
}

type object struct {
	kind           string
	at             point
	boarding       point
	seats          int
	trips          []*trip
	states, values []mark
	running        []timeline.Interval
	link, power    *object
}

type trip struct {
	person              *person
	start, boarded, end float64
}

func (o *object) occupied(t float64, reserved bool) int {
	n := 0
	for _, r := range o.trips {
		if t < r.end && (r.boarded <= t || reserved && r.start <= t) {
			n++
		}
	}
	return n
}

func (o *object) state(t float64) string {
	if o.power != nil && valueAt(o.power.states, t) == "off" {
		return "broken"
	}
	return valueAt(o.states, t)
}

func New(root string, fonts *render.Fonts) (*Style, error) {
	a, err := loadArt(filepath.Join(root, "assets", "looks", "park"))
	if err != nil {
		return nil, err
	}
	theme := render.LightTheme(fonts)
	theme.OwnCaption, theme.NoProgress = true, true
	return &Style{art: a, theme: theme}, nil
}

func (s *Style) Theme() render.Theme { return s.theme }
func (s *Style) Sounds() []audio.Cue { return s.cues }

func (s *Style) Layout(sc *scene.Scene, tl *timeline.Timeline) error {
	if len(sc.Shots) == 0 {
		return fmt.Errorf("park needs shots")
	}
	starts, err := tl.ShotStarts(sc)
	if err != nil {
		return err
	}
	s.shots, s.cues, s.duration = nil, nil, tl.Duration
	s.labels = map[label]*image.RGBA{}
	for i, raw := range sc.Shots {
		end := tl.Duration
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		sh := &shot{Style: s, at: starts[i], end: end, tl: tl, rng: rand.New(rand.NewSource(int64(sc.Seed + i))),
			byID: map[string]*object{}, actors: map[string]*person{}, queues: map[string]*queue{}}
		if err := sh.layout(raw); err != nil {
			return fmt.Errorf("park shot %d: %w", i+1, err)
		}
		s.shots = append(s.shots, sh)
	}
	s.captions = nil
	for i, pt := range tl.Phrases {
		c, w := newCaption(s.theme.Fonts, sc.Phrases[i], pt)
		c.end = min(tl.Duration, pt.End+0.35)
		if i+1 < len(tl.Phrases) {
			c.end = min(c.end, tl.Phrases[i+1].Start)
		}
		tl.AddActivity(c.end, c.end)
		for _, word := range c.words {
			tl.AddActivity(word.At, word.At)
		}
		if w > 960 {
			fmt.Fprintf(os.Stderr, "maket: park phrase %d subtitle is %d px wide (maximum 960 px)\n", i+1, w)
		}
		s.captions = append(s.captions, c)
	}
	return nil
}

func (sh *shot) layout(raw scene.Shot) error {
	p, ok := sh.cameras.Presets[raw.Place.Name]
	if !ok {
		return fmt.Errorf("unknown camera preset %q", raw.Place.Name)
	}
	sh.camera = []leg{{at: sh.at, end: sh.at, from: p, to: p}}
	sh.cuts = []cut{{sh.at, .4, false}}
	type links struct {
		o            *object
		value, power string
	}
	var bindings []links
	for _, c := range raw.Cast {
		if c.Crowd != nil {
			if err := sh.addQueue(c); err != nil {
				return err
			}
			continue
		}
		var cfg struct {
			View, Item, Bind, Power string
			Flip                    bool
			Value                   json.RawMessage
			Riders                  int
			Boarding                *point
		}
		if err := json.Unmarshal(c.Raw, &cfg); err != nil {
			return err
		}
		at := point(c.At)
		if _, ok := sh.sprites.People[c.Kind]; ok {
			p, err := sh.newPerson(c.Kind, at, cfg.View, cfg.Flip)
			if err != nil {
				return err
			}
			sh.actors[c.ID] = p
			if c.State != "" {
				if err := sh.emote(p, c.State, sh.at, sh.end); err != nil {
					return err
				}
			}
			if err := sh.carry(p, cfg.Item, sh.at); err != nil {
				return err
			}
			continue
		}
		o, err := sh.newObject(c.Kind, at, c.State)
		if err != nil {
			return fmt.Errorf("%s: %w", c.ID, err)
		}
		sh.byID[c.ID] = o
		if cfg.Boarding != nil {
			o.boarding = *cfg.Boarding
		}
		if cfg.Riders < 0 || cfg.Riders > o.seats {
			return fmt.Errorf("%s: invalid rider count", c.ID)
		}
		for range cfg.Riders {
			o.trips = append(o.trips, &trip{start: sh.at, boarded: sh.at, end: sh.end})
		}
		if len(cfg.Value) > 0 {
			if err := sh.setBoard(o, cfg.Value, c.State, sh.at); err != nil {
				return err
			}
		}
		bindings = append(bindings, links{o, cfg.Bind, cfg.Power})
	}
	for _, b := range bindings {
		if b.value != "" {
			r := sh.byID[b.value]
			if r == nil || r.seats == 0 {
				return fmt.Errorf("unknown ride %q", b.value)
			}
			switch b.o.kind {
			case "free", "seats6", "seats8", "light":
				if b.o.kind == "seats6" && r.seats > 6 || b.o.kind == "seats8" && r.seats > 8 {
					return fmt.Errorf("%s cannot display %d seats", b.o.kind, r.seats)
				}
				b.o.link = r
			default:
				return fmt.Errorf("%s cannot bind to ride", b.o.kind)
			}
		}
		if b.power != "" {
			p := sh.byID[b.power]
			if p == nil || p.kind != "junction" || b.o.seats == 0 {
				return fmt.Errorf("invalid junction %q for %s", b.power, b.o.kind)
			}
			b.o.power = p
		}
	}
	var cfg struct{ Ambient *ambient }
	if err := json.Unmarshal(raw.Raw, &cfg); err != nil {
		return err
	}
	if cfg.Ambient != nil {
		if err := sh.addAmbient(*cfg.Ambient); err != nil {
			return err
		}
	}
	if err := sh.events(raw.Do); err != nil {
		return err
	}
	for _, ride := range sh.objects {
		if ride.seats > 0 {
			ride.running = ride.runningSpans(sh.at, sh.end)
		}
	}
	for _, p := range sh.people {
		for i, l := range p.legs {
			end := min(sh.end, l.end)
			if i+1 < len(p.legs) {
				end = min(end, p.legs[i+1].at)
			}
			if l.from != l.to {
				sh.tl.AddActivity(max(sh.at, l.at), end)
			}
		}
	}
	sh.prepareBoards()
	sh.soundtrack()
	return nil
}

func (sh *shot) newPerson(kind string, pos point, view string, flip bool) (*person, error) {
	if view == "" {
		view = "front"
	}
	if sh.sprites.People[kind].Frames[view+"_stand_0"].PNG == "" {
		return nil, fmt.Errorf("unknown person/view %s/%s", kind, view)
	}
	p := &person{kind: kind, phase: sh.rng.Float64(), legs: []leg{{sh.at, sh.at, pos, pos, view, flip}}}
	sh.people = append(sh.people, p)
	return p, nil
}

func (sh *shot) newObject(kind string, at point, state string) (*object, error) {
	o := &object{kind: kind, at: at, boarding: at}
	if r, ok := sh.sprites.Rides[kind]; ok {
		o.seats = r.Seats
		if kind == "coaster" {
			o.seats = 6
		}
		if state == "" {
			state = "idle"
		}
		if !rideState(state) {
			return nil, fmt.Errorf("invalid ride state %q", state)
		}
	} else {
		switch kind {
		case "light", "seats6", "seats8", "speaker", "validator", "junction", "gate":
		default:
			if sh.sprites.Boards[kind].PNG == "" && sh.images["fx/"+kind+".png"] == nil {
				return nil, fmt.Errorf("unknown object %q", kind)
			}
		}
	}
	o.states = []mark{{sh.at, sh.end, state}}
	sh.objects = append(sh.objects, o)
	return o, nil
}

func rideState(s string) bool { return s == "idle" || s == "running" || s == "broken" || s == "closed" }

func (sh *shot) guestKind(pool []string, roster []int, index int) (string, error) {
	var kind string
	if len(pool) > 0 {
		kind = pool[sh.rng.Intn(len(pool))]
	}
	if index < len(roster) {
		return fmt.Sprintf("guest%02d", roster[index]), nil
	}
	if kind == "" {
		return "", fmt.Errorf("no matching guest in park sprites.json")
	}
	return kind, nil
}

func (sh *shot) addQueue(c scene.Cast) error {
	if c.ID == "" || c.Crowd.Behavior != "queue" {
		return fmt.Errorf("a park crowd needs id and behavior: queue")
	}
	var cfg struct {
		Crowd struct {
			Queue  string
			Guests []int
		}
	}
	if err := json.Unmarshal(c.Raw, &cfg); err != nil {
		return err
	}
	line := sh.art.queues[cfg.Crowd.Queue]
	n := c.Crowd.Count
	if line == nil || n < 0 || n > len(line.Slots) || len(cfg.Crowd.Guests) > 0 && len(cfg.Crowd.Guests) != n {
		return fmt.Errorf("invalid queue or count for %s", c.ID)
	}
	q := &queue{line: line}
	for i, s := range line.Slots[:n] {
		kind, err := sh.guestKind(sh.guests, cfg.Crowd.Guests, i)
		if err != nil {
			return err
		}
		p, err := sh.newPerson(kind, point{s.X, s.Y}, s.View, s.Flip)
		if err != nil {
			return err
		}
		q.people = append(q.people, p)
	}
	sh.queues[c.ID] = q
	return nil
}

func (sh *shot) cameraAt(t float64) point {
	i := sort.Search(len(sh.camera), func(i int) bool { return sh.camera[i].at > t }) - 1
	l := sh.camera[max(0, i)]
	if t >= l.end {
		return l.to
	}
	return l.from.mix(l.to, smooth((t-l.at)/(l.end-l.at)))
}
