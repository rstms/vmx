package ws

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSizeParse(t *testing.T) {

	size, err := SizeParse("123")
	require.Nil(t, err)
	require.Equal(t, int64(123), size)

	size, err = SizeParse("1024")
	require.Nil(t, err)
	require.Equal(t, int64(1024), size)

	size, err = SizeParse("2048")
	require.Nil(t, err)
	require.Equal(t, int64(2048), size)

	size, err = SizeParse("1K")
	require.Nil(t, err)
	require.Equal(t, int64(1024), size)

	size, err = SizeParse("1.5K")
	require.Nil(t, err)
	require.Equal(t, int64(1024+(1024/2)), size)

	size, err = SizeParse("1M")
	require.Nil(t, err)
	require.Equal(t, int64(1024*1024), size)

	size, err = SizeParse("1G")
	require.Nil(t, err)
	require.Equal(t, int64(1024*1024*1024), size)

	G := int64(1024 * 1024 * 1024)
	size, err = SizeParse("1.25G")
	require.Nil(t, err)
	require.Equal(t, G+G/4, size)

	size, err = SizeParse("1T")
	require.Nil(t, err)
	require.Equal(t, int64(1024*1024*1024*1024), size)

	size, err = SizeParse(".001K")
	require.Nil(t, err)
	require.Equal(t, int64(1), size)

	size, err = SizeParse(".01K")
	require.Nil(t, err)
	require.Equal(t, int64(10), size)

	size, err = SizeParse(".1K")
	require.Nil(t, err)
	require.Equal(t, int64(102), size)

	size, err = SizeParse("1P")
	require.Nil(t, err)
	require.Equal(t, int64(1024*1024*1024*1024*1024), size)

	size, err = SizeParse("1PB")
	require.Nil(t, err)
	require.Equal(t, int64(1024*1024*1024*1024*1024), size)
}

func TestSizeFormat(t *testing.T) {

	size := FormatSize(int64(0))
	require.Equal(t, "0", size)

	size = FormatSize(int64(123))
	require.Equal(t, "123", size)

	size = FormatSize(int64(1024))
	require.Equal(t, "1K", size)

	size = FormatSize(int64(2048))
	require.Equal(t, "2K", size)

	size = FormatSize(int64(1342177280))
	require.Equal(t, "1.25G", size)

	size = FormatSize(int64(1025))
	require.Equal(t, "1K", size)

	size = FormatSize(int64(1024 + 256))
	require.Equal(t, "1.25K", size)

	size = FormatSize(int64(1024 + 512))
	require.Equal(t, "1.5K", size)
}
