//go:build disable_tls

package config

// GetMetricsConfigName returns the name of the configmap containing all
// customizations for the storage bucket.
func GetMetricsConfigName() string { panic("not supported when tls is disabled") }
