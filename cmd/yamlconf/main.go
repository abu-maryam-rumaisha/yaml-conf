// Command yamlconf generates config loaders for structs from YAML files.
//
// Mark structs in the package, then add a single go:generate directive:
//
//	//go:generate go run github.com/abu-maryam-rumaisha/yaml-conf/cmd/yamlconf
//
//	//yamlconf:generate config.yaml
//	type Config struct { ... }
//
// The marker accepts optional func=<LoaderName>, out=<file.go> and
// prefix=<dotted.path> (bind the struct to a sub-tree of the YAML).
// Alternatively, -type/-yaml target one struct explicitly.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	yamlconf "github.com/abu-maryam-rumaisha/yaml-conf"
)

func main() {
	typ := flag.String("type", "", "generate only this struct (default: all structs marked "+yamlconf.Directive+")")
	yml := flag.String("yaml", "", "YAML file, relative to the package dir (default: from the struct's marker)")
	out := flag.String("out", "", "output file (default <type>"+yamlconf.GeneratedSuffix+")")
	fn := flag.String("func", "", "loader function name (default Load<Type>)")
	prefix := flag.String("prefix", "", "dotted YAML path the struct binds to (default: from the marker, or the root)")
	dir := flag.String("dir", ".", "package directory")
	flag.Parse()

	if err := run(*dir, *typ, *yml, *out, *fn, *prefix); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(dir, typ, yml, out, fn, prefix string) error {
	targets, err := yamlconf.Discover(dir)
	if err != nil {
		return err
	}

	if typ != "" {
		// Explicit target: flags override whatever the marker says.
		t := yamlconf.Target{Type: typ}
		for _, m := range targets {
			if m.Type == typ {
				t = m
			}
		}
		if yml != "" {
			t.YAMLFile = yml
		}
		if out != "" {
			t.Output = out
		}
		if fn != "" {
			t.Func = fn
		}
		if prefix != "" {
			t.Prefix = prefix
		}
		if t.YAMLFile == "" {
			return fmt.Errorf("yamlconf: %s has no %s marker; pass -yaml", typ, yamlconf.Directive)
		}
		if t.Output == "" {
			t.Output = strings.ToLower(typ) + yamlconf.GeneratedSuffix
		}
		targets = []yamlconf.Target{t}
	} else if yml != "" || out != "" || fn != "" || prefix != "" {
		return fmt.Errorf("yamlconf: -yaml, -out, -func and -prefix require -type")
	}

	if len(targets) == 0 {
		return fmt.Errorf("yamlconf: no structs marked %s in %s", yamlconf.Directive, dir)
	}

	for _, t := range targets {
		src, err := yamlconf.Generate(yamlconf.Options{
			Dir: dir, Type: t.Type, YAMLFile: t.YAMLFile, Output: t.Output, Func: t.Func, Prefix: t.Prefix,
		})
		if err != nil {
			return err
		}
		path := t.Output
		if !filepath.IsAbs(path) {
			path = filepath.Join(dir, path)
		}
		if err := os.WriteFile(path, src, 0o644); err != nil {
			return err
		}
	}
	return nil
}
