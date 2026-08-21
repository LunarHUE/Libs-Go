package config

import "github.com/spf13/viper"

// SwapEtcdLoader replaces the etcd layer for tests and returns a restore func.
func SwapEtcdLoader(fn func(*viper.Viper) error) (restore func()) {
	prev := loadConfigEtcd
	loadConfigEtcd = fn
	return func() { loadConfigEtcd = prev }
}
