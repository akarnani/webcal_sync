package main

import (
	"os"
	"testing"
)

func TestGetConfig(t *testing.T) {
	// Save original config file if it exists
	originalConfig, _ := os.ReadFile("config.yml")
	defer func() {
		if originalConfig != nil {
			os.WriteFile("config.yml", originalConfig, 0644)
		} else {
			os.Remove("config.yml")
		}
	}()

	t.Run("valid config with single entry", func(t *testing.T) {
		configContent := `- url: "https://example.com/calendar.ics"
  color_id: "1"
  id_format: "url"
  reminder: 30
`
		err := os.WriteFile("config.yml", []byte(configContent), 0644)
		if err != nil {
			t.Fatalf("Failed to write test config: %v", err)
		}

		configs := getConfig()

		if len(configs) != 1 {
			t.Fatalf("Expected 1 config, got %d", len(configs))
		}

		cfg := configs[0]
		if cfg.URL != "https://example.com/calendar.ics" {
			t.Errorf("Expected URL 'https://example.com/calendar.ics', got %q", cfg.URL)
		}
		if cfg.ColorID != "1" {
			t.Errorf("Expected ColorID '1', got %q", cfg.ColorID)
		}
		if cfg.IDFormat != "url" {
			t.Errorf("Expected IDFormat 'url', got %q", cfg.IDFormat)
		}
		if cfg.Reminder != 30 {
			t.Errorf("Expected Reminder 30, got %d", cfg.Reminder)
		}
	})

	t.Run("valid config with multiple entries", func(t *testing.T) {
		configContent := `- url: "https://example.com/calendar1.ics"
  color_id: "1"
  id_format: "url"
  reminder: 30
- url: "https://example.com/calendar2.ics"
  color_id: "2"
  id_format: ""
  reminder: 0
`
		err := os.WriteFile("config.yml", []byte(configContent), 0644)
		if err != nil {
			t.Fatalf("Failed to write test config: %v", err)
		}

		configs := getConfig()

		if len(configs) != 2 {
			t.Fatalf("Expected 2 configs, got %d", len(configs))
		}

		if configs[0].URL != "https://example.com/calendar1.ics" {
			t.Errorf("Expected first URL 'https://example.com/calendar1.ics', got %q", configs[0].URL)
		}
		if configs[1].URL != "https://example.com/calendar2.ics" {
			t.Errorf("Expected second URL 'https://example.com/calendar2.ics', got %q", configs[1].URL)
		}
	})

	t.Run("config with default values", func(t *testing.T) {
		configContent := `- url: "https://example.com/calendar.ics"
`
		err := os.WriteFile("config.yml", []byte(configContent), 0644)
		if err != nil {
			t.Fatalf("Failed to write test config: %v", err)
		}

		configs := getConfig()

		if len(configs) != 1 {
			t.Fatalf("Expected 1 config, got %d", len(configs))
		}

		cfg := configs[0]
		if cfg.URL != "https://example.com/calendar.ics" {
			t.Errorf("Expected URL 'https://example.com/calendar.ics', got %q", cfg.URL)
		}
		if cfg.ColorID != "" {
			t.Errorf("Expected default ColorID '', got %q", cfg.ColorID)
		}
		if cfg.IDFormat != "" {
			t.Errorf("Expected default IDFormat '', got %q", cfg.IDFormat)
		}
		if cfg.Reminder != 0 {
			t.Errorf("Expected default Reminder 0, got %d", cfg.Reminder)
		}
	})

	t.Run("missing config file panics", func(t *testing.T) {
		os.Remove("config.yml")

		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for missing config file")
			}
		}()

		getConfig()
	})

	t.Run("invalid YAML panics", func(t *testing.T) {
		configContent := `this is not valid yaml: [[[`
		err := os.WriteFile("config.yml", []byte(configContent), 0644)
		if err != nil {
			t.Fatalf("Failed to write test config: %v", err)
		}

		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for invalid YAML")
			}
		}()

		getConfig()
	})
}
