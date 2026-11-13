package scene

import "fmt"

type Diagram struct {
	Nodes []DiagramNode `json:"nodes"`
	Links []DiagramLink `json:"links"`
	Steps []DiagramStep `json:"steps"`
	Axis  *DiagramAxis  `json:"axis"`
}

type DiagramNode struct {
	ID    string    `json:"id"`
	Title string    `json:"title"`
	Box   []float64 `json:"box"`
}

type DiagramLink struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type DiagramStep struct {
	At     At     `json:"at"`
	Target string `json:"target"`
	Result string `json:"result"`
	Tick   string `json:"tick"`
}

type DiagramAxis struct {
	At    []float64 `json:"at"`
	Width float64   `json:"width"`
}

func (d Diagram) check() error {
	if len(d.Nodes) == 0 || len(d.Steps) == 0 {
		return fmt.Errorf("needs nodes and steps")
	}
	nodes := map[string]bool{}
	for i, n := range d.Nodes {
		if n.ID == "" || nodes[n.ID] || n.Title == "" {
			return fmt.Errorf("node %d needs a unique id and title", i+1)
		}
		if len(n.Box) != 4 || n.Box[0] < 0 || n.Box[1] < 140 || n.Box[2] > 1080 || n.Box[3] > 1640 || n.Box[2]-n.Box[0] < 80 || n.Box[3]-n.Box[1] < 100 {
			return fmt.Errorf("node %s needs box: [x0, y0, x1, y1], at least 80 by 100, inside x 0..1080, y 140..1640", n.ID)
		}
		nodes[n.ID] = true
	}
	for i, l := range d.Links {
		if !nodes[l.From] || !nodes[l.To] || l.From == l.To {
			return fmt.Errorf("link %d needs two different node ids", i+1)
		}
	}
	for i, s := range d.Steps {
		if !nodes[s.Target] {
			return fmt.Errorf("step %d: unknown target %q", i+1, s.Target)
		}
		if i == 0 || d.Steps[i-1].Target == s.Target {
			continue
		}
		linked := false
		for _, l := range d.Links {
			if l.From == d.Steps[i-1].Target && l.To == s.Target {
				linked = true
			}
		}
		if !linked {
			return fmt.Errorf("step %d: no link from %s to %s", i+1, d.Steps[i-1].Target, s.Target)
		}
	}
	if a := d.Axis; a != nil {
		if len(a.At) != 2 || a.Width <= 0 || a.At[0] < 40 || a.At[0]+a.Width > 1040 || a.At[1] < 200 || a.At[1] > 1540 {
			return fmt.Errorf("axis needs at: [x, y] and positive width, inside x 40..1040, y 200..1540")
		}
	}
	return nil
}
