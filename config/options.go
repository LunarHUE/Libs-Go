package config

// Option customizes a single LoadConfig call.
type Option func(*options)

type options struct {
	envBindings []envBinding
	skipEtcd    bool
	sources     *Sources
}

type envBinding struct {
	key     string
	envVars []string
}

// WithEnvBinding binds a config key to one or more explicit environment
// variables, bypassing the prefix applied by AutomaticEnv. Use it for
// ecosystem-standard variables (KUBECONFIG, SOPS_AGE_KEY_FILE, ...) that
// must not be renamed to a prefixed form. Precedence follows viper's normal
// rule: environment values override file and default values.
func WithEnvBinding(key string, envVars ...string) Option {
	return func(o *options) {
		o.envBindings = append(o.envBindings, envBinding{key: key, envVars: envVars})
	}
}

// WithoutEtcd skips the etcd configuration layer entirely.
func WithoutEtcd() Option {
	return func(o *options) {
		o.skipEtcd = true
	}
}
