package scene

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

type Scene struct {
	Style    string          `json:"style"`
	Look     string          `json:"look"`
	Voice    string          `json:"voice"`
	Seed     int             `json:"seed"`
	Hold     float64         `json:"hold"`
	Guess    *Card           `json:"guess"`
	Answer   *Answer         `json:"answer"`
	Phrases  []Phrase        `json:"phrases"`
	Entities []Entity        `json:"entities"`
	Events   []Event         `json:"events"`
	Shots    []Shot          `json:"shots"`
	Code     *Code           `json:"code"`
	Caption  json.RawMessage `json:"caption"`
}

type Shot struct {
	At    At              `json:"at"`
	Place Place           `json:"place"`
	Cast  []Cast          `json:"cast"`
	Do    []Event         `json:"do"`
	Raw   json.RawMessage `json:"-"`
}

type Place struct {
	Name   string   `json:"-"`
	Wall   Color    `json:"wall"`
	Floor  *float64 `json:"floor"`
	Ground Color    `json:"ground"`
}

type Color struct {
	Name string
	RGB  []uint8
}

type Cast struct {
	Entity
	Size  float64         `json:"size"`
	State string          `json:"state"`
	Crowd *Crowd          `json:"crowd"`
	Raw   json.RawMessage `json:"-"`
}

type Crowd struct {
	Kind  string    `json:"kind"`
	Count int       `json:"count"`
	Box   []float64 `json:"box"`
	Size  []float64 `json:"size"`
	Rows  int       `json:"rows"`
	State string    `json:"state"`
}

type Phrase struct {
	Text    string            `json:"text"`
	Caption string            `json:"caption"`
	Pause   float64           `json:"pause"`
	Stress  map[string]string `json:"stress"`
}

type Card struct {
	Text string `json:"text"`
}

type Answer struct {
	At     At      `json:"at"`
	CardAt *At     `json:"card_at"`
	Text   string  `json:"text"`
	Badge  string  `json:"badge"`
	Size   float64 `json:"size"`
}

type Code struct {
	Lines     []CodeLine  `json:"lines"`
	Highlight []Highlight `json:"highlight"`
}

type CodeLine struct {
	Text   string
	Appear *At
}

type PanelLine struct {
	Text   string `json:"text"`
	Appear *At    `json:"appear"`
	Tone   string `json:"tone"`
}

type Highlight struct {
	At   At
	Line int
	Tone string
}

type Entity struct {
	ID       string          `json:"id"`
	Kind     string          `json:"kind"`
	Role     string          `json:"role"`
	Roles    []Timed[string] `json:"roles"`
	Label    string          `json:"label"`
	At       []float64       `json:"at"`
	Appear   *At             `json:"appear"`
	Status   []Timed[string] `json:"status"`
	States   []Timed[string] `json:"states"`
	Badges   []Timed[string] `json:"badges"`
	Count    int             `json:"count"`
	Spacing  float64         `json:"spacing"`
	LabelPos string          `json:"label_pos"`
	FullWord *bool           `json:"full_word"`
}

type Event struct {
	At      At          `json:"at"`
	Until   *At         `json:"until"`
	Do      string      `json:"do"`
	Target  *Point      `json:"target"`
	Targets []Point     `json:"targets"`
	Into    []string    `json:"into"`
	From    *Point      `json:"from"`
	To      *Point      `json:"to"`
	Token   string      `json:"token"`
	Label   string      `json:"label"`
	State   string      `json:"state"`
	Role    string      `json:"role"`
	Text    string      `json:"text"`
	Sub     string      `json:"sub"`
	Dur     float64     `json:"dur"`
	Zoom    float64     `json:"zoom"`
	Dx      float64     `json:"dx"`
	Pos     []float64   `json:"pos"`
	Tone    string      `json:"tone"`
	Consume bool        `json:"consume"`
	Hold    bool        `json:"hold"`
	Kind    string      `json:"kind"`
	Size    float64     `json:"size"`
	Box     []float64   `json:"box"`
	Title   string      `json:"title"`
	Lines   []PanelLine `json:"lines"`
	Fill    string      `json:"fill"`

	Arc     *float64        `json:"arc"`
	Waiting bool            `json:"waiting"`
	Raw     json.RawMessage `json:"-"`
}

type At struct {
	T      *float64
	Phrase int
	Word   string
	Nth    int
	End    bool
	Offset float64
}

type Point struct {
	ID     string
	Slot   int
	Coords []float64
	Value  *float64
}

type Timed[T any] struct {
	At   At
	V    T
	Hold bool
}

func Load(path string) (*Scene, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var sc Scene
	if err := json.Unmarshal(b, &sc); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if sc.Seed == 0 {
		sc.Seed = 42
	}
	if sc.Hold == 0 {
		sc.Hold = 1.5
	}
	if err := sc.check(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &sc, nil
}

func (sc *Scene) check() error {
	if len(sc.Shots) > 0 {
		if len(sc.Entities) > 0 || len(sc.Events) > 0 {
			return errors.New("shots use cast and do instead of entities and events")
		}
		for i, s := range sc.Shots {
			if err := s.check(sc.Style == "" || sc.Style == "pixel"); err != nil {
				return fmt.Errorf("shot %d: %w", i+1, err)
			}
		}
	}
	return checkEvents(sc.Entities, sc.Events)
}

func (s Shot) check(pixel bool) error {
	wall := s.Place.Wall.Name != "" || len(s.Place.Wall.RGB) == 3
	ground := s.Place.Ground.Name != "" || len(s.Place.Ground.RGB) == 3
	if pixel && s.Place.Name == "" && (!wall || s.Place.Floor != nil && (!ground || *s.Place.Floor < 600 || *s.Place.Floor > 1900)) {
		return errors.New("place needs a backdrop name or wall and ground as [r, g, b] or palette names, floor: 600..1900 or null")
	}
	ids := map[string]bool{}
	var entities []Entity
	for i, c := range s.Cast {
		if c.Crowd != nil {
			x := c.Crowd
			if pixel && (x.Kind == "" || x.Count < 1 || x.Rows < 1 || x.Rows > x.Count || len(x.Box) != 4 || x.Box[0] >= x.Box[2] || x.Box[1] >= x.Box[3] || len(x.Size) != 2 || x.Size[0] < 0.2 || x.Size[1] > 4 || x.Size[0] > x.Size[1]) {
				return fmt.Errorf("cast %d: crowd needs kind, count, rows, box: [x0, y0, x1, y1] and size: [min, max] in 0.2..4", i+1)
			}
			if c.ID != "" {
				if ids[c.ID] {
					return fmt.Errorf("cast %d: duplicate id %q", i+1, c.ID)
				}
				ids[c.ID] = true
				entities = append(entities, c.Entity)
			}
			continue
		}
		if c.ID == "" || ids[c.ID] {
			return fmt.Errorf("cast %d: missing or duplicate id %q", i+1, c.ID)
		}
		if c.Kind == "" || len(c.At) != 2 || c.Size != 0 && (c.Size < 0.2 || c.Size > 4) {
			return fmt.Errorf("cast %s: needs kind, at: [x, y] and size in 0.2..4", c.ID)
		}
		if c.Appear != nil {
			return fmt.Errorf("cast %s: appears at the shot start", c.ID)
		}
		ids[c.ID] = true
		entities = append(entities, c.Entity)
	}
	return checkEvents(entities, s.Do)
}

func checkEvents(entities []Entity, events []Event) error {
	ids := map[string]bool{}
	hasAppear := map[string]bool{}
	for _, e := range entities {
		if e.ID == "" {
			return errors.New("entity without id")
		}
		ids[e.ID] = true
		hasAppear[e.ID] = e.Appear != nil
	}
	for i, ev := range events {
		if ev.Pos != nil && len(ev.Pos) != 2 {
			return fmt.Errorf("event %d: pos wants [x, y] in px", i+1)
		}
		if ev.Do == "number" && ev.Tone != "" && ev.Tone != "accent" && ev.Tone != "ok" && ev.Tone != "error" {
			return fmt.Errorf("event %d: unknown number tone %q", i+1, ev.Tone)
		}
		if ev.Do == "line" || ev.Do == "text" || ev.Do == "panel" {
			if !elementTone(ev.Tone) {
				return fmt.Errorf("event %d: unknown %s tone %q", i+1, ev.Do, ev.Tone)
			}
			switch ev.Do {
			case "line":
				if ev.From == nil || len(ev.From.Coords) != 2 || ev.To == nil || len(ev.To.Coords) != 2 {
					return fmt.Errorf("event %d: line needs from and to: [x, y]", i+1)
				}
				if ev.Kind != "" && ev.Kind != "solid" && ev.Kind != "dash" && ev.Kind != "arrow" {
					return fmt.Errorf("event %d: unknown line kind %q", i+1, ev.Kind)
				}
			case "text":
				if len(ev.Pos) != 2 || ev.Size < 0 {
					return fmt.Errorf("event %d: text needs pos: [x, y] and positive size", i+1)
				}
			case "panel":
				if len(ev.Box) != 4 || ev.Box[0] >= ev.Box[2] || ev.Box[1] >= ev.Box[3] {
					return fmt.Errorf("event %d: panel needs box: [x0, y0, x1, y1]", i+1)
				}
				if ev.Fill != "" && ev.Fill != "card" && ev.Fill != "accent" && ev.Fill != "error" {
					return fmt.Errorf("event %d: unknown panel fill %q", i+1, ev.Fill)
				}
				for j, l := range ev.Lines {
					if !elementTone(l.Tone) {
						return fmt.Errorf("event %d line %d: unknown panel tone %q", i+1, j+1, l.Tone)
					}
				}
			}
		}
		if ev.Do == "zoom" && (ev.To == nil || ev.To.Value == nil || *ev.To.Value < 1 || ev.Dur <= 0) {
			return fmt.Errorf("event %d: zoom needs to >= 1 and positive dur", i+1)
		}
		if ev.Do == "walk" && (ev.Target == nil || ev.Target.ID == "" || ev.To == nil || len(ev.To.Coords) != 2 || ev.Dur <= 0) {
			return fmt.Errorf("event %d: walk needs target, to: [x, y] and positive dur", i+1)
		}
		if ev.Do != "zoom" && ev.To != nil && ev.To.Value != nil {
			return fmt.Errorf("event %d: numeric to needs zoom", i+1)
		}
		pts := append([]Point{}, ev.Targets...)
		for _, p := range []*Point{ev.Target, ev.From, ev.To} {
			if p != nil {
				pts = append(pts, *p)
			}
		}
		for _, p := range pts {
			if p.ID != "" && !ids[p.ID] {
				return fmt.Errorf("event %d: unknown entity %q", i+1, p.ID)
			}
		}
		for _, id := range ev.Into {
			if !ids[id] {
				return fmt.Errorf("event %d: unknown entity %q", i+1, id)
			}
			if ev.Do == "split" && hasAppear[id] {
				return fmt.Errorf("event %d: split half %q must not have appear", i+1, id)
			}
		}
	}
	return nil
}

func elementTone(tone string) bool {
	return tone == "" || tone == "ink" || tone == "accent" || tone == "ok" || tone == "error"
}

func plainTone(tone string) string {
	switch tone {
	case "hi":
		return "accent"
	case "bad":
		return "error"
	}
	return tone
}

func (s *Shot) UnmarshalJSON(b []byte) error {
	type shot Shot
	if err := json.Unmarshal(b, (*shot)(s)); err != nil {
		return err
	}
	s.Raw = append(json.RawMessage(nil), b...)
	return nil
}

func (c *Cast) UnmarshalJSON(b []byte) error {
	type cast Cast
	if err := json.Unmarshal(b, (*cast)(c)); err != nil {
		return err
	}
	c.Raw = append(json.RawMessage(nil), b...)
	return nil
}

func (e *Event) UnmarshalJSON(b []byte) error {
	type event Event
	if err := json.Unmarshal(b, (*event)(e)); err != nil {
		return err
	}
	e.Raw = append(json.RawMessage(nil), b...)
	return nil
}

func (l *PanelLine) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, &l.Text); err == nil {
		return nil
	}
	var pair []json.RawMessage
	if err := json.Unmarshal(b, &pair); err == nil {
		if len(pair) < 1 || len(pair) > 2 || json.Unmarshal(pair[0], &l.Text) != nil {
			return fmt.Errorf("panel line must be \"text\", [text, tone] or {text, tone, appear}: %s", b)
		}
		if len(pair) == 2 && string(pair[1]) != "null" {
			if err := json.Unmarshal(pair[1], &l.Tone); err != nil {
				return err
			}
		}
		l.Tone = plainTone(l.Tone)
		return nil
	}
	type line PanelLine
	if err := json.Unmarshal(b, (*line)(l)); err != nil {
		return err
	}
	l.Tone = plainTone(l.Tone)
	return nil
}

func (p *Place) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, &p.Name); err == nil && p.Name != "" {
		return nil
	}
	type place Place
	return json.Unmarshal(b, (*place)(p))
}

func (c *Color) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, &c.Name); err == nil && c.Name != "" {
		return nil
	}
	return json.Unmarshal(b, &c.RGB)
}

func (a *At) UnmarshalJSON(b []byte) error {
	var f float64
	if err := json.Unmarshal(b, &f); err == nil {
		a.T = &f
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("moment must be seconds or \"p<N>:word\": %s", b)
	}
	return a.parse(s)
}

var momentRe = regexp.MustCompile(`^p(\d+)(\.end)?(?::([^#]+?)(?:#(\d+))?)?([+-]\d+(?:\.\d+)?)?$`)

func (a *At) parse(s string) error {
	m := momentRe.FindStringSubmatch(s)
	if m == nil {
		return fmt.Errorf("moment %q: want p<N>[.end] or p<N>:word[#k][+sec]", s)
	}
	a.Phrase, _ = strconv.Atoi(m[1])
	a.End = m[2] != ""
	a.Word = m[3]
	if a.Word != "" {
		a.Nth = 1
		if m[4] != "" {
			a.Nth, _ = strconv.Atoi(m[4])
		}
	}
	if m[5] != "" {
		a.Offset, _ = strconv.ParseFloat(m[5], 64)
	}
	if a.End && a.Word != "" {
		return fmt.Errorf("moment %q: .end is for a whole phrase", s)
	}
	return nil
}

func (a At) String() string {
	if a.T != nil {
		return strconv.FormatFloat(*a.T, 'f', -1, 64)
	}
	s := "p" + strconv.Itoa(a.Phrase)
	if a.End {
		s += ".end"
	}
	if a.Word != "" {
		s += ":" + a.Word
		if a.Nth > 1 {
			s += "#" + strconv.Itoa(a.Nth)
		}
	}
	if a.Offset != 0 {
		s += fmt.Sprintf("%+g", a.Offset)
	}
	return s
}

func (p *Point) UnmarshalJSON(b []byte) error {
	var f float64
	if string(b) != "null" && json.Unmarshal(b, &f) == nil {
		p.Value = &f
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		p.Slot = -1
		id, slot, ok := strings.Cut(s, "[")
		p.ID = id
		if ok {
			n, err := strconv.Atoi(strings.TrimSuffix(slot, "]"))
			if err != nil {
				return fmt.Errorf("point %q: bad slot", s)
			}
			p.Slot = n
		}
		return nil
	}
	if err := json.Unmarshal(b, &p.Coords); err != nil {
		return fmt.Errorf("point must be an id, \"id[k]\" or coordinates: %s", b)
	}
	return nil
}

func (t *Timed[T]) UnmarshalJSON(b []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil || len(raw) < 2 {
		return fmt.Errorf("timed value must be [moment, value]: %s", b)
	}
	if err := json.Unmarshal(raw[0], &t.At); err != nil {
		return err
	}
	if string(raw[1]) != "null" {
		if err := json.Unmarshal(raw[1], &t.V); err != nil {
			return err
		}
	}
	if len(raw) > 2 {
		var flag string
		if err := json.Unmarshal(raw[2], &flag); err == nil && flag == "hold" {
			t.Hold = true
		}
	}
	return nil
}

func (l *CodeLine) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, &l.Text); err == nil {
		return nil
	}
	var o struct {
		Text   string `json:"text"`
		Appear *At    `json:"appear"`
	}
	if err := json.Unmarshal(b, &o); err != nil {
		return err
	}
	l.Text, l.Appear = o.Text, o.Appear
	return nil
}

func (h *Highlight) UnmarshalJSON(b []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil || len(raw) != 3 {
		return fmt.Errorf("highlight must be [moment, line, tone]: %s", b)
	}
	if err := json.Unmarshal(raw[0], &h.At); err != nil {
		return err
	}
	if err := json.Unmarshal(raw[1], &h.Line); err != nil {
		return err
	}
	return json.Unmarshal(raw[2], &h.Tone)
}

func Words(text string) []string {
	var out []string
	for _, f := range strings.Fields(text) {
		w := strings.TrimFunc(f, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
		if w != "" {
			out = append(out, w)
		}
	}
	return out
}

func Fold(w string) string {
	return strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(w), "ё", "е"), "\u0301", "")
}
