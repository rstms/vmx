package workstation

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
)

const vmxTemplate string = `.encoding = "UTF-8"
config.version = "8"
virtualHW.version = "19"
displayName = "${Name}"
numvcpus = "${CpuCount}"
memsize = "${MemorySize}"
guestOS = "${GuestOS}"
pciBridge0.present = "TRUE"
pciBridge4.present = "TRUE"
pciBridge4.virtualDev = "pcieRootPort"
pciBridge4.functions = "8"
pciBridge5.present = "TRUE"
pciBridge5.virtualDev = "pcieRootPort"
pciBridge5.functions = "8"
pciBridge6.present = "TRUE"
pciBridge6.virtualDev = "pcieRootPort"
pciBridge6.functions = "8"
pciBridge7.present = "TRUE"
pciBridge7.virtualDev = "pcieRootPort"
pciBridge7.functions = "8"
nvme0.present = "TRUE"
nvme0:0.fileName = "${VMDKFile}"
nvme0:0.present = "TRUE"
floppy0.present = "FALSE"
ide1:0.present = "FALSE"
ethernet0.present = "FALSE"
vmx.scoreboard.enabled = "FALSE"
tools.syncTime = "${HostTimeSync}"
time.synchronize.continue = "${HostTimeSync}"
time.synchronize.restore = "${HostTimeSync}"
time.synchronize.resume.disk = "${HostTimeSync}"
time.synchronize.shrink = "${HostTimeSync}"
time.synchronize.tools.startup = "${HostTimeSync}"
guestTimeZone = "${GuestTimeZone}"
isolation.tools.dnd.disable = "${EnableDragAndDrop}"
isolation.tools.copy.disable = "${EnableClipboard}"
isolation.tools.paste.disable = "${EnableClipboard}"
`

var DISPLAY_NAME = regexp.MustCompile(`^displayName = "([^"]+)"`)

var VMGuestOSValues map[string]string = map[string]string{
	"other":   "other-64",
	"openbsd": "other-64",
	"windows": "windows9-64",
	"ubuntu":  "ubuntu-64",
	"debian":  "debian10-64",
	"linux5":  "other5xlinux-64",
	"linux4":  "other4xlinux-64",
}

type VMX struct {
	name    string
	ramSize string
	guestOS string
	lines   []string
	params  map[string]string
}

func GenerateVMX(name string, options CreateOptions) (*VMX, error) {
	vmx := VMX{}
	err := vmx.Generate(name, options)
	if err != nil {
		return nil, err
	}
	return &vmx, nil
}

func getGuestOS(options CreateOptions) (string, error) {
	for osKey, osValue := range VMGuestOSValues {
		if options.GuestOS == osKey {
			return osValue, nil
		}
	}
	return "", fmt.Errorf("unexpected GuestOS: %s", options.GuestOS)
}

func (v *VMX) Generate(name string, options CreateOptions) error {

	v.name = name

	size, err := SizeParse(options.MemorySize)
	if err != nil {
		return err
	}
	v.ramSize = fmt.Sprintf("%d", size/MB)

	guestOS, err := getGuestOS(options)
	if err != nil {
		return err
	}
	v.guestOS = guestOS

	data, err := json.Marshal(&options)
	if err != nil {
		return err
	}
	var omap map[string]any
	err = json.Unmarshal(data, &omap)
	if err != nil {
		return err
	}

	v.params = make(map[string]string)
	for key, value := range omap {
		v.params[key] = fmt.Sprintf("%v", value)
	}

	content := os.Expand(vmxTemplate, v.GetConfig)

	err = v.Write([]byte(content))
	if err != nil {
		return err
	}

	fmt.Printf("Generate: IsoPath=%s\n", options.IsoPath)

	v.SetEFI(options.EFIBoot)
	v.SetISO(options.IsoPath != "", options.IsoAttached, options.IsoPath)
	v.SetEthernet(options.EthernetPresent, options.MacAddress)
	v.SetSerial(options.SerialPipe != "", options.SerialPipe)

	return nil
}

func (v *VMX) Write(data []byte) error {
	v.lines = strings.Split(strings.TrimSpace(string(data)), "\n")
	for _, line := range v.lines {
		match := DISPLAY_NAME.FindStringSubmatch(line)
		if len(match) == 2 {
			v.name = match[1]
		}
	}
	return nil
}

func (v *VMX) Read() ([]byte, error) {
	return []byte(strings.Join(v.lines, "\n")), nil
}

func (v *VMX) GetConfig(key string) string {

	switch key {
	case "Name":
		return v.name
	case "GuestOS":
		return v.guestOS
	case "MemorySize":
		return v.ramSize
	case "VMDKFile":
		return v.name + ".vmdk"
	}
	if v.params[key] == "true" {
		return "TRUE"
	}
	if v.params[key] == "false" {
		return "FALSE"
	}
	return v.params[key]
}

func (v *VMX) removePrefix(prefix string) {
	lines := []string{}
	for _, line := range v.lines {
		if !strings.HasPrefix(line, prefix) {
			lines = append(lines, line)
		}
	}
	v.lines = lines
}

func (v *VMX) addLine(line string) {
	v.lines = append(v.lines, line)
}

func (v *VMX) SetEFI(efi bool) {
	v.removePrefix("firmware =")
	if efi {
		v.lines = append(v.lines, `firmware = "efi"`)
	}
}

func (v *VMX) SetISO(present, attached bool, path string) {
	v.removePrefix("ide1:0.")
	if !present {
		v.addLine(`ide1:0.present = "FALSE"`)
		return
	}
	v.addLine(`ide1:0.present = "TRUE"`)
	v.addLine(`ide1:0.deviceType = "cdrom-image"`)
	v.addLine(`ide1:0.fileName = "` + path + `"`)
	if attached {
		v.addLine(`ide1:0.startConnected = "TRUE"`)
	} else {
		v.addLine(`ide1:0.startConnected = "FALSE"`)
	}
}

func (v *VMX) SetEthernet(present bool, mac string) {
	v.removePrefix("ethernet0.")
	if !present {
		v.addLine(`ethernet0.present = "FALSE"`)
		return
	}
	v.addLine(`ethernet0.present = "TRUE"`)
	v.addLine(`ethernet0.virtualDev = "e1000"`)
	if mac != "" {
		v.addLine(`ethernet0.address = "` + mac + `"`)
		v.addLine(`ethernet0.addressType = "static"`)
	} else {
		v.addLine(`ethernet0.addressType = "generated"`)
	}
}

func (v *VMX) SetSerial(present bool, pipe string) {
	v.removePrefix("serial0.")
	if !present {
		v.addLine(`serial0.present = "FALSE"`)
		return
	}

	v.addLine(`serial0.present = "TRUE"`)
	v.addLine(`serial0.fileType = "pipe"`)
	v.addLine(`serial0.fileName = "\\\\.\\pipe\\` + pipe + `"`)
	v.addLine(`serial0.pipe.endPoint = "client"`)
	v.addLine(`serial0.tryNoRxLoss = "TRUE"`)
}
