package ws

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
)

const vmxTemplate string = `.encoding = "UTF-8"
config.version = "8"
virtualHW.version = "19"
displayName = "${displayName}"
numvcpus = "${numvcpus}"
memsize = "${memsize}"
guestOS = "${guestOS}"
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
`

var DISPLAY_NAME = regexp.MustCompile(`^displayName = "([^"]+)"`)
var MAC_PATTERN = regexp.MustCompile(`^([[:xdigit:]]{2}:){5}[[:xdigit:]]{2}$`)

// OS names generated using the error message from this command:
// 'vmcli VM create -n notavalidname -d /notavaliddir -g notavalidosname

var VMGuestOSValues map[string]bool = map[string]bool{
	"debian12-64":      true,
	"centos8-64":       true,
	"other6xlinux-64":  true,
	"debian13-64":      true,
	"centos9-64":       true,
	"windows11-64":     true,
	"windows9-64":      true,
	"fedora-64":        true,
	"rhel10-64":        true,
	"rhel9-64":         true,
	"opensuse-64":      true,
	"ubuntu-64":        true,
	"vmware-photon-64": true,
}

type VMX struct {
	name    string
	hostOS  string
	macros  map[string]string
	lines   []string
	debug   bool
	verbose bool
}

func newVMX(os, name string) VMX {
	vmx := VMX{
		name:    name,
		hostOS:  os,
		debug:   ViperGetBool("debug"),
		verbose: ViperGetBool("verbose"),
	}
	return vmx
}

func GenerateVMX(os, name string, options *CreateOptions, isoOptions *IsoOptions) (*VMX, error) {

	vmx := newVMX(os, name)
	actions, err := vmx.Generate(options, isoOptions)
	if err != nil {
		return nil, err
	}
	if vmx.verbose {
		for _, action := range actions {
			log.Printf("[%s] %s\n", vmx.name, action)
		}
	}
	return &vmx, nil
}

func InitVMX(os, name string, data []byte) (*VMX, error) {
	vmx := newVMX(os, name)
	err := vmx.Write(data)
	if err != nil {
		return nil, err
	}
	return &vmx, nil
}

func guestOsParams(key string) (string, string, error) {
	flag := "-g"
	guest := strings.TrimSpace(key)
	_, ok := VMGuestOSValues[guest]
	if !ok {
		flag = "-c"
		log.Printf("custom guest os: '%s'\n", guest)
	}
	if len(guest) == 0 {
		return "", "", fmt.Errorf("null guest os value: %s", key)
	}
	return flag, guest, nil
}

func (v *VMX) InitializeFromTemplate(options *CreateOptions, isoOptions *IsoOptions) error {

	v.macros = make(map[string]string)
	v.macros["displayName"] = v.name
	v.macros["numvcpus"] = fmt.Sprintf("%d", options.CpuCount)
	size, err := SizeParse(options.MemorySize)
	if err != nil {
		return err
	}
	v.macros["memsize"] = fmt.Sprintf("%d", size/MB)
	_, osName, err := guestOsParams(options.GuestOS)
	if err != nil {
		return err
	}
	v.macros["guestOS"] = osName
	v.macros["VMDKfile"] = v.name + ".vmdk"

	content := os.Expand(vmxTemplate, v.GetConfig)

	err = v.Write([]byte(content))
	if err != nil {
		return err
	}
	return nil
}

func (v *VMX) Generate(options *CreateOptions, isoOptions *IsoOptions) ([]string, error) {

	actions := []string{}

	// TODO: use vmcli to generate initial VMX file instead of template
	err := v.InitializeFromTemplate(options, isoOptions)
	if err != nil {
		return actions, err
	}

	action, err := v.SetEFI(options.EFIBoot)
	if err != nil {
		return actions, err
	}
	actions = append(actions, action)

	action, err = v.SetFloppy(false)
	if err != nil {
		return actions, err
	}
	actions = append(actions, action)

	if isoOptions != nil {
		action, err = v.SetISO(isoOptions.IsoPresent, isoOptions.IsoBootConnected, isoOptions.IsoFile)
		if err != nil {
			return actions, err
		}
	}
	action, err = v.SetEthernet(options.MacAddress)
	if err != nil {
		return actions, err
	}
	actions = append(actions, action)

	action, err = v.SetSerial(options.SerialPipe, options.SerialClient, options.SerialV2V)
	if err != nil {
		return actions, err
	}
	actions = append(actions, action)
	action, err = v.SetVNC(options.VNCEnabled, options.VNCPort)
	if err != nil {
		return actions, err
	}
	actions = append(actions, action)

	action, err = v.SetClipboard(options.ClipboardEnabled)
	if err != nil {
		return actions, err
	}
	actions = append(actions, action)

	action, err = v.SetFileShare(options.FileShareEnabled, options.SharedHostPath, options.SharedGuestPath)
	if err != nil {
		return actions, err
	}
	actions = append(actions, action)

	action, err = v.SetTimeSync(options.HostTimeSync)
	if err != nil {
		return actions, err
	}
	actions = append(actions, action)

	action, err = v.SetGuestTimeZone(options.GuestTimeZone)
	if err != nil {
		return actions, err
	}
	actions = append(actions, action)

	return actions, nil
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
	case "displayName", "guestOS", "numvcpus", "memsize":
		value = v.macros[key]
	case "VMDKFile":
		value = v.name + ".vmdk"
	}
	if value == "" {
		log.Printf("WARNING: no expansion value found for '%s'\n", key)
		value = "MISSING_VMX_EXPANSION_VALUE"
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

func (v *VMX) SetFloppy(enabled bool) (string, error) {
	if v.debug {
		log.Printf("SetFloppy(%v)\n", enabled)
	}
	v.removePrefix("floppy0")
	if enabled {
		return "", fmt.Errorf("unsupported: floppy enable: '%v'", enabled)
	}
	v.lines = append(v.lines, `floppy0.present = "FALSE"`)
	return "disabled floppy device", nil
}

func (v *VMX) SetGuestTimeZone(zone string) (string, error) {
	if v.debug {
		log.Printf("SetGuestTimeZone('%s')\n", zone)
	}
	v.removePrefix("guestTimeZone")
	if zone == "" {
		return "removed guest time zone", nil
	}
	v.lines = append(v.lines, fmt.Sprintf(`guestTimeZone = "%s"`, zone))
	return fmt.Sprintf("set guest time zone: '%s'", zone), nil
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

func (v *VMX) SetTimeSync(enable bool) (string, error) {
	if v.debug {
		log.Printf("SetTimeSync(%v)\n", enable)
	}

	v.removePrefix("tools.syncTime")
	v.removePrefix("time.synchronize")
	if !enable {
		v.addLine(`tools.syncTime = "FALSE"`)
		return "disabled host time sync", nil
	}
	v.addLine(`tools.syncTime = "TRUE"`)
	v.addLine(`time.synchronize.continue = "TRUE"`)
	v.addLine(`time.synchronize.restore = "TRUE"`)
	v.addLine(`time.synchronize.resume.disk = "TRUE"`)
	v.addLine(`time.synchronize.shrink = "TRUE"`)
	v.addLine(`time.synchronize.tools.startup = "TRUE"`)
	return "enabled host time sync", nil
}
