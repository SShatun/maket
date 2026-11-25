package noir

import (
	"encoding/json"
	"fmt"
	"image"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"maket/internal/audio"
	"maket/internal/render"
	"maket/internal/scene"
	"maket/internal/timeline"
)

type Style struct {
	*art
	theme render.Theme
	sc    *scene.Scene
	tl    *timeline.Timeline
	shots []*shot
	cues  []audio.Cue
	cuts  []*action
}

type shot struct {
	camera []scene.Event
	*Style
	*location
	place      string
	start, end float64
	people     []*person
	byID       map[string]*person
	actions    []*action
	board      *board
	mini       *image.RGBA
	rain       bool
}

type action struct {
	scene.Event
	at, end, phase float64
	actionParams
	image *image.RGBA
	neon  *neon
}

func New(root string, fonts *render.Fonts) (*Style, error) {
	a, err := loadArt(filepath.Join(root, "assets", "looks", "noir"))
	if err != nil {
		return nil, err
	}
	f := *fonts
	f.Bold = a.font
	th := render.LightTheme(&f)
	th.OwnCaption, th.NoProgress = true, true
	th.Caption = render.CaptionStyle{Y: 1790, Size: 46, Ink: a.palette.Paper.ink(), Edge: a.palette.Ink.ink(), Stroke: 4}
	return &Style{art: a, theme: th}, nil
}

func (s *Style) Theme() render.Theme { return s.theme }
func (s *Style) Sounds() []audio.Cue { return s.cues }
func (s *Style) Layout(sc *scene.Scene, tl *timeline.Timeline) error {
	if len(sc.Shots) == 0 {
		return fmt.Errorf("noir needs shots")
	}
	starts, err := tl.ShotStarts(sc)
	if err != nil {
		return err
	}
	s.sc, s.tl, s.shots = sc, tl, nil
	s.cues = nil
	s.play("rain", 0, tl.Duration)
	s.play("jazz", 0, tl.Duration)
	s.cuts = nil
	boards := map[string]*board{}
	for i, raw := range sc.Shots {
		end := tl.Duration
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		sh := &shot{Style: s, start: starts[i], end: end, place: raw.Place.Name, byID: map[string]*person{}}
		if err := sh.layout(raw, boards); err != nil {
			return fmt.Errorf("noir shot %d (%s): %w", i+1, sh.place, err)
		}
		s.shots = append(s.shots, sh)
	}
	for _, p := range tl.Phrases {
		tl.AddActivity(p.Start, p.Start)
		tl.AddActivity(p.End+.35, p.End+.35)
	}
	chord, _ := audio.NoirCue("jazz-end")
	s.addSound(chord, max(0., tl.Duration-chord.Dur), tl.Duration)
	return nil
}

func (sh *shot) layout(raw scene.Shot, boards map[string]*board) error {
	sh.location = sh.places[sh.place]
	if sh.place == "board" {
		sh.location = &location{}
	}
	if sh.location == nil {
		return fmt.Errorf("unknown place")
	}
	if err := sh.loadPlace(sh.location); err != nil {
		return err
	}
	var cfg struct {
		Case json.RawMessage
		Rain *bool
	}
	if err := json.Unmarshal(raw.Raw, &cfg); err != nil {
		return err
	}
	sh.rain = len(sh.FX) > 0
	if cfg.Rain != nil {
		sh.rain = *cfg.Rain
	}
	if sh.rain {
		if len(sh.FX) == 0 {
			return fmt.Errorf("rain needs street")
		}
		sh.tl.AddActivity(sh.start, sh.end)
	}
	if len(cfg.Case) > 0 {
		if sh.place != "board" && sh.place != "office" {
			return fmt.Errorf("case needs board or office")
		}
		var name string
		if json.Unmarshal(cfg.Case, &name) == nil && sh.sc.Cases[name] != nil {
			cfg.Case = sh.sc.Cases[name]
		}
		key := string(cfg.Case)
		if boards[key] == nil {
			b, err := sh.newBoard(cfg.Case)
			if err != nil {
				return err
			}
			boards[key] = b
		}
		sh.board = boards[key]
		if sh.place == "office" {
			sh.mini = sh.miniature(sh.board.full)
		}
	} else if sh.place == "board" {
		return fmt.Errorf("board needs case")
	}
	slots := sh.slots()
	for _, c := range raw.Cast {
		if sh.place == "board" || sh.place == "archive" {
			return fmt.Errorf("place has no people")
		}
		if c.Crowd != nil {
			if err := sh.addQueue(c, slots); err != nil {
				return err
			}
			continue
		}
		var v struct {
			Slot, Pose string
			Bust, Flip *bool
		}
		if err := json.Unmarshal(c.Raw, &v); err != nil {
			return err
		}
		sl := slot{scale: 1}
		if v.Slot != "" {
			var ok bool
			if sl, ok = slots[v.Slot]; !ok {
				return fmt.Errorf("unknown slot %q", v.Slot)
			}
		} else if len(c.At) != 2 {
			return fmt.Errorf("%s needs at or slot", c.ID)
		}
		if len(c.At) == 2 {
			sl.pos = point(c.At)
		}
		if c.Size > 0 {
			sl.scale = c.Size
		}
		if v.Bust != nil {
			sl.bust = *v.Bust
		}
		if v.Flip != nil {
			sl.flip = *v.Flip
		}
		p, err := sh.person(c.ID, c.Kind, v.Pose, c.State, sl)
		if err != nil {
			return err
		}
		p.atCounter = v.Slot == "clerk"
		sh.byID[c.ID] = p
		sh.people = append(sh.people, p)
	}
	for _, ev := range raw.Do {
		e := &action{Event: ev, at: sh.tl.MustResolve(ev.At), end: sh.end}
		var v actionParams
		if err := json.Unmarshal(ev.Raw, &v); err != nil {
			return err
		}
		e.actionParams = v
		if ev.Until != nil {
			e.end = min(sh.end, sh.tl.MustResolve(*ev.Until))
		}
		if e.end <= e.at {
			return fmt.Errorf("%s: until must follow at", e.Do)
		}
		sh.actions = append(sh.actions, e)
	}
	for i, pt := range sh.tl.Phrases {
		p := sh.byID[sh.sc.Phrases[i].Speaker]
		if p == nil || p.client || pt.End <= sh.start || pt.Start >= sh.end {
			continue
		}
		e := &action{Event: scene.Event{Do: "speak"}, at: pt.Start, end: pt.End}
		if err := sh.speak(p, e); err != nil {
			return err
		}
	}
	sort.SliceStable(sh.actions, func(i, j int) bool { return sh.actions[i].at < sh.actions[j].at })
	for _, e := range sh.actions {
		if err := sh.prepare(e); err != nil {
			return fmt.Errorf("%s at %.2f: %w", e.Do, e.at, err)
		}
	}

	sh.score()
	return nil
}

type actionParams struct {
	Count, Row, Phrase int
	Every, Step        float64
	Time               string
	Tip                *point
	Vertical           bool
	Name               string
	Gain               *float64
}

func (sh *shot) addQueue(c scene.Cast, slots map[string]slot) error {
	q := c.Crowd
	var v struct {
		Crowd struct {
			Guests               []string
			Every, Dur, Distance float64
		}
	}
	if err := json.Unmarshal(c.Raw, &v); err != nil {
		return err
	}
	cfg := v.Crowd
	if q.Behavior != "queue" || q.Count < 1 || q.Count > len(sh.Queue) || len(cfg.Guests) > 0 && len(cfg.Guests) != q.Count {
		return fmt.Errorf("invalid queue or guest count")
	}
	if cfg.Every < 0 || cfg.Dur < 0 || cfg.Distance < 0 {
		return fmt.Errorf("queue timing and distance must be nonnegative")
	}
	for i := 0; i < q.Count; i++ {
		kind := "c" + strconv.Itoa(i%6)
		if len(cfg.Guests) > 0 {
			kind = cfg.Guests[i]
		}
		if _, ok := sh.sprites.Clients[kind]; !ok {
			return fmt.Errorf("unknown queue client %q", kind)
		}
		sl := slots["queue"+strconv.Itoa(i+1)]
		p, err := sh.person("", kind, "stand", "calm", sl)
		if err != nil {
			return err
		}
		p.enters = sh.start + float64(i)*cfg.Every
		if cfg.Dur > 0 {
			dist := cfg.Distance
			if dist == 0 {
				dist = 120
			}
			p.pos[0] -= dist
			if err := p.move(p.enters, cfg.Dur, sl.pos); err != nil {
				return err
			}
		}
		sh.tl.AddActivity(p.enters, min(sh.end, p.enters+cfg.Dur))
		sh.people = append(sh.people, p)
	}
	return nil
}

func (sh *shot) speak(p *person, e *action) error {
	if p == nil || p.client {
		return fmt.Errorf("speaking requires a named character")
	}
	e.phase = sh.start
	if e.State != "" {
		if err := p.prepareEmotion(e.State); err != nil {
			return err
		}
	}
	if err := p.prepareEmotion("talk"); err != nil {
		return err
	}
	p.marks = append(p.marks, e)
	sh.tl.AddActivity(max(sh.start, e.at), min(sh.end, e.end))
	return nil
}

func (sh *shot) prepare(e *action) error {
	var p *person
	if e.Target != nil {
		p = sh.byID[e.Target.ID]
		if p == nil {
			return fmt.Errorf("target must be a character id")
		}
	}
	if e.Phrase != 0 {
		if e.Phrase < 1 || e.Phrase > len(sh.sc.Phrases) {
			return fmt.Errorf("unknown phrase %d", e.Phrase)
		}
		value := caption(sh.sc.Phrases[e.Phrase-1])
		switch e.Do {
		case "plate":
			e.Sub = value
		case "say":
			e.Text = value
		default:
			return fmt.Errorf("phrase needs plate or say")
		}
	}
	if e.Dur < 0 {
		return fmt.Errorf("negative duration")
	}
	switch e.Do {
	case "keys", "letters":
		capacity := len(sh.Hooks)
		if e.Do == "letters" {
			capacity = len(sh.Holes)
		}
		if capacity == 0 || e.Count < 0 || e.Count > capacity {
			return fmt.Errorf("invalid count %d for %s", e.Count, sh.place)
		}
		sh.play("click", e.at, e.end)
	case "door":
		if sh.Open == "" || e.State != "open" && e.State != "closed" {
			return fmt.Errorf("door needs open or closed at entrance or vault")
		}
	case "headline":
		if sh.place != "kiosk" || e.State != "fresh" && e.State != "stale" {
			return fmt.Errorf("headline needs fresh or stale at kiosk")
		}
	case "log":
		if e.Row < 1 || e.Row > len(sh.Cards) {
			return fmt.Errorf("invalid archive row %d", e.Row)
		}
	case "reveal":
		if sh.board == nil {
			return fmt.Errorf("reveal needs case")
		}
		if e.Dur == 0 {
			e.Dur = .45
		}
		if e.Every == 0 {
			e.Every = e.Dur
		}
		if e.Every < e.Dur {
			return fmt.Errorf("every must cover one yarn duration")
		}
	case "plate":
		if e.Title == "" && e.Sub == "" {
			return fmt.Errorf("empty plate")
		}
	case "sfx":
		c, ok := audio.NoirCue(e.Name)
		if !ok {
			return fmt.Errorf("unknown noir sound %q", e.Name)
		}
		if e.Gain != nil {
			if *e.Gain < 0 {
				return fmt.Errorf("negative sound gain")
			}
			c.Gain *= *e.Gain
		}
		end := e.end
		if e.Dur > 0 {
			end = min(end, e.at+e.Dur)
		}
		sh.addSound(c, e.at, end)
	case "pose", "emotion":
		if p == nil {
			return fmt.Errorf("target required")
		}
		var err error
		if e.Do == "pose" {
			err = p.preparePose(e.State)
		} else {
			err = p.prepareEmotion(e.State)
		}
		if err != nil {
			return err
		}
		p.marks = append(p.marks, e)
	case "say", "speak":
		if e.Do == "say" {
			if e.Text == "" {
				return fmt.Errorf("empty speech")
			}
			e.Text = strings.ReplaceAll(e.Text, "\r\n", "\n")
			if strings.Count(e.Text, "\n") > 1 {
				return fmt.Errorf("speech bubble accepts at most two lines")
			}
			if len(e.Box) != 0 && (len(e.Box) != 4 || e.Box[0] < 0 || e.Box[1] < 0 || e.Box[2] > render.W || e.Box[3] > render.H || e.Box[2]-e.Box[0] < 40 || e.Box[3]-e.Box[1] < 40) {
				return fmt.Errorf("invalid speech box")
			}
		}
		if e.Do == "speak" || p != nil {
			return sh.speak(p, e)
		}
	case "walk":
		if p == nil {
			return fmt.Errorf("target required")
		}
		sh.tl.AddActivity(e.at, min(e.at+e.Dur, e.end))
		dur := e.Dur
		if e.Until != nil {
			dur = min(dur, e.end-e.at)
		}
		return p.move(e.at, dur, point(e.To.Coords))
	case "icon", "prop", "flash":
		key := "props/" + e.Kind + ".png"
		if e.Do == "icon" {
			if p == nil {
				return fmt.Errorf("icon needs target")
			}
			key = "icons/" + e.Kind + ".png"
		} else if len(e.Pos) != 2 {
			return fmt.Errorf("prop needs pos")
		}
		if e.Do == "flash" || e.Kind == "flash" {
			key = "props/flash.png"
			e.end = min(e.end, e.at+3./render.FPS)
			sh.play("shutter", e.at, sh.end)
		}
		if err := sh.load(key); err != nil {
			return err
		}
		im := sh.images[key]
		if e.Kind == "watch" {
			var err error
			im, err = sh.watch(im, e.Time)
			if err != nil {
				return err
			}
		}
		if e.Size == 0 {
			e.Size = 1
		}
		if e.Size < .1 || e.Size > 4 {
			return fmt.Errorf("size wants 0.1..4")
		}
		e.image = resize(im, e.Size, false)
	case "neon":
		v := sh.Neon
		if e.Text != "" {
			v.Text = e.Text
		}
		if len(e.Pos) == 2 {
			v.At = point(e.Pos)
			v.Vertical = e.Vertical
		}
		if e.Size != 0 {
			v.Px = e.Size
		}
		if e.Step != 0 {
			v.Step = e.Step
		}
		if sh.Neon.BehindBlinds && e.Pos == nil && e.Text == "" && e.Size == 0 {
			e.neon = sh.windowSign()
		} else {
			if v.Text == "" || v.At == (point{}) || v.Px < 1 || v.Px > 240 || v.Step < 0 || v.Step > 300 || len([]rune(v.Text)) > 32 {
				return fmt.Errorf("neon needs short text, pos and size")
			}
			e.neon = sh.makeNeon(v)
		}
		sh.neonSounds(e)
	case "zoom":
		sh.camera = append(sh.camera, e.Event)
		sh.tl.AddActivity(e.at, min(e.end, e.at+e.Dur))
	case "cut":
		if e.State != "" && e.State != "black" && e.State != "white" {
			return fmt.Errorf("cut wants black or white")
		}
		if e.Dur == 0 {
			e.Dur = .3
		}
		sh.cuts = append(sh.cuts, e)
		sh.tl.AddActivity(max(0., e.at-e.Dur/2), min(sh.tl.Duration, e.at+e.Dur/2))
		sh.play("shutter", e.at, sh.end)
	default:
		return fmt.Errorf("unknown event")
	}
	return nil
}

func caption(p scene.Phrase) string {
	text := p.Caption
	if text == "" {
		text = p.Text
	}
	return strings.ReplaceAll(text, "\u0301", "")
}
