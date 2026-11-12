package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"maket/internal/charkit"
)

func charMain(cmd string, args []string) int {
	fs := flag.NewFlagSet("char "+cmd, flag.ExitOnError)
	root := fs.String("root", ".", "repository root with assets/")
	look := fs.String("look", "", "look folder in assets/looks/, default assets/live")
	fs.Parse(args)
	var err error
	switch cmd {
	case "list":
		err = charList(charkit.Dir(*root, *look))
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
