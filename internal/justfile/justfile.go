// Package justfile loads recipe information from a justfile by shelling out
// to `just --dump --dump-format json`.
package justfile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

// Parameter is a single recipe parameter, as reported by `just --dump`.
type Parameter struct {
	Name       string `json:"name"`
	Default    string `json:"default"`
	Kind       string `json:"kind"` // "singular", "star" (*args), "plus" (+args)
	HasDefault bool   `json:"-"`
}

// UnmarshalJSON captures whether "default" was present (vs. null) so callers
// can tell "no default" apart from "default is empty string".
func (p *Parameter) UnmarshalJSON(data []byte) error {
	type raw struct {
		Name    string  `json:"name"`
		Default *string `json:"default"`
		Kind    string  `json:"kind"`
	}
	var r raw
	if err := json.Unmarshal(data, &r); err != nil {
		return err
	}
	p.Name = r.Name
	p.Kind = r.Kind
	if r.Default != nil {
		p.Default = *r.Default
		p.HasDefault = true
	}
	return nil
}

// Variadic reports whether the parameter collects the remaining arguments.
func (p Parameter) Variadic() bool {
	return p.Kind == "star" || p.Kind == "plus"
}

// Signature renders the parameter the way it would appear in the justfile
// source, e.g. "env=\"staging\"" or "*args".
func (p Parameter) Signature() string {
	switch p.Kind {
	case "star":
		return "*" + p.Name
	case "plus":
		return "+" + p.Name
	default:
		if p.HasDefault {
			return fmt.Sprintf("%s=%q", p.Name, p.Default)
		}
		return p.Name
	}
}

// Recipe describes a single just recipe.
type Recipe struct {
	Name          string       `json:"name"`
	Doc           *string      `json:"doc"`
	Parameters    []Parameter  `json:"parameters"`
	Dependencies  []Dependency `json:"dependencies"`
	Private       bool         `json:"private"`
	Quiet         bool         `json:"quiet"`
	attributesRaw []json.RawMessage
	Body          [][]json.RawMessage `json:"body"`

	Group string   `json:"-"`
	Attrs []string `json:"-"`
	Lines []string `json:"-"`
}

// Dependency is a recipe dependency (another recipe run before this one).
type Dependency struct {
	Recipe string
}

func (d *Dependency) UnmarshalJSON(data []byte) error {
	// Dependencies are either a bare recipe name (string) or an object
	// like {"recipe": "name", "arguments": [...]}.
	var name string
	if err := json.Unmarshal(data, &name); err == nil {
		d.Recipe = name
		return nil
	}
	var obj struct {
		Recipe string `json:"recipe"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	d.Recipe = obj.Recipe
	return nil
}

func (r *Recipe) UnmarshalJSON(data []byte) error {
	type alias Recipe
	aux := &struct {
		Attributes []json.RawMessage `json:"attributes"`
		*alias
	}{alias: (*alias)(r)}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	r.attributesRaw = aux.Attributes
	r.parseAttributes()
	r.parseBody()
	return nil
}

func (r *Recipe) parseAttributes() {
	for _, raw := range r.attributesRaw {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			r.Attrs = append(r.Attrs, s)
			continue
		}
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw, &obj); err == nil {
			for k, v := range obj {
				var vs string
				if err := json.Unmarshal(v, &vs); err == nil {
					if k == "group" {
						r.Group = vs
					}
					r.Attrs = append(r.Attrs, fmt.Sprintf("%s(%q)", k, vs))
					continue
				}
				r.Attrs = append(r.Attrs, k)
			}
		}
	}
	sort.Strings(r.Attrs)
}

// parseBody reconstructs each shell line of the recipe, rendering
// interpolations as {{name}}.
func (r *Recipe) parseBody() {
	for _, line := range r.Body {
		var b strings.Builder
		for _, tok := range line {
			var s string
			if err := json.Unmarshal(tok, &s); err == nil {
				b.WriteString(s)
				continue
			}
			// Interpolation token: [["variable","name"]] or similar nested form.
			var nested [][]json.RawMessage
			if err := json.Unmarshal(tok, &nested); err == nil {
				for _, pair := range nested {
					if len(pair) == 2 {
						var name string
						json.Unmarshal(pair[1], &name)
						b.WriteString("{{" + name + "}}")
					}
				}
				continue
			}
		}
		r.Lines = append(r.Lines, b.String())
	}
}

// Alias maps an alternate name to a recipe.
type Alias struct {
	Name   string `json:"name"`
	Target string `json:"target"`
}

// Dump is the top-level structure produced by `just --dump --dump-format json`.
type Dump struct {
	Recipes map[string]*Recipe `json:"recipes"`
	Aliases map[string]Alias   `json:"aliases"`
	Source  string             `json:"source"`
}

// Load runs `just --dump --dump-format json` in dir (or the current
// directory if dir is empty) and parses the result.
func Load(dir string) (*Dump, error) {
	args := []string{"--dump", "--dump-format", "json"}
	cmd := exec.Command("just", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("just --dump failed: %s", msg)
	}
	var dump Dump
	if err := json.Unmarshal(stdout.Bytes(), &dump); err != nil {
		return nil, fmt.Errorf("parsing just dump: %w", err)
	}
	for name, rec := range dump.Recipes {
		rec.Name = name
	}
	return &dump, nil
}

// SortedRecipes returns non-private recipes sorted by name, unless
// includePrivate is true.
func (d *Dump) SortedRecipes(includePrivate bool) []*Recipe {
	out := make([]*Recipe, 0, len(d.Recipes))
	for _, r := range d.Recipes {
		if r.Private && !includePrivate {
			continue
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// AliasesFor returns the alias names that point at the given recipe.
func (d *Dump) AliasesFor(recipe string) []string {
	var out []string
	for _, a := range d.Aliases {
		if a.Target == recipe {
			out = append(out, a.Name)
		}
	}
	sort.Strings(out)
	return out
}
