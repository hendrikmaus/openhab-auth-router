package config

import (
	"fmt"
)

// Validate config struct with some basic assertions
func Validate(config *Main) error {
	for user, userData := range config.Users {
		if len(userData.Entrypoint) == 0 {
			return fmt.Errorf("The field `entrypoint` is missing for user '%s'", user)
		}

		if len(userData.Sitemaps.Default) == 0 {
			return fmt.Errorf("The field `sitemaps.default` is missing for user '%s'", user)
		}

		if len(userData.Sitemaps.Allowed) == 0 {
			return fmt.Errorf("The field `sitemaps.allowed` is missing for user '%s'", user)
		}
	}

	return nil
}
