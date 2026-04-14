package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	const content = `
server:
  port: 9090

location:
  lat: 41.8009
  lon: -87.9331
  timezone: America/Chicago

metra:
  route: BNSF
  station: HINSDALE
  destination: CUS

garbage:
  pickup_day: thursday
  recycling_weeks: even
  yard_waste: april-november
  holidays:
    - "2026-07-04"
    - "2026-12-25"

allowed_senders:
  "eric@example.com": Eric
  "kayla@example.com": Kayla
`

	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Set env vars (simulate systemd EnvironmentFile)
	t.Setenv("OWM_API_KEY", "test-owm-key")
	t.Setenv("METRA_API_KEY", "test-metra-key")
	t.Setenv("INGEST_BEARER_TOKEN", "test-token")

	// Change to a directory without a .env file so loadDotenv is a no-op
	t.Chdir(dir)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Server.Port != 9090 {
		t.Errorf("Server.Port = %d, want 9090", cfg.Server.Port)
	}
	if cfg.Location.Lat != 41.8009 {
		t.Errorf("Location.Lat = %v, want 41.8009", cfg.Location.Lat)
	}
	if cfg.Location.Timezone != "America/Chicago" {
		t.Errorf("Location.Timezone = %q", cfg.Location.Timezone)
	}
	if cfg.Garbage.PickupDay != "thursday" {
		t.Errorf("Garbage.PickupDay = %q", cfg.Garbage.PickupDay)
	}
	if len(cfg.Garbage.Holidays) != 2 {
		t.Errorf("Garbage.Holidays = %v", cfg.Garbage.Holidays)
	}
	if cfg.Senders["eric@example.com"] != "Eric" {
		t.Errorf("Senders[eric] = %q", cfg.Senders["eric@example.com"])
	}
	if cfg.OWMAPIKey != "test-owm-key" {
		t.Errorf("OWMAPIKey = %q", cfg.OWMAPIKey)
	}
	if cfg.IngestBearerToken != "test-token" {
		t.Errorf("IngestBearerToken = %q", cfg.IngestBearerToken)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadDotenv(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	const content = `
# A comment
KEY1=value1
KEY2=value with spaces
KEY3 = trimmed

EMPTY_LINE_BELOW=ok
`
	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Chdir(dir)
	// Clear any existing values to ensure dotenv sets them
	os.Unsetenv("KEY1")
	os.Unsetenv("KEY2")
	os.Unsetenv("KEY3")
	os.Unsetenv("EMPTY_LINE_BELOW")

	loadDotenv()

	tests := map[string]string{
		"KEY1":             "value1",
		"KEY2":             "value with spaces",
		"KEY3":             "trimmed",
		"EMPTY_LINE_BELOW": "ok",
	}
	for k, want := range tests {
		if got := os.Getenv(k); got != want {
			t.Errorf("Getenv(%q) = %q, want %q", k, got, want)
		}
	}
}

func TestLoadDotenvDoesNotOverrideExisting(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	os.WriteFile(envPath, []byte("EXISTING_KEY=from_dotenv\n"), 0644)

	t.Chdir(dir)
	t.Setenv("EXISTING_KEY", "from_systemd")

	loadDotenv()

	if got := os.Getenv("EXISTING_KEY"); got != "from_systemd" {
		t.Errorf("dotenv overrode env var: got %q, want %q", got, "from_systemd")
	}
}
