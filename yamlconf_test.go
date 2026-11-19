package yamlconf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func genSrc(t *testing.T, src, yml string) (string, error) {
	t.Helper()
	dir := t.TempDir()
	must(t, os.WriteFile(filepath.Join(dir, "c.go"), []byte("package p\n\nimport \"time\"\n\nvar _ time.Duration\n\n"+src), 0o644))
	must(t, os.WriteFile(filepath.Join(dir, "c.yaml"), []byte(yml), 0o644))
	out, err := Generate(Options{Dir: dir, Type: "C", YAMLFile: "c.yaml"})
	return string(out), err
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestGenerateErrors(t *testing.T) {
	cases := []struct {
		name, src, yml, want string
	}{
		{"non-pointer", "type C struct{ A int `yaml:\"a\"` }", "a: 1", "must be a pointer"},
		{"bad fallback", "type C struct{ A *int `yaml:\"a\"` }", "a: ${X:abc}", `"abc" is not a valid int`},
		{"overflow", "type C struct{ A *int8 `yaml:\"a\"` }", "a: 300", "not a valid int8"},
		{"bad literal bool", "type C struct{ A *bool `yaml:\"a\"` }", "a: maybe", "not a valid bool"},
		{"bad duration", "type C struct{ A *time.Duration `yaml:\"a\"` }", "a: ${D:5 parsecs}", "not a valid time.Duration"},
		{"unknown key", "type C struct{ A *int `yaml:\"a\"` }", "b: 1", "no struct field"},
		{"malformed ref", "type C struct{ A *string `yaml:\"a\"` }", "a: ${X", "malformed reference"},
		{"struct needs map", "type C struct{ A *D `yaml:\"a\"` }\ntype D struct{}", "a: 1", "must be a mapping"},
		{"slice needs seq", "type C struct{ A []*int `yaml:\"a\"` }", "a: 1", "needs a YAML sequence"},
		{"slice elem pointer", "type C struct{ A []int `yaml:\"a\"` }", "a: [1]", "must be a pointer"},
		{"map key", "type C struct{ A map[int]*int `yaml:\"a\"` }", "a: {1: 1}", "only map[string]"},
		{"unknown merged key", "type C struct{ A *D `yaml:\"a\"`; B *D `yaml:\"b\"` }\ntype D struct{ X *int `yaml:\"x\"` }",
			"a: &a {x: 1, y: 2}\nb: {<<: *a}", "no struct field"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := genSrc(t, c.src, c.yml)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want error containing %q, got %v", c.want, err)
			}
		})
	}
}

func TestMergeOverridePrecedence(t *testing.T) {
	out, err := genSrc(t,
		"type C struct{ A *D `yaml:\"a\"`; B *D `yaml:\"b\"`; E *D `yaml:\"e\"` }\ntype D struct{ X *int `yaml:\"x\"`; Y *int `yaml:\"y\"` }",
		"a: &a {x: 1, y: 2}\nb: &b {x: 3}\ne:\n  <<: [*b, *a]\n  y: 9\n")
	must(t, err)
	// e.x comes from b (earlier merge source wins), e.y is the explicit 9.
	e := out[strings.LastIndex(out, "c.B = "):]
	if !strings.Contains(e, "int(3)") || !strings.Contains(e, "int(9)") || strings.Contains(e, "int(1)") || strings.Contains(e, "int(2)") {
		t.Fatalf("merge precedence wrong:\n%s", e)
	}
}

func TestEmptyYAML(t *testing.T) {
	out, err := genSrc(t, "type C struct{ A *int `yaml:\"a\"` }", "")
	must(t, err)
	if !strings.Contains(out, "func LoadCWith(_ func") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func writeFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for n, s := range files {
		must(t, os.WriteFile(filepath.Join(dir, n), []byte(s), 0o644))
	}
	return dir
}

func TestDiscover(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"a.go": `package p

//yamlconf:generate app.yaml func=LoadApp out=app_gen.go prefix=x.y
type App struct{ A *int ` + "`yaml:\"a\"`" + ` }

type (
	// grouped spec
	//yamlconf:generate db.yaml
	DB struct{}

	Plain struct{}
)

//yamlconf:generated-not-a-marker x.yaml
type Other struct{}
`,
		"skip_yamlconf.go": "package p\n\n//yamlconf:generate nope.yaml\ntype Gen struct{}\n",
	})
	got, err := Discover(dir)
	must(t, err)
	want := []Target{
		{Type: "App", YAMLFile: "app.yaml", Func: "LoadApp", Output: "app_gen.go", Prefix: "x.y"},
		{Type: "DB", YAMLFile: "db.yaml", Output: "db_yamlconf.go"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("target %d: got %+v want %+v", i, got[i], want[i])
		}
	}
}

func TestDiscoverErrors(t *testing.T) {
	cases := map[string]struct{ src, want string }{
		"missing yaml": {"//yamlconf:generate\ntype C struct{}", "missing YAML file"},
		"not struct":   {"//yamlconf:generate c.yaml\ntype C int", "not a struct"},
		"bad option":   {"//yamlconf:generate c.yaml foo=bar\ntype C struct{}", `unknown option "foo"`},
		"extra arg":    {"//yamlconf:generate c.yaml d.yaml\ntype C struct{}", "unexpected argument"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Discover(writeFiles(t, map[string]string{"c.go": "package p\n\n" + c.src + "\n"}))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want %q, got %v", c.want, err)
			}
		})
	}
}

func TestPrefix(t *testing.T) {
	const src = "package p\n\ntype C struct{ X *int `yaml:\"x\"` }\n"
	const yml = `
base: &base {x: 7}
app:
  db:
    x: ${DBX:1}
  merged:
    <<: *base
  servers:
    - {x: 2}
    - {x: 3}
  empty:
  scalar: 5
`
	dir := writeFiles(t, map[string]string{"c.go": src, "c.yaml": yml})
	g := func(prefix string) (string, error) {
		out, err := Generate(Options{Dir: dir, Type: "C", YAMLFile: "c.yaml", Prefix: prefix})
		return string(out), err
	}

	ok := map[string]string{
		"app.db":        `lookup("DBX")`,
		".app.db.":      `lookup("DBX")`,
		"app.merged":    "int(7)",
		"app.servers.1": "int(3)",
		"app.empty":     "c := &C{}\n\treturn c, nil",
	}
	for prefix, want := range ok {
		out, err := g(prefix)
		if err != nil || !strings.Contains(out, want) {
			t.Errorf("prefix %q: want %q, err=%v\n%s", prefix, want, err, out)
		}
	}
	out, _ := g("app.db")
	if !strings.Contains(out, "app.db.x (env DBX)") {
		t.Errorf("runtime error message should include prefix:\n%s", out)
	}

	bad := map[string]string{
		"app.nope":      "app.nope not found",
		"app.servers.9": "app.servers.9 not found",
		"app.scalar":    "must point to a mapping",
		"app.scalar.x":  "is a scalar",
		"app..db":       "empty segment",
		"app.servers":   "must point to a mapping",
	}
	for prefix, want := range bad {
		if _, err := g(prefix); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("prefix %q: want error %q, got %v", prefix, want, err)
		}
	}
	// Unknown keys under the prefix are still rejected, with the full path.
	dir2 := writeFiles(t, map[string]string{"c.go": src, "c.yaml": "a:\n  b:\n    y: 1\n"})
	if _, err := Generate(Options{Dir: dir2, Type: "C", YAMLFile: "c.yaml", Prefix: "a.b"}); err == nil || !strings.Contains(err.Error(), "a.b.y") {
		t.Errorf("want unknown key error with full path, got %v", err)
	}
}

func TestDiscoverHonoursBuildConstraints(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"a.go":           "package p\n\n//yamlconf:generate a.yaml\ntype A struct{}\n",
		"ignored.go":     "//go:build ignore\n\npackage main\n\n//yamlconf:generate x.yaml\ntype X struct{}\n",
		"other_plan9.go": "package p\n\n//yamlconf:generate y.yaml\ntype Y struct{}\n",
	})
	got, err := Discover(dir)
	must(t, err)
	if len(got) != 1 || got[0].Type != "A" {
		t.Fatalf("want only A, got %+v", got)
	}
}
