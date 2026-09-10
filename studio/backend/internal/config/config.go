package config

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/url"
	"slices"
	"strings"
	"time"
)

const (
	defaultAddress           = "127.0.0.1:8080"
	defaultMaxRequestBytes   = int64(257 << 20)
	defaultJARBytes          = int64(256 << 20)
	defaultTextArtifactBytes = int64(1 << 20)
	defaultPreviewBytes      = int64(1 << 20)
)

type LookupEnv func(string) (string, bool)

type Config struct {
	Address           string
	DataDir           string
	AllowedHosts      []string
	AllowedOrigins    []string
	MaxRequestBytes   int64
	JARBytes          int64
	TextArtifactBytes int64
	PreviewBytes      int64
	TopologyNodes     int
	TopologyEdges     int
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

func Parse(args []string, lookupEnv LookupEnv) (Config, error) {
	if lookupEnv == nil {
		lookupEnv = func(string) (string, bool) { return "", false }
	}

	address := envOr(lookupEnv, "STUDIO_ADDRESS", defaultAddress)
	dataDir := envOr(lookupEnv, "STUDIO_DATA_DIR", "")
	origins := envOr(lookupEnv, "STUDIO_ALLOWED_ORIGINS", "http://127.0.0.1:3000,http://localhost:3000")

	set := flag.NewFlagSet("studio-api", flag.ContinueOnError)
	set.SetOutput(io.Discard)
	set.StringVar(&address, "address", address, "HTTP listen address")
	set.StringVar(&dataDir, "data-dir", dataDir, "persistent data directory")
	set.StringVar(&origins, "allowed-origins", origins, "comma-separated browser origins")
	if err := set.Parse(args); err != nil {
		return Config{}, err
	}
	if set.NArg() != 0 {
		return Config{}, fmt.Errorf("unexpected arguments: %s", strings.Join(set.Args(), " "))
	}

	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		return Config{}, errors.New("data directory is required; use --data-dir or STUDIO_DATA_DIR")
	}
	if err := validateAddress(address); err != nil {
		return Config{}, err
	}
	allowedOrigins, err := parseOrigins(origins)
	if err != nil {
		return Config{}, err
	}

	return Config{
		Address:           address,
		DataDir:           dataDir,
		AllowedHosts:      allowedHosts(address),
		AllowedOrigins:    allowedOrigins,
		MaxRequestBytes:   defaultMaxRequestBytes,
		JARBytes:          defaultJARBytes,
		TextArtifactBytes: defaultTextArtifactBytes,
		PreviewBytes:      defaultPreviewBytes,
		TopologyNodes:     500,
		TopologyEdges:     1000,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}, nil
}

func envOr(lookupEnv LookupEnv, key, fallback string) string {
	if value, ok := lookupEnv(key); ok {
		return value
	}
	return fallback
}

func validateAddress(address string) error {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("invalid address %q: %w", address, err)
	}
	if port == "" {
		return fmt.Errorf("invalid address %q: port is required", address)
	}
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return fmt.Errorf("address %q is not loopback; public binding is not supported", address)
	}
	return nil
}

func allowedHosts(address string) []string {
	host, _, _ := net.SplitHostPort(address)
	hosts := []string{"127.0.0.1", "localhost", "::1"}
	if host != "" && !slices.Contains(hosts, host) {
		hosts = append(hosts, host)
	}
	return hosts
}

func parseOrigins(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	origins := make([]string, 0)
	for raw := range strings.SplitSeq(value, ",") {
		raw = strings.TrimSpace(raw)
		parsed, err := url.Parse(raw)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
			return nil, fmt.Errorf("invalid allowed origin %q", raw)
		}
		origin := parsed.Scheme + "://" + parsed.Host
		if !slices.Contains(origins, origin) {
			origins = append(origins, origin)
		}
	}
	return origins, nil
}
