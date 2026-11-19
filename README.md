# yamlconf

A `go generate` tool that turns a YAML file plus a Go struct into a type-safe
config loader.

YAML values can name an environment variable and a fallback with
`${ENV:fallback}`. The generated code reads env vars at runtime. Fallbacks and
literal values are checked against the field types when you run
`go generate` and written into the code as Go constants. At runtime there is
no YAML parsing and no reflection.

There are **no implicit defaults**. Every field is a pointer (or a slice or
map), and a field with no value stays `nil`.

## Quick start

**1. Write the struct and mark it**

```go
package config

import "time"

//go:generate go run github.com/abu-maryam-rumaisha/yaml-conf/cmd/yamlconf

//yamlconf:generate config.yaml
type Config struct {
	Name    *string        `yaml:"name"`
	Port    *int           `yaml:"port"`
	Debug   *bool          `yaml:"debug"`
	Timeout *time.Duration `yaml:"timeout"`
	DB      *Database      `yaml:"db"`
}

type Database struct {
	Host *string `yaml:"host"`
	Port *uint16 `yaml:"port"`
	User *string `yaml:"user"`
}
```

**2. Write the YAML**

```yaml
name: "billing"             # literal
port: ${APP_PORT:8080}      # env APP_PORT, otherwise 8080
debug: ${APP_DEBUG}         # env APP_DEBUG, otherwise nil
timeout: ${APP_TIMEOUT:30s}
db:
  host: ${DB_HOST:localhost}
  port: ${DB_PORT:5432}
  user: ${DB_USER}
```

**3. Generate and use**

```sh
go generate ./...
```

This writes `config_yamlconf.go` with two functions:

```go
cfg, err := config.LoadConfig()                     // reads os.LookupEnv
cfg, err := config.LoadConfigWith(func(k string) (string, bool) {
	return map[string]string{"APP_PORT": "9000"}[k], true // custom source, e.g. in tests
})
```

## Value syntax

| YAML value              | Meaning                                                      |
| ----------------------- | ------------------------------------------------------------ |
| `${ENV:fallback}`       | Use env `ENV`. If it is unset or empty, use `fallback`.      |
| `${ENV}`                | Use env `ENV`. If it is unset or empty, the field is `nil`.  |
| `${ENV:}`               | Same as `${ENV}`. An empty fallback means no fallback.       |
| `${:value}`             | Same as the literal `value`.                                 |
| `5`, `true`, `"name"`   | Literal value, with no env lookup.                           |
| `~`, `null`, or nothing | The field is `nil`.                                          |

Rules:

- **Empty env vars:** an env var that is set to `""` counts as unset.
- **Colons in fallbacks:** the value is split at the first `:`, so a fallback
  can contain colons: `${DB_URL:postgres://u:p@localhost/db}`.
- **Quoting:** quoted values are resolved the same way, so `"${X:y}"` works.
  Inside inline lists (`[a, b]`) you **must** quote them, because YAML doesn't
  allow unquoted `{}` there: `tags: [api, "${REGION:eu}"]`.
- **Typos:** a value that starts with `${` but isn't a valid reference
  (for example `${X`) fails `go generate`.

## Supported types

| Go type                                    | Parsed with                     |
| ------------------------------------------ | ------------------------------- |
| `*string`                                  | as is                           |
| `*bool`                                    | `strconv.ParseBool`             |
| `*int`, `*int8` … `*int64`, `*rune`        | `strconv.ParseInt` (base 10)    |
| `*uint`, `*uint8` … `*uint64`, `*byte`     | `strconv.ParseUint` (base 10)   |
| `*float32`, `*float64`                     | `strconv.ParseFloat`            |
| `*time.Duration`                           | `time.ParseDuration`            |
| `*MyType` where `type MyType string` (etc) | the parser for its base type    |
| `*Struct`                                  | a YAML mapping                  |
| `[]*T`, `*[]*T`                            | a YAML sequence                 |
| `map[string]*T`                            | a YAML mapping                  |

`T` can be any type in this table, so containers can nest: `[]*Server`,
`map[string]*[]*int`, `*[][]*int`, and so on.

### Nil rules

- A scalar field is `nil` when neither the env var nor a fallback gives a value.
- A struct, slice or map is `nil` when **nothing inside it** got a value.
- Inside a non-nil slice, an item with no value is a `nil` entry, so indexes
  still match the YAML.
- A map only gets keys that have a value.
- Fields that aren't in the YAML are `nil`.

## Lists, maps and references

```go
type Config struct {
	Replicas []*Database        `yaml:"replicas"`
	Tags     []*string          `yaml:"tags"`
	Limits   map[string]*float64 `yaml:"limits"`
}
```

```yaml
primary: &primary            # anchor
  host: ${DB_HOST:localhost}
  port: 5432

replicas:
  - <<: *primary             # merge key: inherit, then override
    host: ${REPLICA1_HOST:replica-1}
  - *primary                 # alias
  - host: ${REPLICA3_HOST}   # nil entry when REPLICA3_HOST is unset

tags:
  - api
  - ${REGION:eu-west-1}

limits:
  cpu: ${CPU_LIMIT:0.5}
  mem: 512
```

With a list of merge sources, `<<: [*a, *b]`, keys you write directly win
over merged ones, and among the sources the earlier one wins. This follows the
YAML merge spec.

> Note: in this example `primary` has to be a field of `Config` as well,
> because unknown keys are errors. To keep shared anchors outside your struct,
> put them in a separate section and use a [prefix](#binding-to-a-section-prefix).

## The marker comment

```
//yamlconf:generate <file.yaml> [prefix=a.b.c] [func=LoadX] [out=x.go]
```

| Option      | Default              | Meaning                                         |
| ----------- | -------------------- | ----------------------------------------------- |
| first arg   | (required)           | YAML file, relative to the package directory. Can also be written `yaml=`. |
| `prefix=`   | document root        | Dotted path to the section this struct binds to |
| `func=`     | `Load<Type>`         | Loader name. A `<func>With` variant is also generated. |
| `out=`      | `<type>_yamlconf.go` | Output file name                                |

- **Placement:** put the marker directly above the `type` declaration. It also
  works for a struct inside a grouped `type ( ... )` block.
- **Spacing:** write it with **no space** after `//`, like `//go:generate`.
  Then gofmt leaves it alone and `go doc` hides it.
- **One directive per package:** a single `//go:generate go run github.com/abu-maryam-rumaisha/yaml-conf/cmd/yamlconf`
  handles every marked struct and writes one file for each.
- **Generated files:** files ending in `_yamlconf.go` are skipped when the
  package is scanned.

## Binding to a section (prefix)

Several structs can share one file, each bound to its own section:

```yaml
shared: &pool
  max_conn: 20

services:
  billing:
    db:
      <<: *pool
      host: ${BILLING_DB_HOST:localhost}
  search:
    nodes:
      - host: ${SEARCH_1:search-1}
      - host: ${SEARCH_2:search-2}
```

```go
//yamlconf:generate config.yaml prefix=services.billing.db func=LoadBillingDB
type DB struct {
	Host    *string `yaml:"host"`
	MaxConn *int    `yaml:"max_conn"`
}

//yamlconf:generate config.yaml prefix=services.search.nodes.0 func=LoadFirstNode
type Node struct {
	Host *string `yaml:"host"`
}
```

- A number in the path picks a list item (`nodes.0`).
- Merge keys and aliases work along the path.
- Only keys under the prefix are checked. Keys elsewhere in the file are ignored.
- If the prefix points at an empty value, the loader returns a struct with
  every field `nil`. If a path segment doesn't exist, or the path doesn't end
  on a mapping, `go generate` fails.
- Error messages include the full path, e.g. `services.billing.db.host`.

## Errors

**At generate time** (`go generate` fails, with the file, key path and line):

- a fallback or literal that doesn't fit the field: `${PORT:abc}` on `*int`,
  `300` on `*int8`, `maybe` on `*bool`, `5 parsecs` on `*time.Duration`
- a scalar or struct field that isn't a pointer, or a slice item that isn't a pointer
- a YAML key with no matching struct field (merged keys included)
- a YAML shape that doesn't match the field (list given for a struct, etc.)
- a malformed `${...}` reference
- a map key type other than `string`, embedded fields, or anonymous struct types
- a bad marker (missing file, unknown option, placed on a non-struct)
- a prefix that can't be found

**At runtime:** an env value that doesn't parse returns an error naming the
key and the variable:

```
yamlconf: db.port (env DB_PORT): strconv.ParseUint: parsing "70000": value out of range
```

## Field names

A field's YAML key comes from its `yaml:"..."` tag. Without a tag, it's the
field name in lowercase (the `gopkg.in/yaml.v3` convention). Fields tagged
`yaml:"-"` and unexported fields are skipped.

## CLI flags

Markers are the recommended way to use the tool. Flags are available for
one-off runs or for overriding a marker:

```
go run github.com/abu-maryam-rumaisha/yaml-conf/cmd/yamlconf [flags]

  -type    generate only this struct (default: all marked structs)
  -yaml    YAML file (default: from the struct's marker)
  -prefix  dotted YAML path (default: from the marker, or the root)
  -func    loader function name (default: Load<Type>)
  -out     output file (default: <type>_yamlconf.go)
  -dir     package directory (default: .)
```

`-yaml`, `-prefix`, `-func` and `-out` need `-type`. Values you pass override
the marker's values.

```go
//go:generate go run github.com/abu-maryam-rumaisha/yaml-conf/cmd/yamlconf -type Config -yaml config.yaml -prefix app
```

## Library use

```go
targets, err := yamlconf.Discover("./config")   // structs with a marker
src, err := yamlconf.Generate(yamlconf.Options{
	Dir:      "./config",
	Type:     "Config",
	YAMLFile: "config.yaml",
	Prefix:   "app",
	Func:     "LoadConfig",
})
```

## Limitations

- An env value is never split into a list. `${HOSTS:a,b}` on a `[]*string`
  field is not split on commas. Write one YAML list item per value.
- No fixed-size arrays, no `interface{}`/`any`, and no map keys other than `string`.
- Prefix segments can't contain `.`.

See [`example/`](example) for a working package with generated output and tests.
