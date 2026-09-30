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

func TestPathToName(t *testing.T) {
	initTestConfig(t)
	name, err := PathToName("C:\\dir\\subdir\\file.ext")
	require.Nil(t, err)
	require.Equal(t, "file", name)

	require.Equal(t, "file", name)
	name, err = PathToName("file.ext")
	require.Nil(t, err)
	require.Equal(t, "file", name)
	name, err = PathToName("/file.ext")
	require.Nil(t, err)
	require.Equal(t, "file", name)
	name, err = PathToName("/subdir/subdir/file.ext")
	require.Nil(t, err)
	require.Equal(t, "file", name)
}

func TestPathToNameNormalizeError(t *testing.T) {
	initTestConfig(t)
	_, err := PathToName("D:file.ext")
	require.NotNil(t, err)
	log.Println(err)
}

func TestPathNormalize(t *testing.T) {
	initTestConfig(t)
	path := "C:\\dir\\subdir\\file.ext"
	expected := "/C/dir/subdir/file.ext"
	normalized, err := PathNormalize(path)
	require.Nil(t, err)
	require.Equal(t, expected, normalized)

	path = "c:\\dir\\\\foo\\\\bar"
	expected = "/c/dir/foo/bar"
	normalized, err = PathNormalize(path)
	require.Nil(t, err)
	require.Equal(t, expected, normalized)
}

func TestPathnameFormat(t *testing.T) {
	initTestConfig(t)
	normalized := "/C/dir/subdir/file.ext"
	windowsFormatted := "C:\\dir\\subdir\\file.ext"
	unixFormatted := "/C/dir/subdir/file.ext"

	path, err := PathnameFormat("windows", normalized)
	require.Nil(t, err)
	require.Equal(t, windowsFormatted, path)

	path, err = PathnameFormat("unix", normalized)
	require.Nil(t, err)
	require.Equal(t, unixFormatted, path)
}

func TestPathNoSubdirectory(t *testing.T) {
	initTestConfig(t)
	path := "C:file_without_subdirectory.ext"
	_, err := PathNormalize(path)
	require.NotNil(t, err)
	log.Println(err)
}

func TestPathBareDrive(t *testing.T) {
	initTestConfig(t)
	path := "C:"
	_, err := PathNormalize(path)
	require.NotNil(t, err)
	log.Println(err)
}

func TestPathCompare(t *testing.T) {
	initTestConfig(t)
	a := "C:\\subdir\\file.ext"
	b := "/C/subdir/file.ext"
	ok, err := PathCompare(a, b)
	require.Nil(t, err)
	require.True(t, ok)

	c := "D:\\subdir\\file.ext"
	ok, err = PathCompare(a, c)
	require.Nil(t, err)
	require.False(t, ok)
}

func TestFileListLocalUnix(t *testing.T) {
	initTestConfig(t)
	fmt.Printf("before_viper: %s\n", FormatJSON(viper.AllSettings()))
	ViperSet("hostname", "localhost")
	ViperSet("shell", "sh")
	v, err := NewVMXController()
	require.Nil(t, err)
	vmx := v.(*vmctl)
	require.IsType(t, &vmctl{}, vmx)
	lines, err := vmx.LocalExec("ls", []string{"-l", "."}, nil, nil)
	files, err := ParseFileList("unix", lines)
	require.Nil(t, err)
	require.NotEmpty(t, files)
	for _, file := range files {
		require.IsType(t, VMFile{}, file)
		fmt.Printf("%v\n", file)
	}
	fmt.Printf("after_viper: %s\n", FormatJSON(viper.AllSettings()))

}

func TestFileListRemoteWinexec(t *testing.T) {
	initTestConfig(t)
	fmt.Printf("before_viper: %s\n", FormatJSON(viper.AllSettings()))
	ViperSet("shell", "winexec")

	fmt.Printf("viper: %s\n", FormatJSON(viper.AllSettings()))

	v, err := NewVMXController()
	require.Nil(t, err)
	vmx := v.(*vmctl)
	require.IsType(t, &vmctl{}, vmx)
	require.Equal(t, "winexec", vmx.Shell)
	lines, err := vmx.RemoteExec("dir", []string{"."}, nil, nil)
	require.Nil(t, err)
	require.NotEmpty(t, lines)
	for _, line := range lines {
		log.Printf("%s\n", line)
	}
	fmt.Printf("after_viper: %s\n", FormatJSON(viper.AllSettings()))
}

func TestIsIsoPath(t *testing.T) {
	initTestConfig(t)
	var testData map[string]bool = map[string]bool{
		"iso":           true,
		"iso/":          true,
		"iso/foo":       true,
		"iso/foo/":      true,
		"iso/foo/bar":   true,
		"iso/foo/bar/":  true,
		"foo":           false,
		"foo/":          false,
		"foo/bar":       false,
		"foo/bar/":      false,
		"foo/bar/baz":   false,
		"foo/bar/baz/":  false,
		"":              false,
		"/":             false,
		"/foo":          false,
		"/foo/":         false,
		"/foo/bar":      false,
		"/foo/bar/":     false,
		"/foo/bar/baz":  false,
		"/foo/bar/baz/": false,
	}

	for input, expected := range testData {
		result, err := IsIsoPath(input)
		require.Nil(t, err)
		display := fmt.Sprintf("IsIsoPath(%s) -> %v", input, result)
		require.Equal(t, expected, result, display)
		log.Println(display)
	}
}

func TestFormatIsoPath(t *testing.T) {
	initTestConfig(t)
	var testData map[string]string = map[string]string{
		"":              "/H/vmware/iso",
		"iso":           "/H/vmware/iso",
		"iso/":          "/H/vmware/iso",
		"iso/foo":       "/H/vmware/iso/foo",
		"iso/foo/":      "/H/vmware/iso/foo",
		"iso/foo/bar":   "/H/vmware/iso/foo/bar",
		"iso/foo/bar/":  "/H/vmware/iso/foo/bar",
		"foo":           "/H/vmware/iso/foo",
		"foo/":          "/H/vmware/iso/foo",
		"foo/bar":       "/H/vmware/iso/foo/bar",
		"foo/bar/":      "/H/vmware/iso/foo/bar",
		"foo/bar/baz":   "/H/vmware/iso/foo/bar/baz",
		"foo/bar/baz/":  "/H/vmware/iso/foo/bar/baz",
		"/":             "",
		"/foo":          "",
		"/foo/":         "",
		"/foo/bar":      "",
		"/foo/bar/":     "",
		"/foo/bar/baz":  "",
		"/foo/bar/baz/": "",
	}

	for input, expected := range testData {
		result, err := FormatIsoPath("/H/vmware/iso", input)
		if expected == "" {
			require.NotNil(t, err)
		} else {
			require.Nil(t, err)
		}
		display := fmt.Sprintf("FormatIsoPath(%s) -> %v", input, result)
		require.Equal(t, expected, result, display)
		log.Println(display)
	}
}

func TestFormatIsoPathname(t *testing.T) {
	initTestConfig(t)
	var testData map[string]string = map[string]string{
		"/":        "",
		"":         "",
		"file":     "/H/vmware/iso/file.iso",
		"file/":    "",
		"file.iso": "/H/vmware/iso/file.iso",

		"iso/file":         "/H/vmware/iso/file.iso",
		"iso/file.iso":     "/H/vmware/iso/file.iso",
		"iso/foo/file":     "/H/vmware/iso/foo/file.iso",
		"iso/foo/file.iso": "/H/vmware/iso/foo/file.iso",

		"foo/file":              "/H/vmware/iso/foo/file.iso",
		"foo/file.iso":          "/H/vmware/iso/foo/file.iso",
		"foo/bar/file":          "/H/vmware/iso/foo/bar/file.iso",
		"foo/bar/file.iso":      "/H/vmware/iso/foo/bar/file.iso",
		"foo/bar/file/":         "",
		"/file.iso":             "/file.iso",
		"/file":                 "/file.iso",
		"/foo/file":             "/foo/file.iso",
		"/foo/bar/":             "",
		"/foo/file.iso":         "/foo/file.iso",
		"/foo/bar/file":         "/foo/bar/file.iso",
		"/foo/bar/file.iso":     "/foo/bar/file.iso",
		"/foo/bar/baz/file":     "/foo/bar/baz/file.iso",
		"/foo/bar/baz/file.iso": "/foo/bar/baz/file.iso",
		"/foo/bar/baz/":         "",
	}
	for input, expected := range testData {
		result, err := FormatIsoPathname("/H/vmware/iso", input)
		if expected == "" {
			require.NotNil(t, err)
		} else {
			require.Nil(t, err)
		}
		display := fmt.Sprintf("FormatIsoPathname(%s) -> %v", input, result)
		require.Equal(t, expected, result, display)
		log.Println(display)
	}
}

func TestPathFormat(t *testing.T) {
	initTestConfig(t)
	path, err := PathFormat("unix", "/sub/dir")
	require.Nil(t, err)
	require.Equal(t, "/sub/dir", path)

	path, err = PathFormat("unix", "/sub/dir/")
	require.Nil(t, err)
	require.Equal(t, "/sub/dir", path)

	path, err = PathFormat("windows", "///C//foo/bar.ext/baz/")
	require.Nil(t, err)
	require.Equal(t, "C:\\foo\\bar.ext\\baz", path)
}
