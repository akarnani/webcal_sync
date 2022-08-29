package main

import (
	"os"

	"gopkg.in/yaml.v2"
)

type Config struct {
	URL      string `yaml:"url"`
	ColorID  string `yaml:"color_id"`
	IDFormat string `yaml:"id_format"`
}

func getConfig() []Config {
	f, err := os.ReadFile("config.yml")
	if err != nil {
		panic(err)
	}

	var cfg []Config

	if err := yaml.UnmarshalStrict(f, &cfg); err != nil {
		panic(err)
	}

	return cfg

}
