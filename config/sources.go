package config

import (
	"embed"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Source identifies the configuration layer a key's effective value came from.
type Source string

const (
	// SourceDefault marks a value that came from the default config file.
	SourceDefault Source = "default"
	// SourceFile marks a value that came from the override config file.
	SourceFile Source = "file"
	// SourceEnv marks a value that came from an environment variable —
	// prefixed via AutomaticEnv or explicitly bound with WithEnvBinding.
	SourceEnv Source = "env"
	// SourceUnset marks a key that is known (for example env-bound) but that
	// no layer set; the struct field holds its zero value.
	SourceUnset Source = "unset"
)

// Sources maps config keys (viper dotted form, e.g. "cluster.namespace") to
// the layer that produced their effective value. It is a snapshot taken at
// the end of LoadConfig: later environment changes do not update it.
//
// The etcd layer is a placeholder no-op at this commit and is not attributed;
// when it gains an implementation it must also gain a Source label here.
type Sources map[string]Source

// Lookup returns the source for key and whether the key is known. Keys are
// case-insensitive, matching viper.
func (s Sources) Lookup(key string) (Source, bool) {
	source, ok := s[strings.ToLower(key)]
	return source, ok
}

// WithSources records, into *dst, the layer each config key's effective value
// came from. dst is overwritten on a successful load and left untouched on
// error.
func WithSources(dst *Sources) Option {
	return func(o *options) {
		o.sources = dst
	}
}

// buildSources labels every key viper knows about. Precedence mirrors the
// merge order in LoadConfig: env beats the override file beats the default
// file. An environment variable set to the empty string does not count —
// viper ignores it with AllowEmptyEnv off, so the label must too.
func buildSources(v *viper.Viper, envPrefix string, bindings []envBinding, defaultKeys, overrideKeys map[string]bool) Sources {
	bound := make(map[string][]string, len(bindings))
	for _, binding := range bindings {
		bound[strings.ToLower(binding.key)] = binding.envVars
	}
	sources := make(Sources)
	for _, key := range v.AllKeys() {
		switch {
		case envSet(key, envPrefix, bound):
			sources[key] = SourceEnv
		case overrideKeys[key]:
			sources[key] = SourceFile
		case defaultKeys[key]:
			sources[key] = SourceDefault
		default:
			sources[key] = SourceUnset
		}
	}
	return sources
}

func envSet(key, envPrefix string, bound map[string][]string) bool {
	for _, name := range bound[key] {
		if value, ok := os.LookupEnv(name); ok && value != "" {
			return true
		}
	}
	if value, ok := os.LookupEnv(automaticEnvName(key, envPrefix)); ok && value != "" {
		return true
	}
	return false
}

// automaticEnvName derives the variable AutomaticEnv reads for a key: the
// replacer from loadConfigEnv applied to the upper-cased key, behind the
// prefix when one is set.
func automaticEnvName(key, envPrefix string) string {
	replacer := strings.NewReplacer("-", "_", ".", "_")
	name := strings.ToUpper(replacer.Replace(key))
	if envPrefix == "" {
		return name
	}
	return strings.ToUpper(envPrefix) + "_" + name
}

// layerKeys snapshots the keys a single config file sets, by reading it into
// a throwaway viper. Used only when source tracking is requested.
func layerKeys(filePath string, embedded *embed.FS) (map[string]bool, error) {
	layer := viper.New()
	if err := loadConfigFile(layer, filePath, embedded); err != nil {
		return nil, err
	}
	keys := make(map[string]bool)
	for _, key := range layer.AllKeys() {
		keys[key] = true
	}
	return keys, nil
}
