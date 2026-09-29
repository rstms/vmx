package controller

import (
	"github.com/stretchr/testify/require"
	"log"
	"path/filepath"
	"testing"
)

/*
func dumpConfig(t *testing.T) {
	filename := viper.ConfigFileUsed()
	log.Printf("configFileUsed: %s\n", filename)
	dir, err := os.Getwd()
	require.Nil(t, err)
	log.Printf("current directory: %s\n", dir)
	var buf bytes.Buffer
	err = viper.WriteConfigTo(&buf)
	require.Nil(t, err)
	log.Println(buf.String())
}
*/

func initModifyTestConfig(t *testing.T) {
	testFile := filepath.Join("testdata", "config.yaml")
	Init("test", Version, testFile)
	ViperSet("debug", true)
}

func TestModifyBoot(t *testing.T) {
	initModifyTestConfig(t)
	require.Nil(t, nil)
	log.Printf("ModifyBoot")
}
