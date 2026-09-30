package controller

import (
	"bytes"
	"fmt"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"log"
	"os"
	"path/filepath"
	"testing"
)

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

func initTestConfig(t *testing.T) {
	viper.Reset()
	testFile := filepath.Join("testdata", "config.yaml")
	Init("vmx", Version, testFile)
	ViperSet("debug", true)
}

func TestControllerList(t *testing.T) {
	initTestConfig(t)
	vmx, err := NewVMXController()
	require.Nil(t, err)
	fmt.Printf("controller: %+v\n", vmx)
	lines, err := vmx.Files("", FilesOptions{})
	require.Nil(t, err)
	for i, line := range lines {
		fmt.Printf("%d %s\n", i, line)
	}
}
