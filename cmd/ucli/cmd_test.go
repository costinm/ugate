package main

import (
	"bytes"
	"log"
	"testing"

	"github.com/costinm/mk8s/gcp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func TestAuto(t *testing.T) {
	obj := &gcp.MetricListRequest{}

	cmd := &cobra.Command{}
	ParseStruct(cmd, obj)

}

func TestViper(t *testing.T) {
	var yamlExample = []byte(`
Hacker: true
name: steve
hobbies:
- skateboarding
- snowboarding
- go
clothing:
  jacket: leather
  trousers: denim
age: 35
eyes : brown
beard: true
`)
	viper.SetConfigType("yaml")
	viper.ReadConfig(bytes.NewBuffer(yamlExample))

	log.Println("Viper", viper.GetString("clothing.jacket"), viper.AllSettings())

	var x struct {
		Hobbies []string
		Clothing struct {
			Jacket string
			Trousers string
		}
	}

	// based on 'mapstructure' lib to map[string] interfaces to structs
	viper.Unmarshal(&x)

	log.Println(x)
}
