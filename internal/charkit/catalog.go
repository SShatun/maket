package charkit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type State struct {
	Frames int     `json:"frames"`
	FPS    float64 `json:"fps"`
	Loop   bool    `json:"loop"`
}

type Entry struct {
	Label   string           `json:"label,omitempty"`
	Aliases []string         `json:"aliases,omitempty"`
	Origin  string           `json:"origin,omitempty"`
	Dir     string           `json:"dir,omitempty"`
	Size    [2]int           `json:"size"`
	Anchor  [2]int           `json:"anchor"`
	Top     [2]int           `json:"top"`
	Pixel   int              `json:"pixel,omitempty"`
	Rest    string           `json:"rest"`
	Tint    *bool            `json:"tint,omitempty"`
	States  map[string]State `json:"states"`
}

func (e Entry) StateNames() []string {
	names := make([]string, 0, len(e.States))
	for s := range e.States {
		names = append(names, s)
	}
	sort.Strings(names)
	return names
}

type Catalog struct {
	Dir     string
	Entries map[string]Entry
	tree    *node
}

func Dir(root, look string) string {
	if look == "" {
		return filepath.Join(root, "assets", "live")
	}
	return filepath.Join(root, "assets", "looks", look)
}

func Load(dir string) (*Catalog, error) {
	b, err := os.ReadFile(filepath.Join(dir, "sprites.json"))
	if err != nil {
		return nil, err
	}
	tree, err := parse(b)
	if err != nil {
		return nil, fmt.Errorf("sprites.json: %w", err)
	}
	var f struct {
		Sprites map[string]Entry `json:"sprites"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("sprites.json: %w", err)
	}
	return &Catalog{Dir: dir, Entries: f.Sprites, tree: tree}, nil
}

func (c *Catalog) Names() []string {
	sp := c.tree.get("sprites")
	if sp == nil {
		return nil
	}
	return append([]string(nil), sp.keys...)
}

func (c *Catalog) Put(name string, entry json.RawMessage) error {
	n, err := parse(entry)
	if err != nil {
		return fmt.Errorf("entry %s: %w", name, err)
	}
	var e Entry
	if err := json.Unmarshal(entry, &e); err != nil {
		return fmt.Errorf("entry %s: %w", name, err)
	}
	sp := c.tree.get("sprites")
	if sp == nil {
		return fmt.Errorf("sprites.json: no sprites")
	}
	sp.set(name, quote(name), n)
	path := filepath.Join(c.Dir, "sprites.json")
	old, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	c.tree.write(&buf, 0)
	if bytes.HasSuffix(old, []byte("\n")) {
		buf.WriteByte('\n')
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	c.Entries[name] = e
	return nil
}

type node struct {
	kind     byte
	raw      string
	keys     []string
	rawKeys  map[string]string
	children map[string]*node
	items    []*node
}

func (n *node) get(key string) *node {
	if n == nil || n.kind != '{' {
		return nil
	}
	return n.children[key]
}

func (n *node) set(key, raw string, v *node) {
	if _, ok := n.children[key]; !ok {
		n.keys = append(n.keys, key)
		n.rawKeys[key] = raw
	}
	n.children[key] = v
}

func parse(b []byte) (*node, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	n, err := readNode(dec, b)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("trailing data")
	}
	return n, nil
}

func rawToken(dec *json.Decoder, b []byte) (json.Token, string, error) {
	from := dec.InputOffset()
	tok, err := dec.Token()
	if err != nil {
		return nil, "", err
	}
	return tok, strings.TrimLeft(string(b[from:dec.InputOffset()]), " \t\r\n,:"), nil
}

func readNode(dec *json.Decoder, b []byte) (*node, error) {
	tok, raw, err := rawToken(dec, b)
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			n := &node{kind: '{', rawKeys: map[string]string{}, children: map[string]*node{}}
			for dec.More() {
				kt, kraw, err := rawToken(dec, b)
				if err != nil {
					return nil, err
				}
				v, err := readNode(dec, b)
				if err != nil {
					return nil, err
				}
				n.set(kt.(string), kraw, v)
			}
			_, err := dec.Token()
			return n, err
		case '[':
			n := &node{kind: '['}
			for dec.More() {
				v, err := readNode(dec, b)
				if err != nil {
					return nil, err
				}
				n.items = append(n.items, v)
			}
			_, err := dec.Token()
			return n, err
		}
		return nil, fmt.Errorf("unexpected %v", t)
	}
	return &node{kind: '0', raw: raw}, nil
}

func quote(s string) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.Encode(s)
	return strings.TrimSuffix(buf.String(), "\n")
}

func (n *node) write(w *bytes.Buffer, depth int) {
	pad := strings.Repeat(" ", depth+1)
	switch n.kind {
	case '{':
		if len(n.keys) == 0 {
			w.WriteString("{}")
			return
		}
		w.WriteString("{\n")
		for i, k := range n.keys {
			w.WriteString(pad)
			w.WriteString(n.rawKeys[k])
			w.WriteString(": ")
			n.children[k].write(w, depth+1)
			if i < len(n.keys)-1 {
				w.WriteByte(',')
			}
			w.WriteByte('\n')
		}
		w.WriteString(strings.Repeat(" ", depth))
		w.WriteByte('}')
	case '[':
		if len(n.items) == 0 {
			w.WriteString("[]")
			return
		}
		w.WriteString("[\n")
		for i, v := range n.items {
			w.WriteString(pad)
			v.write(w, depth+1)
			if i < len(n.items)-1 {
				w.WriteByte(',')
			}
			w.WriteByte('\n')
		}
		w.WriteString(strings.Repeat(" ", depth))
		w.WriteByte(']')
	default:
		w.WriteString(n.raw)
	}
}
