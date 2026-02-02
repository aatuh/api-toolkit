//go:build !dev

// Package securityprofile provides security posture defaults.
package securityprofile

func devBypassEnabled() bool {
	return false
}
