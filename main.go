// Command jui is an interactive, ncurses-style picker for the just command
// runner: browse, filter, and inspect recipes before running one.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"

	"jui/internal/justfile"
	"jui/internal/ui"
)

const version = "0.1.0"

func main() {
	var (
		dir     string
		showVer bool
	)
	flag.StringVar(&dir, "d", "", "directory containing the justfile (default: search upward from cwd)")
	flag.StringVar(&dir, "dir", "", "directory containing the justfile (default: search upward from cwd)")
	flag.BoolVar(&showVer, "version", false, "print version and exit")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "jui — an interactive picker for just\n\nUsage:\n  jui [-d dir]\n\nOptions:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if showVer {
		fmt.Println("jui " + version)
		return
	}

	if _, err := exec.LookPath("just"); err != nil {
		fmt.Fprintln(os.Stderr, "jui: `just` was not found on PATH; install it from https://github.com/casey/just")
		os.Exit(1)
	}

	dump, err := justfile.Load(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "jui: "+err.Error())
		os.Exit(1)
	}
	if len(dump.Recipes) == 0 {
		fmt.Fprintln(os.Stderr, "jui: no recipes found")
		os.Exit(1)
	}

	m := ui.New(dump)
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "jui: "+err.Error())
		os.Exit(1)
	}

	result := final.(*ui.Model).Result
	if !result.Ran || result.Recipe == nil {
		return
	}

	runRecipe(dir, result.Recipe.Name, result.Args)
}

// runRecipe replaces the current process with `just <recipe> <args...>` so
// stdio, the tty, signals, and the exit code all behave exactly as if the
// user had typed the command themselves.
func runRecipe(dir, recipe string, args []string) {
	justPath, err := exec.LookPath("just")
	if err != nil {
		fmt.Fprintln(os.Stderr, "jui: "+err.Error())
		os.Exit(1)
	}
	if dir != "" {
		if err := os.Chdir(dir); err != nil {
			fmt.Fprintln(os.Stderr, "jui: "+err.Error())
			os.Exit(1)
		}
	}
	argv := append([]string{"just", recipe}, args...)
	if err := syscall.Exec(justPath, argv, os.Environ()); err != nil {
		fmt.Fprintln(os.Stderr, "jui: exec failed: "+err.Error())
		os.Exit(1)
	}
}
