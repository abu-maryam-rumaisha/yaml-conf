package example

import (
	"strings"
	"testing"
	"time"
)

func env(m map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) { v, ok := m[k]; return v, ok }
}

func TestFallbacksAndNil(t *testing.T) {
	c, err := LoadConfigWith(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if *c.Name != "my-service" || *c.Port != 8080 || *c.Level != "info" || *c.Timeout != 90*time.Second {
		t.Fatalf("fallbacks wrong: %+v", c)
	}
	if c.Debug != nil || c.Secret != nil || c.DB.User != nil {
		t.Fatal("env-only fields should be nil when unset")
	}
	if c.Unused != nil {
		t.Fatal("struct with no values should be nil")
	}
	if len(c.Replicas) != 3 || c.Replicas[2] != nil {
		t.Fatalf("replica[2] should be nil element, got %+v", c.Replicas)
	}
	if *c.Replicas[0].Host != "replica-1" || *c.Replicas[0].MaxConn != 10 {
		t.Fatal("merge key not applied")
	}
	if *c.Replicas[1].Host != "localhost" {
		t.Fatal("alias not applied")
	}
	if *c.Tags[1] != "eu-west-1" || *c.Limits["cpu"] != 0.5 || *c.Limits["mem"] != 512 {
		t.Fatal("list/map values wrong")
	}
	if m := *c.Matrix; *m[0][1] != 2 || m[1][1] != nil {
		t.Fatal("matrix wrong")
	}
}

func TestEnvOverrides(t *testing.T) {
	c, err := LoadConfigWith(env(map[string]string{
		"APP_PORT": "9000", "APP_DEBUG": "true", "DB_HOST": "db.prod",
		"REPLICA3_HOST": "r3", "APP_TIMEOUT": "5s", "APP_SECRET": "",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if *c.Port != 9000 || !*c.Debug || *c.DB.Host != "db.prod" || *c.Timeout != 5*time.Second {
		t.Fatalf("overrides wrong: %+v", c)
	}
	if *c.Replicas[1].Host != "db.prod" || *c.Replicas[2].Host != "r3" {
		t.Fatal("alias should see env too")
	}
	if c.Secret != nil {
		t.Fatal("empty env counts as unset")
	}
}

func TestBadEnv(t *testing.T) {
	_, err := LoadConfigWith(env(map[string]string{"DB_PORT": "70000"}))
	if err == nil || !strings.Contains(err.Error(), "db.port (env DB_PORT)") {
		t.Fatalf("want range error, got %v", err)
	}
}

func TestPrefixLoader(t *testing.T) {
	db, err := LoadPrimaryDBWith(env(map[string]string{"DB_HOST": "db.prod"}))
	if err != nil {
		t.Fatal(err)
	}
	if *db.Host != "db.prod" || *db.Port != 5432 || *db.MaxConn != 10 || db.User != nil {
		t.Fatalf("prefix loader wrong: %+v", db)
	}
	_, err = LoadPrimaryDBWith(env(map[string]string{"DB_PORT": "x"}))
	if err == nil || !strings.Contains(err.Error(), "db.port (env DB_PORT)") {
		t.Fatalf("error should carry full path, got %v", err)
	}
}
