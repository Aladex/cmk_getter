package config

import (
	"github.com/jinzhu/configor"
)

var ConfigCmkGetter Config

func init() {
	// Read config file
	ConfigCmkGetter, _ = ReadConfig()
}

func (c *Config) New() (*Config, error) {
	// Read config file from config/config.yaml
	err := configor.Load(c, "config.yaml")
	if err != nil {
		panic(err)
	}
	return c, nil
}

type Config struct {
	// Config struct for config file
	Listen   string   `yaml:"listen"`
	Port     int      `yaml:"port"`
	Domain   string   `json:"domain" yaml:"domain"`
	Site     string   `json:"site" yaml:"site"`
	Folders  []string `json:"folders" yaml:"folders"`
	Username string   `json:"username" yaml:"username"`
	Password string   `json:"password" yaml:"password"`
	Polling  int      `json:"polling" yaml:"polling"`
}

func ReadConfig() (Config, error) {
	// Read config file from config/config.yaml
	var config Config
	err := configor.Load(&config, "config.yaml")
	if err != nil {
		return config, err
	}
	return config, nil
}
