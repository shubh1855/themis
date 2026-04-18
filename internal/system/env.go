package system

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// EnvGet retrieves the value of an environment variable.
func EnvGet(key string) string {
	return os.Getenv(key)
}

// EnvSet sets an environment variable for the current process.
func EnvSet(key, value string) error {
	if err := os.Setenv(key, value); err != nil {
		return fmt.Errorf("system: setenv %q: %w", key, err)
	}
	return nil
}

// EnvList returns all environment variables as a sorted map.
func EnvList() map[string]string {
	result := make(map[string]string)
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		}
	}
	return result
}

// EnvKeys returns all environment variable names, sorted.
func EnvKeys() []string {
	envs := os.Environ()
	keys := make([]string, 0, len(envs))
	for _, env := range envs {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) >= 1 {
			keys = append(keys, parts[0])
		}
	}
	sort.Strings(keys)
	return keys
}
