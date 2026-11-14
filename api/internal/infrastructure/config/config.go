// Package config reads process configuration from the environment, with an
// optional .env file for local development.
package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr         string
	DatabaseURL      string
	OpenFGAAPIURL    string
	OpenFGAStoreName string
	OpenFGAModelPath string
	JWTSecret        string
	JWTTTL           time.Duration
	CORSOrigin       string
}

var defaults = map[string]string{
	"HTTP_ADDR":          ":8088",
	"DATABASE_URL":       "postgres://fgademo:password@localhost:5432/fgademo?sslmode=disable",
	"OPENFGA_API_URL":    "http://localhost:8080",
	"OPENFGA_STORE_NAME": "fga-demo",
	"OPENFGA_MODEL_PATH": "fga/model.fga",
	"JWT_SECRET":         "dev-secret-change-me",
	"JWT_TTL":            "12h",
	"CORS_ORIGIN":        "http://localhost:5173",
}

// dotEnvPaths is where Load looks for .env: the working directory (api/),
// then the repository root.
var dotEnvPaths = []string{".env", "../.env"}

// Load reads the first .env found (without overriding variables already set
// in the process) and returns the resolved configuration.
func Load() (Config, error) {
	for _, p := range dotEnvPaths {
		found, err := loadDotEnv(p)
		if err != nil {
			return Config{}, err
		}
		if found {
			break
		}
	}
	ttl, err := time.ParseDuration(get("JWT_TTL"))
	if err != nil {
		return Config{}, fmt.Errorf("JWT_TTL: %w", err)
	}
	return Config{
		HTTPAddr:         get("HTTP_ADDR"),
		DatabaseURL:      get("DATABASE_URL"),
		OpenFGAAPIURL:    get("OPENFGA_API_URL"),
		OpenFGAStoreName: get("OPENFGA_STORE_NAME"),
		OpenFGAModelPath: get("OPENFGA_MODEL_PATH"),
		JWTSecret:        get("JWT_SECRET"),
		JWTTTL:           ttl,
		CORSOrigin:       get("CORS_ORIGIN"),
	}, nil
}

func get(key string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return defaults[key]
}

func loadDotEnv(path string) (bool, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		if _, set := os.LookupEnv(k); set {
			continue
		}
		os.Setenv(k, strings.Trim(strings.TrimSpace(v), `"`))
	}
	return true, sc.Err()
}
