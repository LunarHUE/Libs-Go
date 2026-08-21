package config

import "github.com/spf13/viper"

// Not yet implemented, but placeholder for future use. A function variable so
// tests can observe whether the etcd layer ran (see WithoutEtcd).
var loadConfigEtcd = func(v *viper.Viper) error {
	// log.Warnf("Etcd config loading is not implemented yet")

	// return fmt.Errorf("not implemented")
	return nil
}
