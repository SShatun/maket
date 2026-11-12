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
	Style    string   `json:"style"`
	Look     string   `json:"look"`
	Voice    string   `json:"voice"`
	Seed     int      `json:"seed"`
	Hold     float64  `json:"hold"`
	Guess    *Card    `json:"guess"`
	Answer   *Answer  `json:"answer"`
	Phrases  []Phrase `json:"phrases"`
	Entities []Entity `json:"entities"`
	Events   []Event  `json:"events"`
	Code     *Code    `json:"code"`
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
	At      At       `json:"at"`
	Until   *At      `json:"until"`
	Do      string   `json:"do"`
	Target  *Point   `json:"target"`
	Targets []Point  `json:"targets"`
	Into    []string `json:"into"`
	From    *Point   `json:"from"`
	To      *Point   `json:"to"`
	Token   string   `json:"token"`
	Label   string   `json:"label"`
	State   string   `json:"state"`
	Role    string   `json:"role"`
	Text    string   `json:"text"`
	Sub     string   `json:"sub"`
	Dur     float64  `json:"dur"`
	Zoom    float64  `json:"zoom"`
	Dx      float64  `json:"dx"`
	Consume bool     `json:"consume"`
	Hold    bool     `json:"hold"`

	Arc     *float64 `json:"arc"`
	Waiting bool     `json:"waiting"`
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
	ids := map[string]bool{}
	hasAppear := map[string]bool{}
	for _, e := range sc.Entities {
		if e.ID == "" {
			return errors.New("entity without id")
		}
		ids[e.ID] = true
		hasAppear[e.ID] = e.Appear != nil
	}
	for i, ev := range sc.Events {
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
	return strings.ReplaceAll(strings.ToLower(w), "ё", "е")
}
