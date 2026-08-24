package config_test

import (
	"testing"

	"github.com/lunarhue/libs-go/config"
)

// TestWithSources labels one key per layer: default file, override file,
// prefixed env, bound env, and a bound-but-unset key.
func TestWithSources(t *testing.T) {
	dir := t.TempDir()
	defaults := writeFile(t, dir, "defaults.yaml", "shared: d\nonly_one: d\nname: d\n")
	override := writeFile(t, dir, "override.yaml", "only_one: o\n")

	t.Setenv("MYAPP_NAME", "env-name")
	t.Setenv("KUBECONFIG", "/env/kubeconfig")

	var sources config.Sources
	cfg, err := config.LoadConfig[testConfig](nil, defaults, override, "MYAPP",
		config.WithEnvBinding("kubeconfig", "KUBECONFIG"),
		config.WithSources(&sources),
	)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.OnlyOne != "o" {
		t.Fatalf("only_one = %q, want override value %q", cfg.OnlyOne, "o")
	}

	want := map[string]config.Source{
		"shared":     config.SourceDefault,
		"only_one":   config.SourceFile,
		"name":       config.SourceEnv,
		"kubeconfig": config.SourceEnv,
	}
	for key, wantSource := range want {
		got, ok := sources.Lookup(key)
		if !ok {
			t.Errorf("Lookup(%q): key unknown, want %s", key, wantSource)
			continue
		}
		if got != wantSource {
			t.Errorf("Lookup(%q) = %s, want %s", key, got, wantSource)
		}
	}

	if _, ok := sources.Lookup("no_such_key"); ok {
		t.Error("Lookup of an unknown key reported ok")
	}
}

// TestWithSourcesEmptyEnvIgnored pins the AllowEmptyEnv-off rule: a variable
// set to "" leaves the default in effect, and the label must agree.
func TestWithSourcesEmptyEnvIgnored(t *testing.T) {
	dir := t.TempDir()
	defaults := writeFile(t, dir, "defaults.yaml", "name: d\n")

	t.Setenv("MYAPP_NAME", "")

	var sources config.Sources
	cfg, err := config.LoadConfig[testConfig](nil, defaults, "", "MYAPP",
		config.WithSources(&sources),
	)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Name != "d" {
		t.Fatalf("name = %q, want default %q (empty env must be ignored)", cfg.Name, "d")
	}
	if got, ok := sources.Lookup("name"); !ok || got != config.SourceDefault {
		t.Errorf("Lookup(name) = %v, %v; want %s, true", got, ok, config.SourceDefault)
	}
}

// TestWithSourcesBoundUnset pins the unset label: an env-bound key that no
// layer sets is known to the table but marked unset.
func TestWithSourcesBoundUnset(t *testing.T) {
	dir := t.TempDir()
	defaults := writeFile(t, dir, "defaults.yaml", "name: d\n")

	var sources config.Sources
	if _, err := config.LoadConfig[testConfig](nil, defaults, "", "MYAPP",
		config.WithEnvBinding("kubeconfig", "SOME_UNSET_VAR_FOR_TEST"),
		config.WithSources(&sources),
	); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if got, ok := sources.Lookup("kubeconfig"); !ok || got != config.SourceUnset {
		t.Errorf("Lookup(kubeconfig) = %v, %v; want %s, true", got, ok, config.SourceUnset)
	}
}

// TestWithSourcesNotRequested checks the zero-cost path: without the option,
// the caller's map stays nil and loading is unaffected.
func TestWithSourcesNotRequested(t *testing.T) {
	dir := t.TempDir()
	defaults := writeFile(t, dir, "defaults.yaml", "name: d\n")

	cfg, err := config.LoadConfig[testConfig](nil, defaults, "", "MYAPP")
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Name != "d" {
		t.Fatalf("name = %q, want %q", cfg.Name, "d")
	}
}
