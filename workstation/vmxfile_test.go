package workstation

import (
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"log"
	"testing"
)

func initTestConfig(t *testing.T) {
	viper.SetConfigFile("testdata/config.yaml")
	err := viper.ReadInConfig()
	require.Nil(t, err)
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

	formatted, err := PathFormat("windows", normalized)
	require.Nil(t, err)
	require.Equal(t, path, formatted)
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

func TestFileListUnix(t *testing.T) {
	initTestConfig(t)
	viper.Set("debug", true)
	viper.Set("verbose", true)
	viper.Set("relay", "")
	viper.Set("hostname", "localhost")
	v, err := NewController()
	require.Nil(t, err)
	vmx := v.(*vmctl)
	require.IsType(t, &vmctl{}, vmx)
	_, lines, _, err := vmx.Exec("sh", []string{"-c", "ls -l ."}, "")
	files, err := ParseFileList("unix", lines)
	require.Nil(t, err)
	require.NotEmpty(t, files)
	for _, file := range files {
		require.IsType(t, VMFile{}, file)
		log.Printf("%v\n", file)
	}

}
