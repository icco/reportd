package lib

import (
	"fmt"
	"regexp"
)

// ValidateService returns an error if the service name is invalid.
// The service name must not be empty, must be less than 32 characters,
// and must match the regex "^[a-zA-Z0-9_-]+$".
func ValidateService(service string) error {
	if service == "" {
		return fmt.Errorf("service must not be empty")
	}

	if len(service) > 32 {
		return fmt.Errorf("service must be less than 32 characters")
	}

	validRegex, err := regexp.Compile("^[a-zA-Z0-9_-]+$")
	if err != nil {
		return fmt.Errorf("compiling regex: %w", err)
	}

	if !validRegex.MatchString(service) {
		return fmt.Errorf("service must match regex")
	}

	return nil
}
