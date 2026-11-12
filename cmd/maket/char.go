package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"maket/internal/charkit"
	"maket/internal/render"
	"maket/internal/style/pixel"
)

var validName = regexp.MustCompile(`^[a-z0-9_-]+$`)

func charMain(cmd string, args []string) int {
	var pos []string
	for len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		pos, args = append(pos, args[0]), args[1:]
	}
	fs := flag.NewFlagSet("char "+cmd, flag.ExitOnError)
	root := fs.String("root", ".", "repository root with assets/")
	look := fs.String("look", "", "look folder in assets/looks/, default assets/live")
	out := fs.String("o", "", "output directory or file")
	frame := fs.Int("i", -1, "only this frame")
	name := fs.String("name", "", "character name in the catalog")
	label := fs.String("label", "", "what the character is, in Russian")
	alias := fs.String("alias", "", "other words for it, comma separated")
	refs := fs.String("ref", "", "designer characters to compare with, comma separated")
	fs.Parse(args)
	dir := charkit.Dir(*root, *look)
	var err error
	switch {
	case cmd == "list" && len(pos) == 0:
		err = charList(dir)
	case cmd == "grid" && (len(pos) == 1 || len(pos) == 2):
		state := ""
		if len(pos) == 2 {
			state = pos[1]
		}
		err = charGrid(dir, pos[0], state, *frame)
	case cmd == "build" && len(pos) == 1:
		err = charBuild(pos[0], *out)
	case cmd == "check" && len(pos) == 1:
		err = charCheck(dir, pos[0])
	case cmd == "sheet" && len(pos) == 1:
		err = charSheet(dir, pos[0], *out, *refs)
	case cmd == "add" && len(pos) == 1:
		err = charAdd(dir, pos[0], *name, *label, *alias)
	default:
		fmt.Fprintln(os.Stderr, usage)
		return 2
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "maket:", err)
		return 1
	}
	return 0
}

func charList(dir string) error {
	c, err := charkit.Load(dir)
	if err != nil {
		return err
	}
	for _, name := range c.Names() {
		e := c.Entries[name]
		fmt.Printf("%s | %s | %s | %s | %s\n", name, e.Label, strings.Join(e.Aliases, ", "), strings.Join(e.StateNames(), " "), e.Origin)
	}
	return nil
}

func charGrid(dir, name, state string, frame int) error {
	c, err := charkit.Load(dir)
	if err != nil {
		return err
	}
	s, err := charkit.GridFile(c, name, state, frame)
	if err != nil {
		return err
	}
	fmt.Print(s)
	return nil
}

func charBuild(gridPath, draft string) error {
	if draft == "" {
		return fmt.Errorf("char build: -o is required")
	}
	f, err := os.Open(gridPath)
	if err != nil {
		return err
	}
	g, err := charkit.ParseGrid(f)
	f.Close()
	if err != nil {
		return fmt.Errorf("%s: %w", gridPath, err)
	}
	if err := g.Write(draft); err != nil {
		return err
	}
	src, err := os.ReadFile(gridPath)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(draft, "grid.txt"), src, 0o644); err != nil {
		return err
	}
	fmt.Println(draft)
	return nil
}

func draftOrCatalog(c *charkit.Catalog, arg string) (charkit.Entry, string, error) {
	if b, err := os.ReadFile(filepath.Join(arg, "entry.json")); err == nil {
		var e charkit.Entry
		if err := json.Unmarshal(b, &e); err != nil {
			return e, "", fmt.Errorf("%s/entry.json: %w", arg, err)
		}
		return e, arg, nil
	}
	e, ok := c.Entries[arg]
	if !ok {
		return e, "", fmt.Errorf("%s is neither a draft with entry.json nor a character in %s", arg, c.Dir)
	}
	return e, filepath.Join(c.Dir, arg), nil
}

func charIssues(dir, arg string) (*charkit.Catalog, charkit.Entry, string, []string, error) {
	c, err := charkit.Load(dir)
	if err != nil {
		return nil, charkit.Entry{}, "", nil, err
	}
	e, frames, err := draftOrCatalog(c, arg)
	if err != nil {
		return nil, e, "", nil, err
	}
	r, err := c.Rules()
	if err != nil {
		return nil, e, "", nil, err
	}
	return c, e, frames, charkit.Check(e, frames, r), nil
}

func charCheck(dir, arg string) error {
	_, _, _, bad, err := charIssues(dir, arg)
	if err != nil {
		return err
	}
	if len(bad) == 0 {
		fmt.Println("ok")
		return nil
	}
	for _, b := range bad {
		fmt.Println(b)
	}
	return fmt.Errorf("%s: %d problems", arg, len(bad))
}

func charSheet(dir, arg, out, refList string) error {
	if out == "" {
		return fmt.Errorf("char sheet: -o is required")
	}
	c, err := charkit.Load(dir)
	if err != nil {
		return err
	}
	e, frames, err := draftOrCatalog(c, arg)
	if err != nil {
		return err
	}
	var refs []string
	if refList != "" {
		refs = strings.Split(refList, ",")
	} else {
		for _, n := range c.Names() {
			if o := c.Entries[n].Origin; (o == "" || o == "designer") && len(refs) < 3 && n != arg {
				refs = append(refs, n)
			}
		}
	}
	roles, over, err := pixel.Colors(dir)
	if err != nil {
		return err
	}
	theme := render.PixelTheme(nil, nil, over)
	im, err := charkit.Sheet(e, frames, c, refs, []color.RGBA{roles["consumer"], roles["storage"]}, theme.BG)
	if err != nil {
		return err
	}
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	err = png.Encode(f, im)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		fmt.Println(out)
	}
	return err
}

func charAdd(dir, draft, name, label, alias string) error {
	if !validName.MatchString(name) || label == "" {
		return fmt.Errorf("char add: -name of lowercase letters, digits, - and _ and -label are required")
	}
	if _, err := os.Stat(filepath.Join(draft, "entry.json")); err != nil {
		return fmt.Errorf("char add: %s is not a draft: %w", draft, err)
	}
	c, e, _, bad, err := charIssues(dir, draft)
	if err != nil {
		return err
	}
	if old, ok := c.Entries[name]; ok && (old.Origin == "" || old.Origin == "designer") {
		return fmt.Errorf("char add: %s is a designer character, pick another name", name)
	}
	if len(bad) > 0 {
		for _, b := range bad {
			fmt.Println(b)
		}
		return fmt.Errorf("char add: %s does not pass check, nothing added", draft)
	}
	e.Label, e.Origin, e.Dir = label, "generated", name+"/"
	e.Aliases = nil
	for _, a := range strings.Split(alias, ",") {
		if a = strings.TrimSpace(a); a != "" {
			e.Aliases = append(e.Aliases, a)
		}
	}
	files := map[string][]byte{}
	for _, st := range e.StateNames() {
		for i := 0; i < e.States[st].Frames; i++ {
			f := fmt.Sprintf("%s_%d.png", st, i)
			if files[f], err = os.ReadFile(filepath.Join(draft, f)); err != nil {
				return err
			}
		}
	}
	if g, err := os.ReadFile(filepath.Join(draft, "grid.txt")); err == nil {
		files["grid.txt"] = g
	}
	dst := filepath.Join(c.Dir, name)
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	for f, b := range files {
		if err := os.WriteFile(filepath.Join(dst, f), b, 0o644); err != nil {
			return err
		}
	}
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if err := c.Put(name, b); err != nil {
		return err
	}
	fmt.Println(dst)
	return nil
}
