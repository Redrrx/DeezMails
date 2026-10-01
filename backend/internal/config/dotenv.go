package config

import (
	"fmt"
	"os"

	envconfig "github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

func loadEnvironment(paths ...string) (map[string]string, error) {
	values := envconfig.ToMap(os.Environ())
	for _, path := range paths {
		fileValues, err := godotenv.Read(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read %s: invalid or unreadable dotenv file", path)
		}
		for name, value := range fileValues {
			if _, exists := values[name]; !exists {
				values[name] = value
			}
		}
	}
	return values, nil
}
