package example

import "time"

//go:generate go run github.com/abu-maryam-rumaisha/yaml-conf/cmd/yamlconf

type LogLevel string

// Config is the service configuration.
//
//yamlconf:generate config.yaml
type Config struct {
	Name    *string        `yaml:"name"`
	Port    *int           `yaml:"port"`
	Debug   *bool          `yaml:"debug"`
	Level   *LogLevel      `yaml:"level"`
	Timeout *time.Duration `yaml:"timeout"`
	Secret  *string        `yaml:"secret"`

	DB       *Database           `yaml:"db"`
	Replicas []*Database         `yaml:"replicas"`
	Tags     []*string           `yaml:"tags"`
	Limits   map[string]*float64 `yaml:"limits"`
	Matrix   *[][]*int           `yaml:"matrix"`
	Unused   *Database           `yaml:"unused"`
}

// Database is also loadable on its own from the "db" section of config.yaml.
//
//yamlconf:generate config.yaml prefix=db func=LoadPrimaryDB
type Database struct {
	Host    *string `yaml:"host"`
	Port    *uint16 `yaml:"port"`
	User    *string `yaml:"user"`
	MaxConn *int    `yaml:"max_conn"`
}
