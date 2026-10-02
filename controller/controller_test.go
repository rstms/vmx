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
	ViperSet("verbose", true)
	vmx, err := NewVMXController()
	require.Nil(t, err)
	fmt.Printf("controller: %+v\n", vmx)
	lines, err := vmx.Files("", FilesOptions{})
	require.Nil(t, err)
	for i, line := range lines {
		fmt.Printf("%d %s\n", i, line)
	}
}

func TestControllerParseUSB(t *testing.T) {
	initTestConfig(t)
	ViperSet("verbose", true)

	device, err := ParseUSBDevice("0x1234")
	require.Nil(t, err)
	require.Equal(t, USBDevice{VID: "1234", PID: "", Autoclean: false}, *device)
	require.Equal(t, "vid:0x1234 autoclean:0", device.String())
	log.Printf("%s\n", device.FormatAutoconnectLine("usb", 0))
	log.Printf("%s\n", device.FormatQuirksLine(0))
	log.Printf("string: %s\n", device.String())

	device, err = ParseUSBDevice("1234:abcd")
	require.Nil(t, err)
	require.Equal(t, USBDevice{VID: "1234", PID: "abcd", Autoclean: false}, *device)
	require.Equal(t, "vid:0x1234 pid:0xabcd autoclean:0", device.String())
	log.Printf("%s\n", device.FormatAutoconnectLine("usb", 1))
	log.Printf("%s\n", device.FormatQuirksLine(1))
	log.Printf("string: %s\n", device.String())

	device, err = ParseUSBDevice("vid:0x1234 autoclean:0")
	require.Nil(t, err)
	require.Equal(t, USBDevice{VID: "1234", PID: "", Autoclean: false}, *device)
	require.Equal(t, "vid:0x1234 autoclean:0", device.String())
	log.Printf("%s\n", device.FormatAutoconnectLine("usb", 2))
	log.Printf("%s\n", device.FormatQuirksLine(2))
	log.Printf("string: %s\n", device.String())

	device, err = ParseUSBDevice("baad-f00d autoclean:1")
	require.Nil(t, err)
	require.Equal(t, USBDevice{VID: "baad", PID: "f00d", Autoclean: true}, *device)
	require.Equal(t, "vid:0xbaad pid:0xf00d autoclean:1", device.String())
	log.Printf("%s\n", device.FormatAutoconnectLine("usb", 3))
	log.Printf("%s\n", device.FormatQuirksLine(3))
	log.Printf("string: %s\n", device.String())
}

func TestControllerRegex(t *testing.T) {
	match := VID_PATTERN.FindStringSubmatch("0xabcd")
	require.Equal(t, 2, len(match))
}
