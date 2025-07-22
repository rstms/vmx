package ws

import (
	"encoding/json"
	"fmt"
	"log"
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
pciBridge4.functions = "8"
pciBridge4.present = "TRUE"
pciBridge4.virtualDev = "pcieRootPort"
pciBridge5.functions = "8"
pciBridge5.present = "TRUE"
pciBridge5.virtualDev = "pcieRootPort"
pciBridge6.functions = "8"
pciBridge6.present = "TRUE"
pciBridge6.virtualDev = "pcieRootPort"
pciBridge7.functions = "8"
pciBridge7.present = "TRUE"
pciBridge7.virtualDev = "pcieRootPort"
nvme0.present = "TRUE"
nvme0:0.fileName = "${VMDKFile}"
nvme0:0.present = "TRUE"
floppy0.present = "FALSE"
ide1:0.present = "FALSE"
ethernet0.present = "FALSE"
tools.syncTime = "${HostTimeSync}"
time.synchronize.continue = "${HostTimeSync}"
time.synchronize.restore = "${HostTimeSync}"
time.synchronize.resume.disk = "${HostTimeSync}"
time.synchronize.shrink = "${HostTimeSync}"
time.synchronize.tools.startup = "${HostTimeSync}"
guestTimeZone = "${GuestTimeZone}"
`

var DISPLAY_NAME = regexp.MustCompile(`^displayName = "([^"]+)"`)
var MAC_PATTERN = regexp.MustCompile(`^([[:xdigit:]]{2}:){5}[[:xdigit:]]{2}$`)

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
	hostOS  string
	ramSize string
	guestOS string
	lines   []string
	params  map[string]string
	debug   bool
	verbose bool
}

func GenerateVMX(os, name string, options *CreateOptions, isoOptions *IsoOptions) (*VMX, error) {

	vmx := VMX{
		name:    name,
		hostOS:  os,
		debug:   ViperGetBool("debug"),
		verbose: ViperGetBool("verbose"),
	}

	err := vmx.Generate(options, isoOptions)
	if err != nil {
		return nil, err
	}
	return &vmx, nil
}

func getGuestOS(key string) (string, error) {
	for osKey, osValue := range VMGuestOSValues {
		if key == osKey {
			return osValue, nil
		}
	}
	return "", fmt.Errorf("unexpected GuestOS: %s", key)
}

func (v *VMX) Generate(options *CreateOptions, isoOptions *IsoOptions) error {

	size, err := SizeParse(options.MemorySize)
	if err != nil {
		return err
	}
	v.ramSize = fmt.Sprintf("%d", size/MB)

	guestOS, err := getGuestOS(options.GuestOS)
	if err != nil {
		return err
	}
	v.guestOS = guestOS

	pmap, err := v.mapOptions(options)
	if err != nil {
		return err
	}
	v.params = pmap

	content := os.Expand(vmxTemplate, v.GetConfig)

	err = v.Write([]byte(content))
	if err != nil {
		return err
	}

	_, err = v.SetEFI(options.EFIBoot)
	if err != nil {
		return err
	}

	if isoOptions != nil {
		_, err = v.SetISO(isoOptions.IsoPresent, isoOptions.IsoBootConnected, isoOptions.IsoFile)
		if err != nil {
			return err
		}
	}
	_, err = v.SetEthernet(options.MacAddress)
	if err != nil {
		return err
	}
	// FIXME: CreateOptions needs flags for client-mode, other-end-is-an-app
	_, err = v.SetSerial(options.SerialPipe, options.SerialClient, options.SerialV2V)
	if err != nil {
		return err
	}
	_, err = v.SetVNC(options.VNCEnabled, options.VNCPort)
	if err != nil {
		return err
	}

	_, err = v.SetClipboard(options.ClipboardEnabled)
	if err != nil {
		return err
	}

	_, err = v.SetFileShare(options.FileShareEnabled, options.SharedHostPath, options.SharedGuestPath)
	if err != nil {
		return err
	}

	return nil
}

func (v *VMX) mapOptions(options *CreateOptions) (map[string]string, error) {
	pmap := make(map[string]string)
	data, err := json.Marshal(&options)
	if err != nil {
		return pmap, err
	}
	var omap map[string]any
	err = json.Unmarshal(data, &omap)
	if err != nil {
		return pmap, err
	}

	for key, value := range omap {
		pmap[key] = fmt.Sprintf("%v", value)
	}
	return pmap, nil
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

	var value string
	switch key {
	case "Name":
		value = v.name
	case "GuestOS":
		value = v.guestOS
	case "MemorySize":
		value = v.ramSize
	case "VMDKFile":
		value = v.name + ".vmdk"
	}
	if v.params[key] == "true" {
		value = "TRUE"
	}
	if v.params[key] == "false" {
		value = "FALSE"
	}
	if value == "" {
		value = v.params[key]
	}
	if v.debug {
		log.Printf("set VMX %s = %s\n", key, value)
	}
	return value
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

func (v *VMX) SetEFI(efi bool) (string, error) {
	if v.debug {
		log.Printf("SetEFI(%v)\n", efi)
	}
	v.removePrefix("firmware =")
	if efi {
		v.lines = append(v.lines, `firmware = "efi"`)
		return "set boot firmware: 'EFI'", nil
	}
	return "set boot firmware: 'BIOS'", nil
}

func (v *VMX) SetISO(present, bootConnected bool, path string) (string, error) {

	if v.debug {
		log.Printf("SetIso(present=%v, bootConnected=%v, path=%s)\n", present, bootConnected, path)
	}

	v.removePrefix("ide1:0.")
	if !present {
		v.addLine(`ide1:0.present = "FALSE"`)
		return "removed CD/DVD ISO", nil
	}
	v.addLine(`ide1:0.present = "TRUE"`)
	v.addLine(`ide1:0.deviceType = "cdrom-image"`)
	normalized, err := PathNormalize(path)
	if err != nil {
		return "", err
	}
	hostPath, err := PathnameFormat(v.hostOS, path)
	if err != nil {
		return "", err
	}
	v.addLine(`ide1:0.fileName = "` + hostPath + `"`)
	var atBoot string
	if bootConnected {
		v.addLine(`ide1:0.startConnected = "TRUE"`)
		atBoot = "connected"
	} else {
		v.addLine(`ide1:0.startConnected = "FALSE"`)
		atBoot = "disconnected"
	}
	return fmt.Sprintf("set CD/DVD ISO (%s at boot): '%s'", atBoot, normalized), nil
}

// FIXME: more NIC options could be modified
func (v *VMX) SetEthernet(mac string) (string, error) {
	if v.debug {
		log.Printf("SetEthernet(%s)\n", mac)
	}
	v.removePrefix("ethernet0.")
	if mac == "" {
		v.addLine(`ethernet0.present = "FALSE"`)
		return "removed ethernet device", nil
	}

	v.addLine(`ethernet0.present = "TRUE"`)
	v.addLine(`ethernet0.virtualDev = "e1000"`)
	if mac == "auto" {
		v.addLine(`ethernet0.addressType = "generated"`)
		return "set auto-generated MAC address", nil
	}

	if MAC_PATTERN.MatchString(mac) {
		v.addLine(`ethernet0.address = "` + mac + `"`)
		v.addLine(`ethernet0.addressType = "static"`)
		return fmt.Sprintf("set MAC address: %s", mac), nil
	}

	return "", fmt.Errorf("invalid MAC address: '%s'", mac)

}

func (v *VMX) SetSerial(pipe string, isClient, isV2V bool) (string, error) {
	if v.debug {
		log.Printf("SetSerial(%s, %v, %v)\n", pipe, isClient, isV2V)
	}
	v.removePrefix("serial0.")
	if pipe == "" {
		v.addLine(`serial0.present = "FALSE"`)
		return "removed serial device", nil
	}

	v.addLine(`serial0.present = "TRUE"`)
	v.addLine(`serial0.fileType = "pipe"`)

	normalized, err := PathNormalize(pipe)
	if err != nil {
		return "", err
	}
	hostPipe := normalized

	if v.debug {
		log.Printf("SetSerial: hostOS: %s\n", v.hostOS)
	}

	if v.hostOS == "windows" {
		normalized = strings.TrimLeft(normalized, "//.")
		if v.debug {
			log.Printf("normalized: %s\n", normalized)
		}
		if strings.HasPrefix(normalized, "pipe/") {
			if len(normalized) <= 5 {
				return "", fmt.Errorf("invalid named pipe format: '%s'", pipe)
			}
			normalized = normalized[5:]
		}
		normalized = "//./pipe/" + normalized
		hostPipe = strings.ReplaceAll(normalized, "/", "\\")
	}

	var ttyMode string
	if isV2V {
		ttyMode = "v2v"
		v.addLine(`serial0.tryNoRxLoss = "FALSE"`)
	} else {
		ttyMode = "app"
		v.addLine(`serial0.tryNoRxLoss = "TRUE"`)
	}

	v.addLine(`serial0.fileName = "` + hostPipe + `"`)

	ttyEnd := "server"
	if isClient {
		v.addLine(`serial0.pipe.endPoint = "client"`)
		ttyEnd = "client"
	}

	return fmt.Sprintf("set tty %s %s pipe %s", ttyMode, ttyEnd, hostPipe), nil
}

// fixme: VNC password
func (v *VMX) SetVNC(enabled bool, port int) (string, error) {
	if v.debug {
		log.Printf("SetVNC(%v, %d)\n", enabled, port)
	}
	v.removePrefix("RemoteDisplay.vnc.")
	if !enabled {
		v.addLine(`RemoteDisplay.vnc.enabled = "FALSE"`)
		return "disabled VNC", nil
	}
	v.addLine(`RemoteDisplay.vnc.enabled = "TRUE"`)
	if port != 5900 {
		v.addLine(fmt.Sprintf(`RemoteDisplay.vnc.port = "%d"`, port))
	}
	return fmt.Sprintf("enabled VNC on port %d", port), nil
}

func (v *VMX) SetClipboard(enable bool) (string, error) {
	if v.debug {
		log.Printf("SetClipboard(%v)\n", enable)
	}
	v.removePrefix("isolation.tools.copy")
	v.removePrefix("isolation.tools.pase")
	v.removePrefix("isolation.tools.dnd")

	value := "TRUE"
	action := "disabled clipboard"
	if enable {
		value = "FALSE"
		action = "enabled clipboard"
	}
	v.addLine(fmt.Sprintf(`isolation.tools.copy.disable = "%s"`, value))
	v.addLine(fmt.Sprintf(`isolation.tools.paste.disable = "%s"`, value))
	v.addLine(fmt.Sprintf(`isolation.tools.dnd.disable = "%s"`, value))
	return action, nil
}

/*
> isolation.tools.hgfs.disable = "FALSE"
> sharedFolder0.present = "TRUE"
> sharedFolder0.enabled = "TRUE"
> sharedFolder0.readAccess = "TRUE"
> sharedFolder0.writeAccess = "TRUE"
> sharedFolder0.hostPath = "H:\vmware\howdy_share"
> sharedFolder0.guestName = "howdy_share"
> sharedFolder0.expiration = "never"
> sharedFolder.maxNum = "1"
*/
func (v *VMX) SetFileShare(enable bool, hostPath, guestPath string) (string, error) {
	if v.debug {
		log.Printf("SetFileShare(%v, %s, %s)\n", enable, hostPath, guestPath)
	}

	v.removePrefix("sharedFolder")
	v.removePrefix("isolation.tools.hgfs.")
	if !enable {
		v.addLine(`isolation.tools.hgfs.disable = "TRUE"`)
		return "disabled filesystem share", nil
	}

	formatted, err := PathnameFormat(v.hostOS, hostPath)
	if err != nil {
		return "", err
	}
	hostPath = formatted

	if hostPath == "" {
		return "", fmt.Errorf("missing filesystem share host path")
	}

	if guestPath == "" {
		return "", fmt.Errorf("missing filesystem share guest path")
	}

	v.addLine(`isolation.tools.hgfs.disable = "FALSE"`)
	v.addLine(`sharedFolder0.present = "TRUE"`)
	v.addLine(`sharedFolder0.enabled = "TRUE"`)
	v.addLine(`sharedFolder0.readAccess = "TRUE"`)
	v.addLine(`sharedFolder0.writeAccess = "TRUE"`)
	v.addLine(fmt.Sprintf(`sharedFolder0.guestName = "%s"`, guestPath))
	v.addLine(fmt.Sprintf(`sharedFolder0.hostPath = "%s"`, hostPath))
	v.addLine(`sharedFolder0.expiration = "never"`)
	v.addLine(`sharedFolder0.maxNum = "1"`)
	return fmt.Sprintf("enabled filesystem share: host=%s guest=%s", hostPath, guestPath), nil
}
