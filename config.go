package main

import (
	"io/ioutil"
	"log"

	"gopkg.in/yaml.v2"
)

const YAML_CONFIG = "./sample.yml"

/*
	To load config from YAML you need to do:

	cfg := config{}
	cfg.readConfig()
	fmt.Println(cfg)
*/

type config struct {
	BaseURL string `yaml:"base_url"`

	Dirs struct {
		Origins  string `yaml:"origins"`
		Compared string `yaml:"compared"`
	} `yaml:"dirs"`

	Routes []struct {
		Path        string `yaml:"path"`
		Name        string `yaml:"name,omitempty"`
		Resolutions map[interface{}][]struct {
			Type string `yaml:"type"`
			URL  string `yaml:"url"`
		} `yaml:"resolutions"`
	}
}

func (c *config) readConfig() *config {

	yamlFile, err := ioutil.ReadFile(YAML_CONFIG)
	if err != nil {
		log.Println("YAML file isn't found", err)
	}

	err = yaml.Unmarshal(yamlFile, c)
	if err != nil {
		log.Println("yaml.Unmarshal error", err)
	}

	return c

}
