package ws

import (
	"log"
	"regexp"
	"strings"
)

var ARP_ADDR = regexp.MustCompile(`.*?((?:\d\d*\.){3}\d\d*).*((?:(?:[[:xdigit:]]{2}[-:])){5}[[:xdigit:]]{2}).*`)

func ArpScan(mac string, lines []string) (string, error) {
	colonsMac := strings.ReplaceAll(mac, "-", ":")
	dashesMac := strings.ReplaceAll(mac, ":", "-")
	for _, line := range lines {
		m := ARP_ADDR.FindStringSubmatch(line)
		if len(m) == 3 {
			//log.Printf("ip=%s mac=%s\n", m[1], m[2])
			if m[2] == colonsMac || m[2] == dashesMac {
				//log.Printf("MATCHED: %s\n", line)
				return m[1], nil
			}
		}
	}
	return "", nil
}

func (v *vmctl) ArpQuery(vm *VM) (string, error) {
	//log.Printf("trying arp query for %s\n", vm.MacAddress)
	var command string
	switch v.Remote {
	case "windows":
		command = "arp -a"
	case "openbsd":
		command = "arp -an"
	case "linux":
		command = "arp -n"
	default:
		log.Printf("WARNING: arp query not implemented for remote os: '%s'", v.Remote)
		return "", nil
	}
	lines, err := v.RemoteExec(command, nil)
	if err != nil {
		return "", Fatal(err)
	}
	addr, err := ArpScan(vm.MacAddress, lines)
	if err != nil {
		return "", Fatal(err)
	}
	return addr, nil
}
