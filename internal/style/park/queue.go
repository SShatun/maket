package park

import (
	"fmt"
	"math"
	"slices"

	"maket/internal/audio"
)

const boardingDuration = .72

type admission struct {
	queue            *queue
	ride             *object
	left             int
	next, every, dur float64
}

func (sh *shot) advance(until float64) {
	for {
		var next *admission
		when := math.Inf(1)
		for _, req := range sh.pending {
			t := max(req.next, sh.clock)
			if req.left == 0 || len(req.queue.people) == 0 || t > until || t >= sh.end || t >= when {
				continue
			}
			r := req.ride
			if r.kind != "gate" && (r.occupied(t, true) >= r.seats || r.state(t) != "idle") {
				continue
			}
			next, when = req, t
		}
		if next == nil {
			for _, req := range sh.pending {
				if req.left > 0 && len(req.queue.people) > 0 {
					sh.tl.AddActivity(max(sh.clock, req.next), until)
				}
			}
			break
		}
		sh.board(next, when)
		next.left--
		next.next = when + next.every
		sh.clock = when
	}
	sh.pending = slices.DeleteFunc(sh.pending, func(a *admission) bool { return a.left == 0 || len(a.queue.people) == 0 })
	sh.clock = until
}

func (sh *shot) board(a *admission, t float64) {
	q, r := a.queue, a.ride
	p := q.people[0]
	q.people = q.people[1:]
	_, from := p.pose(t)
	gate := r.at
	if q.Gate != nil {
		gate = *q.Gate
	}
	entry := r.boarding
	dur := a.dur
	if dur == 0 {
		dur = boardingDuration
	}
	end := t + dur
	mid := t + dur*from.distance(gate)/max(1, from.distance(gate)+gate.distance(entry))
	if mid > t {
		p.move(t, mid, from, gate, "", false)
	}
	if end > mid {
		p.move(mid, end, gate, entry, "", false)
	}
	sh.signals = append(sh.signals, signal{t, gate})
	p.hidden = append(p.hidden, mark{end, sh.end, "aboard"})
	if r.kind != "gate" {
		r.trips = append(r.trips, &trip{p, t, end, sh.end})
	}
	if end < sh.end {
		sh.cue(end, audio.Click, 0)
	}
	for i, p := range q.people {
		slot := q.Slots[i]
		_, from := p.pose(t)
		p.move(t, t+.45, from, point{slot.X, slot.Y}, slot.View, slot.Flip)
	}
}

func (sh *shot) release(r *object, a action) error {
	if r == nil || r.seats == 0 || a.To == nil || len(a.To.Coords) != 2 {
		return fmt.Errorf("release needs ride target and exit point in to")
	}
	exit := point(a.To.Coords)
	for _, trip := range r.trips {
		if a.t < trip.boarded || a.t >= trip.end {
			continue
		}
		trip.end = a.t
		p := trip.person
		if p == nil {
			kind, err := sh.guestKind(sh.guests, nil, 0)
			if err != nil {
				return err
			}
			p, err = sh.newPerson(kind, r.at, "front", false)
			if err != nil {
				return err
			}
			p.hidden = append(p.hidden, mark{sh.at, a.t, "aboard"})
		} else {
			p.hidden[len(p.hidden)-1].end = a.t
		}
		start := r.boarding
		if a.From != nil {
			if len(a.From.Coords) != 2 {
				return fmt.Errorf("release from needs coordinates")
			}
			start = point(a.From.Coords)
		}
		dur := a.Dur
		if dur == 0 {
			dur = start.distance(exit) / 180
		}
		end := a.t + dur
		p.move(a.t, end, start, exit, "front", false)
		p.poses = append(p.poses, mark{end, end + .15, "happy"})
		p.hidden = append(p.hidden, mark{end + .15, sh.end, "left"})
		break
	}
	return nil
}
