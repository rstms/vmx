package controller

import (
	"github.com/rstms/winexec/client"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

func initClient(t *testing.T) *client.WinexecClient {
	initTestConfig(t)
	c, err := client.NewWinexecClient("winexec.client")
	require.Nil(t, err)
	return c
}

func TestFileDownload(t *testing.T) {
	filesDir := filepath.Join("testdata", "files")
	if IsDir(filesDir) {
		err := os.RemoveAll(filesDir)
		require.Nil(t, err)
	}
	err := os.Mkdir(filesDir, 0700)
	require.Nil(t, err)
	c := initClient(t)
	dst := filepath.Join(filesDir, "hosts")
	src := "/c/windows/system32/drivers/etc/hosts"
	err = c.Download(dst, src)
	require.Nil(t, err)
	exists := IsFile(dst)
	require.True(t, exists)
}

func TestFileUpload(t *testing.T) {
	c := initClient(t)
	c.RemoveAll("/c/tmp/testdir")
	err := c.MkdirAll("/c/tmp/testdir", 0700)
	require.Nil(t, err)
	src := filepath.Join("testdata", "config.yaml")
	dst := "/c/tmp/testdir/config.yaml"
	err = c.Upload(dst, src, false)
	require.Nil(t, err)
	err = c.RemoveAll("/c/tmp/testdir")
	require.Nil(t, err)
}
