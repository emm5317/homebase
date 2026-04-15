package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server     ServerConfig            `yaml:"server"`
	Location   LocationConfig          `yaml:"location"`
	Metra      MetraConfig             `yaml:"metra"`
	Garbage    GarbageConfig           `yaml:"garbage"`
	Reminders  []ReminderEntry         `yaml:"reminders"`
	Meals      MealsConfig             `yaml:"meals"`
	QuietHours string                  `yaml:"quiet_hours,omitempty"`
	Senders    map[string]string       `yaml:"allowed_senders"`

	// Loaded from environment variables
	MetraAPIKey       string `yaml:"-"`
	OWMAPIKey         string `yaml:"-"`
	SkylightEmail     string `yaml:"-"`
	SkylightPassword  string `yaml:"-"`
	SkylightFrameID   string `yaml:"-"`
	IngestBearerToken string `yaml:"-"`
}

type ReminderEntry struct {
	Date string `yaml:"date"` // "YYYY-MM-DD" (one-off) or "MM-DD" (annual)
	Text string `yaml:"text"` // e.g. "Mom's birthday"
	Icon string `yaml:"icon"` // optional emoji, defaults to 🎂 if empty
}

type MealsConfig struct {
	Rotation  []string `yaml:"rotation"`
	StartDate string   `yaml:"start_date"` // YYYY-MM-DD anchors day 0
}

type ServerConfig struct {
	Port int `yaml:"port"`
}

type LocationConfig struct {
	Lat      float64 `yaml:"lat"`
	Lon      float64 `yaml:"lon"`
	Timezone string  `yaml:"timezone"`
}

type MetraConfig struct {
	Route       string `yaml:"route"`
	Station     string `yaml:"station"`
	Destination string `yaml:"destination"`
	// Optional overrides for the Metra GTFS-RT feed URLs. When empty, the
	// provider falls back to the public defaults documented at
	// https://metra.com/metra-gtfs-api.
	TripUpdatesURL string `yaml:"trip_updates_url,omitempty"`
	AlertsURL      string `yaml:"alerts_url,omitempty"`
	WalkMinutes    int    `yaml:"walk_minutes,omitempty"`
}

type GarbageConfig struct {
	PickupDay      string   `yaml:"pickup_day"`
	RecyclingWeeks string   `yaml:"recycling_weeks"`
	YardWaste      string   `yaml:"yard_waste"`
	Holidays       []string `yaml:"holidays"`
}

func Load(configPath string) (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{Port: 8080},
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	// Load .env file if it exists (simple key=value parsing)
	loadDotenv()

	cfg.MetraAPIKey = os.Getenv("METRA_API_KEY")
	cfg.OWMAPIKey = os.Getenv("OWM_API_KEY")
	cfg.SkylightEmail = os.Getenv("SKYLIGHT_EMAIL")
	cfg.SkylightPassword = os.Getenv("SKYLIGHT_PASSWORD")
	cfg.SkylightFrameID = os.Getenv("SKYLIGHT_FRAME_ID")
	cfg.IngestBearerToken = os.Getenv("INGEST_BEARER_TOKEN")

	return cfg, nil
}

func loadDotenv() {
	data, err := os.ReadFile(".env")
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		// Don't override existing env vars (systemd EnvironmentFile takes precedence)
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
}
