package config

import "fmt"

var (
	Major = 0
	Minor = 3
	Patch = 0
)

func Version() string {
	return fmt.Sprintf("v%d.%d.%d", Major, Minor, Patch)
}
