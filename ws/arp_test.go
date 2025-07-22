package ws

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestArpScanWindows(t *testing.T) {

	lines := []string{
		"Interface: 192.168.66.16 --- 0x6",
		"Internet Address      Physical Address      Type",
		"192.168.66.1          00-e2-69-3e-73-e6     dynamic",
		"192.168.66.6          00-e2-69-3e-73-e6     dynamic",
		"192.168.66.7          00-e2-69-3e-73-e6     dynamic",
		"192.168.66.8          30-05-5c-71-86-1b     dynamic",
		"192.168.66.11         e8-6a-64-04-01-ad     dynamic",
	}
	mac := "30:05:5c:71:86:1b"
	ip := "192.168.66.8"
	addr, err := ArpScan(mac, lines)
	require.Nil(t, err)
	require.Equal(t, ip, addr)
}

func TestArpScanOpenBSD(t *testing.T) {

	lines := []string{
		"Host                                 Ethernet Address    Netif Expire    Flags",
		"192.168.33.1                         b0:39:56:20:82:ea     em1 19m35s",
		"192.168.66.32                        (incomplete)          em0 expired",
		"192.168.33.3                         00:e2:69:3e:73:e7     em1 permanent l",
		"192.168.66.1                         00:e2:69:3e:73:e6     em0 permanent l",
		"192.168.66.4                         a0:b3:cc:e3:cc:a4     em0 5m8s",
		"192.168.66.6                         00:e2:69:3e:73:e6     em0 permanent l",
	}
	mac := "00:e2:69:3e:73:e6"
	ip := "192.168.66.1"
	addr, err := ArpScan(mac, lines)
	require.Nil(t, err)
	require.Equal(t, ip, addr)
}

func TestArpScanLinux(t *testing.T) {

	lines := []string{
		"Address                  HWtype  HWaddress           Flags Mask            Iface",
		"192.168.67.1             ether   00:e2:69:3e:73:e6   C                     ens33",
	}
	mac := "00:e2:69:3e:73:e6"
	ip := "192.168.67.1"
	addr, err := ArpScan(mac, lines)
	require.Nil(t, err)
	require.Equal(t, ip, addr)
}
