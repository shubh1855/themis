package system

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

func EnvGet(key string) string {
	return os.Getenv(key)
}

func EnvSet(key, value string) error {
	if err := os.Setenv(key, value); err != nil {
		return fmt.Errorf("system: setenv %q: %w", key, err)
	}
	return nil
}

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
