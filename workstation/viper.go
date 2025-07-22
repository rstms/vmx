package workstation

import (
	"bytes"
	"github.com/spf13/viper"
	"log"
)

var viperPrefix string

func ViperInit(prefix string) {
	viperPrefix = prefix
	if ViperGetBool("debug") {
		var buf bytes.Buffer
		err := viper.WriteConfigTo(&buf)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("config file: %s\n### START ###\n%s\n### END ###\n", viper.ConfigFileUsed(), buf.String())
	}
}

func viperKey(key string) string {
	return "vmx." + key
}

func ViperGetString(key string) string {
	return viper.GetString(viperKey(key))
}

func ViperGetBool(key string) bool {
	return viper.GetBool(viperKey(key))
}

func ViperGetInt(key string) int {
	return viper.GetInt(viperKey(key))
}

func ViperGetInt64(key string) int64 {
	return viper.GetInt64(viperKey(key))
}

func ViperGetStringSlice(key string) []string {
	return viper.GetStringSlice(viperKey(key))
}

func ViperSetDefault(key string, value any) {
	viper.SetDefault(viperKey(key), value)
}

func ViperSet(key string, value any) {
	viper.Set(viperKey(key), value)
}
