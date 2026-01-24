package main

import (
	"log/slog"
	"os"

	"gopkg.in/yaml.v2"
)

type Config struct {
	URL      string `yaml:"url"`
	ColorID  string `yaml:"color_id"`
	IDFormat string `yaml:"id_format"`
	Reminder int    `yaml:"reminder"`
}

func getConfig() []Config {
	f, err := os.ReadFile("config.yml")
	if err != nil {
		slog.Error("Unable to read config file", "error", err)
		panic(err)
	}

	var cfg []Config

	if err := yaml.UnmarshalStrict(f, &cfg); err != nil {
		slog.Error("Unable to parse config file", "error", err)
		panic(err)
	}

	return cfg

}
