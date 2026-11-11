package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv               string
	HTTPAddr             string
	ScyllaHosts          []string
	ScyllaPort           int
	ScyllaKeyspace       string
	ScyllaUsername       string
	ScyllaPassword       string
	ScyllaConnectTimeout time.Duration
	ScyllaNumConns       int

	JWTAccessSecret   string
	JWTRefreshSecret  string
	AccessTokenTTL    time.Duration
	RefreshTokenTTL   time.Duration
	MaxFailedLogins   int
	LockDuration      time.Duration
	ActivationCodeTTL time.Duration
}

func Load() (Config, error) {
	timeout, err := time.ParseDuration(getEnv("SCYLLA_CONNECT_TIMEOUT", "10s"))
	if err != nil || timeout <= 0 {
		return Config{}, fmt.Errorf("SCYLLA_CONNECT_TIMEOUT must be a positive duration")
	}

	port, err := strconv.Atoi(getEnv("SCYLLA_PORT", "9042"))
	if err != nil || port < 1 || port > 65535 {
		return Config{}, fmt.Errorf("SCYLLA_PORT must be between 1 and 65535")
	}

	numConns, err := strconv.Atoi(getEnv("SCYLLA_NUM_CONNS", "2"))
	if err != nil || numConns < 1 {
		return Config{}, fmt.Errorf("SCYLLA_NUM_CONNS must be a positive integer")
	}

	accessMinutes, err := positiveIntEnv("ACCESS_TOKEN_TTL_MINUTES", 15)
	if err != nil {
		return Config{}, err
	}

	refreshHours, err := positiveIntEnv("REFRESH_TOKEN_TTL_HOURS", 168)
	if err != nil {
		return Config{}, err
	}

	maxFailedLogins, err := positiveIntEnv("AUTH_MAX_FAILED_LOGINS", 5)
	if err != nil {
		return Config{}, err
	}

	lockDuration, err := positiveDurationEnv("AUTH_LOCK_DURATION", "15m")
	if err != nil {
		return Config{}, err
	}

	activationCodeTTL, err := positiveDurationEnv("AUTH_ACTIVATION_CODE_TTL", "15m")
	if err != nil {
		return Config{}, err
	}

	accessSecret := os.Getenv("JWT_ACCESS_SECRET")
	refreshSecret := os.Getenv("JWT_REFRESH_SECRET")

	if (accessSecret == "") != (refreshSecret == "") {
		return Config{}, fmt.Errorf("JWT_ACCESS_SECRET and JWT_REFRESH_SECRET must either both be set or both be empty")
	}

	if accessSecret != "" {
		if len(accessSecret) < 32 || len(refreshSecret) < 32 {
			return Config{}, fmt.Errorf("JWT secrets must each contain at least 32 bytes")
		}
		if accessSecret == refreshSecret {
			return Config{}, fmt.Errorf("JWT access and refresh secrets must be different")
		}
	}

	hosts := strings.Split(getEnv("SCYLLA_HOSTS", "localhost"), ",")
	for i := range hosts {
		hosts[i] = strings.TrimSpace(hosts[i])
		if hosts[i] == "" {
			return Config{}, fmt.Errorf("SCYLLA_HOSTS contains an empty host")
		}
	}

	return Config{
		AppEnv:               getEnv("APP_ENV", "development"),
		HTTPAddr:             getEnv("HTTP_ADDR", ":8080"),
		ScyllaHosts:          hosts,
		ScyllaPort:           port,
		ScyllaKeyspace:       getEnv("SCYLLA_KEYSPACE", "psbc"),
		ScyllaUsername:       os.Getenv("SCYLLA_USERNAME"),
		ScyllaPassword:       os.Getenv("SCYLLA_PASSWORD"),
		ScyllaConnectTimeout: timeout,
		ScyllaNumConns:       numConns,
		JWTAccessSecret:      accessSecret,
		JWTRefreshSecret:     refreshSecret,
		AccessTokenTTL:       time.Duration(accessMinutes) * time.Minute,
		RefreshTokenTTL:      time.Duration(refreshHours) * time.Hour,
		MaxFailedLogins:      maxFailedLogins,
		LockDuration:         lockDuration,
		ActivationCodeTTL:    activationCodeTTL,
	}, nil
}

func positiveIntEnv(key string, fallback int) (int, error) {
	value, err := strconv.Atoi(getEnv(key, strconv.Itoa(fallback)))
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}
	return value, nil
}

func positiveDurationEnv(key, fallback string) (time.Duration, error) {
	value, err := time.ParseDuration(getEnv(key, fallback))
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", key)
	}
	return value, nil
}

func getEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
