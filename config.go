package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"gopkg.in/yaml.v2"
)

const yamlConfigPath = "./sample.yml"
const jsonConfigPath = "./sample.json"

/*
	To load config from YAML/JSON you need to do:

	cfg := config{}
	cfg.readConfigYML() // or cfg.readConfigJSON()
	fmt.Println(cfg)
*/

type config struct {
	BaseURL string `yaml:"base_url" json:"origins"`

	Dirs struct {
		Origins  string `yaml:"origins" json:"origins"`
		Compared string `yaml:"compared" json:"compared"`
	} `yaml:"dirs" json:"dirs"`

	Routes []struct {
		Path        string `yaml:"path" json:"path"`
		Name        string `yaml:"name,omitempty" json:"name,omitempty"`
		Resolutions map[interface{}][]struct {
			Type string `yaml:"type" json:"type"`
			URL  string `yaml:"url" json:"url"`
		} `yaml:"resolutions" json:"resolutions"`
	}
}

func (c *config) readConfigYML() *config {

	yamlFile, err := ioutil.ReadFile(yamlConfigPath)
	if err != nil {
		log.Println("YAML file isn't found", err)
	}

	err = yaml.Unmarshal(yamlFile, c)
	if err != nil {
		log.Println("yaml.Unmarshal error", err)
	}

	return c

}

func (c *config) readConfigJSON() *config {

	jsonFile, err := os.Open(jsonConfigPath)
	defer jsonFile.Close()

	if err != nil {
		fmt.Println(err.Error())
	}

	jsonParser := json.NewDecoder(jsonFile)
	jsonParser.Decode(&c)

	return c

}
