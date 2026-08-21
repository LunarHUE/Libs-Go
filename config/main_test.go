package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lunarhue/libs-go/config"
	"github.com/spf13/viper"
)

type testConfig struct {
	Shared     string `mapstructure:"shared"`
	OnlyOne    string `mapstructure:"only_one"`
	Kubeconfig string `mapstructure:"kubeconfig"`
	Name       string `mapstructure:"name"`
}

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	return path
}

// TestSequentialLoadsAreIndependent drives the defect the old package-global
// viper had: a second LoadConfig call merged over the first call's keys, and
// the first call's env prefix kept applying. With a per-call viper, two
// sequential loads under different prefixes and override files must be fully
// independent.
func TestSequentialLoadsAreIndependent(t *testing.T) {
	dir := t.TempDir()
	fileOne := writeFile(t, dir, "one.yaml", "shared: from-file-1\nonly_one: call1-value\n")
	fileTwo := writeFile(t, dir, "two.yaml", "shared: from-file-2\n")

	t.Setenv("APPONE_SHARED", "env-one")

	one, err := config.LoadConfig[testConfig](nil, "", fileOne, "APPONE")
	if err != nil {
		t.Fatalf("first LoadConfig: %v", err)
	}
	if one.Shared != "env-one" {
		t.Errorf("call 1 shared = %q, want env override %q", one.Shared, "env-one")
	}
	if one.OnlyOne != "call1-value" {
		t.Errorf("call 1 only_one = %q, want %q", one.OnlyOne, "call1-value")
	}

	two, err := config.LoadConfig[testConfig](nil, "", fileTwo, "APPTWO")
	if err != nil {
		t.Fatalf("second LoadConfig: %v", err)
	}
	if two.Shared != "from-file-2" {
		t.Errorf("call 2 shared = %q, want %q (call 1's APPONE env/prefix must not apply)", two.Shared, "from-file-2")
	}
	if two.OnlyOne != "" {
		t.Errorf("call 2 only_one = %q, want empty: key set only by call 1 leaked into call 2", two.OnlyOne)
	}
}

// TestWithEnvBinding checks that an explicitly bound, unprefixed env var
// (KUBECONFIG) reaches its config key while sibling keys still resolve
// through the prefixed AutomaticEnv path, and that env beats the file value.
func TestWithEnvBinding(t *testing.T) {
	dir := t.TempDir()
	file := writeFile(t, dir, "cfg.yaml", "kubeconfig: from-file\nname: from-file\n")

	t.Setenv("KUBECONFIG", "/env/kubeconfig")
	t.Setenv("MYAPP_NAME", "env-name")

	cfg, err := config.LoadConfig[testConfig](nil, "", file, "MYAPP",
		config.WithEnvBinding("kubeconfig", "KUBECONFIG"),
	)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Kubeconfig != "/env/kubeconfig" {
		t.Errorf("kubeconfig = %q, want unprefixed KUBECONFIG value %q", cfg.Kubeconfig, "/env/kubeconfig")
	}
	if cfg.Name != "env-name" {
		t.Errorf("name = %q, want prefixed env value %q", cfg.Name, "env-name")
	}
}

// TestWithEnvBindingPrecedenceWithoutEnv checks the file value survives when
// the bound env var is unset.
func TestWithEnvBindingPrecedenceWithoutEnv(t *testing.T) {
	dir := t.TempDir()
	file := writeFile(t, dir, "cfg.yaml", "kubeconfig: from-file\n")

	cfg, err := config.LoadConfig[testConfig](nil, "", file, "MYAPP",
		config.WithEnvBinding("kubeconfig", "SOME_UNSET_VAR_FOR_TEST"),
	)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Kubeconfig != "from-file" {
		t.Errorf("kubeconfig = %q, want file value %q", cfg.Kubeconfig, "from-file")
	}
}

// TestWithoutEtcdSkipsEtcdLayer swaps in an observing etcd loader and checks
// WithoutEtcd prevents it from running, while a plain load still runs it.
func TestWithoutEtcdSkipsEtcdLayer(t *testing.T) {
	dir := t.TempDir()
	file := writeFile(t, dir, "cfg.yaml", "name: n\n")

	called := false
	restore := config.SwapEtcdLoader(func(*viper.Viper) error {
		called = true
		return nil
	})
	defer restore()

	if _, err := config.LoadConfig[testConfig](nil, "", file, "MYAPP", config.WithoutEtcd()); err != nil {
		t.Fatalf("LoadConfig with WithoutEtcd: %v", err)
	}
	if called {
		t.Error("etcd loader ran despite WithoutEtcd()")
	}

	if _, err := config.LoadConfig[testConfig](nil, "", file, "MYAPP"); err != nil {
		t.Fatalf("LoadConfig without option: %v", err)
	}
	if !called {
		t.Error("etcd loader did not run on a default load")
	}
}
