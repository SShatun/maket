package noir

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"slices"
	"strconv"
	"sync"

	"github.com/fogleman/gg"
)

type slot struct {
	pos        point
	scale      float64
	bust, flip bool
}

func (p *location) slots() map[string]slot {
	m := map[string]slot{}
	for name, v := range map[string]portrait{"pool": p.Pool, "detective": p.Detective, "suspect": p.Suspect, "accountant": p.Accountant} {
		if v.Scale > 0 {
			m[name] = slot{v.Pos, v.Scale, true, false}
		}
	}
	for name, v := range map[string]slot{"clerk": {pos: p.Clerk, scale: .62}, "doorman": {pos: p.Doorman, scale: p.DoormanScale}, "newsboy": {pos: p.Newsboy, scale: p.NewsboyScale}, "buyer": {pos: p.Buyer, scale: .65, flip: true}} {
		if v.pos != (point{}) {
			m[name] = v
		}
	}
	for i, pos := range p.Workers {
		m["worker"+strconv.Itoa(i+1)] = slot{pos: pos, scale: p.WorkerScales[i]}
	}
	for i, pos := range p.Crowd {
		m["crowd"+strconv.Itoa(i+1)] = slot{pos: pos, scale: p.CrowdScale, flip: i%2 == 1}
	}
	for i, pos := range p.Clients {
		m["client"+strconv.Itoa(i+1)] = slot{pos: pos, scale: p.ClientScale, flip: i%2 == 1}
	}
	for i, pos := range p.Queue {
		k := p.QueueScale
		if i < len(p.QueueScales) {
			k = p.QueueScales[i]
		}
		m["queue"+strconv.Itoa(i+1)] = slot{pos: pos, scale: k, flip: p.QueueFlip || len(p.QueueScales) > 0}
	}
	return m
}

type appearance struct {
	pose, emotion string
	flip          bool
}

type movement struct {
	at, end  float64
	from, to point
}

func (m movement) position(t float64) point {
	u := clamp((t - m.at) / (m.end - m.at))
	return point{m.from[0] + (m.to[0]-m.from[0])*u, m.from[1] + (m.to[1]-m.from[1])*u}
}

type person struct {
	*art
	slot
	id, kind, dir     string
	initial           appearance
	client, atCounter bool
	meta              sprite
	enters            float64
	moves             []movement
	marks             []*action
	mu                sync.Mutex
	cache             map[appearance]*image.RGBA
}

func (a *art) person(id, kind, pose, emotion string, sl slot) (*person, error) {
	meta, ok := a.sprites.People[kind]
	dir := "people/" + kind + "/"
	client := false
	if !ok {
		meta, ok = a.sprites.Clients[kind]
		if !ok {
			return nil, fmt.Errorf("unknown person %q", kind)
		}
		if sl.bust {
			return nil, fmt.Errorf("%s has no bust", kind)
		}
		dir = "clients/" + kind + "/"
		client = true
	}
	if pose == "" {
		pose = "stand"
	}
	if emotion == "" {
		emotion = "calm"
	}
	p := &person{art: a, id: id, kind: kind, dir: dir, meta: meta, client: client, slot: sl, initial: appearance{pose, emotion, sl.flip}, cache: map[appearance]*image.RGBA{}}
	if err := p.preparePose(pose); err != nil {
		return nil, err
	}
	if err := p.prepareEmotion(emotion); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *person) preparePose(name string) error {
	if _, ok := p.meta.Head[name]; !ok || p.bust && name != "stand" {
		return fmt.Errorf("invalid pose %s/%s", p.kind, name)
	}
	if p.bust {
		return nil
	}
	return p.load(p.dir + "body_" + name + ".png")
}

func (p *person) prepareEmotion(name string) error {
	if !slices.Contains(p.sprites.Emotions, name) || p.client && name != "calm" {
		return fmt.Errorf("invalid emotion %s/%s", p.kind, name)
	}
	return p.load(p.face(name))
}

func (p *person) face(name string) string {
	if p.client {
		return p.dir + "head.png"
	}
	if p.bust {
		return p.dir + "bust_" + name + ".png"
	}
	return p.dir + "head_" + name + ".png"
}

func (p *person) move(at, dur float64, to point) error {
	if p.bust || dur <= 0 {
		return fmt.Errorf("%s: walk requires a full figure and positive dur", p.id)
	}
	for i := range 8 {
		if err := p.preparePose("walk_" + strconv.Itoa(i)); err != nil {
			return err
		}
	}
	from := p.pos
	if n := len(p.moves); n > 0 {
		from = p.moves[n-1].position(at)
	}
	p.moves = append(p.moves, movement{at, at + dur, from, to})
	return nil
}

func (p *person) image(a appearance) *image.RGBA {
	p.mu.Lock()
	defer p.mu.Unlock()
	if im := p.cache[a]; im != nil {
		return im
	}
	im := p.images[p.face(a.emotion)]
	if !p.bust {
		body := p.images[p.dir+"body_"+a.pose+".png"]
		out := image.NewRGBA(body.Rect)
		copy(out.Pix, body.Pix)
		paste(out, im, p.meta.Head[a.pose].pixel())
		im = out
	}
	im = resize(im, p.scale, a.flip)
	if len(p.cache) > 0 && (len(p.cache)+1)*len(im.Pix) > 16<<20 {
		clear(p.cache)
	}
	p.cache[a] = im
	return im
}

type figure struct {
	person     *person
	body       *image.RGBA
	origin     image.Point
	head, feet point
}

func (p *person) figure(t float64) figure {
	a, pos := p.initial, p.pos
	var speech *action
	for _, e := range p.marks {
		if t < e.at || t >= e.end {
			continue
		}
		switch e.Do {
		case "pose":
			a.pose = e.State
		case "emotion":
			a.emotion = e.State
		case "speak", "say":
			speech = e
		}
	}
	if speech != nil {
		if int((t-speech.phase)*6)%2 == 1 {
			a.emotion = "talk"
		} else if speech.State != "" {
			a.emotion = speech.State
		}
	}
	for i := len(p.moves) - 1; i >= 0; i-- {
		m := p.moves[i]
		if t < m.at {
			continue
		}
		pos = m.position(t)
		if m.from[0] != m.to[0] {
			a.flip = m.to[0] > m.from[0]
		}
		if t <= m.end+momentTolerance {
			a.pose = "walk_" + strconv.Itoa(int((t-m.at)*8)%8)
		}
		break
	}
	im := p.image(a)
	origin := pos
	head := point{pos[0] + float64(im.Rect.Dx())/2, pos[1]}
	if !p.bust {
		ax := p.sprites.Feet[0]
		if a.flip {
			ax = float64(p.sprites.Canvas[0]) - ax
		}
		origin = point{pos[0] - ax*p.scale, pos[1] - p.sprites.Feet[1]*p.scale}
		h := p.meta.Head[a.pose]
		h[0] += float64(p.images[p.face(a.emotion)].Rect.Dx()) / 2
		if a.flip {
			h[0] = float64(p.sprites.Canvas[0]) - h[0]
		}
		head = point{origin[0] + h[0]*p.scale, origin[1] + h[1]*p.scale}
	}
	if p.bust {
		pos[1] += float64(im.Rect.Dy())
	}
	return figure{p, im, origin.pixel(), head, pos}
}

func (f figure) shadow(dst *image.RGBA, wall bool) {
	if f.person.bust {
		if wall {
			im := resize(f.body, 1.12, false)
			off := f.origin.Add(image.Pt(260, f.body.Rect.Dy()-im.Rect.Dy()-40))
			draw.DrawMask(dst, im.Rect.Add(off), image.NewUniform(color.RGBA{8, 8, 12, 255}), image.Point{}, im, image.Point{}, draw.Over)
		}
		return
	}
	k := f.person.scale
	dc := gg.NewContextForRGBA(dst)
	dc.SetRGB255(10, 10, 14)
	dc.DrawEllipse(f.feet[0]+22.5*k, f.feet[1]+4*k, 172.5*k, 18*k)
	dc.Fill()
}

func (l *lamp) apply(layer *image.RGBA) {
	if l == nil {
		return
	}
	dx, dy := l.Dir[0]*l.Pixels, l.Dir[1]*l.Pixels
	for y := layer.Rect.Min.Y; y < layer.Rect.Max.Y; y++ {
		for x := layer.Rect.Min.X; x < layer.Rect.Max.X; x++ {
			i := layer.PixOffset(x, y)
			px := layer.Pix[i:][:4]
			if px[3] == 0 {
				continue
			}
			alpha := float64(px[3]) / 255
			m := float64(l.mask.GrayAt(x, y).Y) / 255
			other := layer.RGBAAt(x+dx, y+dy).A
			edge := math.Max(0, float64(px[3])-float64(other)) * m / 255
			light := (l.Floor + (1-l.Floor)*m) * (1 - edge)
			for j := range 3 {
				px[j] = uint8(float64(px[j])*l.Tint[j]*light + float64(l.Rim[j])*alpha*edge)
			}
		}
	}
}
