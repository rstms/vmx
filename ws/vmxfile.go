package ws

import (
	"fmt"
	"log"
	"regexp"
	"strings"
)

var DISPLAY_NAME = regexp.MustCompile(`^displayName = "([^"]+)"`)
var MAC_PATTERN = regexp.MustCompile(`^([[:xdigit:]]{2}:){5}[[:xdigit:]]{2}$`)
var ISO_FILENAME_PATTERN = regexp.MustCompile(`^ide1:0\.fileName = "([^"]*)"`)
var ISO_PRESENT_PATTERN = regexp.MustCompile(`^ide1:0\.present = "([^"]*)"`)
var USB_ID_PATTERN = regexp.MustCompile(`^[0-9a-fA-F]{4}:[0-9a-fA-F]{4}$`)

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

func InitVMX(os, name string, data []byte) (*VMX, error) {
	vmx := newVMX(os, name)
	err := vmx.Write(data)
	if err != nil {
		return nil, Fatal(err)
	}
	return &vmx, nil
}

func GuestOsParams(key string) (string, string, error) {
	flag := "-g"
	guest := strings.TrimSpace(key)
	_, ok := VMGuestOSValues[guest]
	if !ok {
		flag = "-c"
		log.Printf("custom guest os: '%s'\n", guest)
	}
	if len(guest) == 0 {
		return "", "", Fatalf("null guest os value: %s", key)
	}
	return flag, guest, nil
}

func (v *VMX) Configure(options *CreateOptions, isoOptions *IsoOptions) ([]string, error) {

	actions := []string{}

	if options == nil {
		return actions, Fatalf("missing CreateOptions")
	}
	if isoOptions == nil {
		return actions, Fatalf("missing IsoOptions")
	}

	if options.ModifyName {
		action, err := v.SetName(options.Name)
		if err != nil {
			return actions, Fatal(err)
		}
		actions = append(actions, action)
	}

	if options.ModifyCpu {
		action, err := v.SetCpu(options.CpuCount)
		if err != nil {
			return actions, Fatal(err)
		}
		actions = append(actions, action)
	}

	if options.ModifyMemory {
		action, err := v.SetMemory(options.MemorySize)

		if err != nil {
			return actions, Fatal(err)
		}
		actions = append(actions, action)
	}

	if options.ModifyDisk {
		action, err := v.SetDisk(options.DiskName)
		if err != nil {
			return actions, Fatal(err)
		}
		actions = append(actions, action)
	}

	if options.ModifyFloppy {
		action, err := v.SetFloppy(false)
		if err != nil {
			return actions, Fatal(err)
		}
		actions = append(actions, action)
	}

	if options.ModifyEFI {
		action, err := v.SetEFI(options.EFIBoot)
		if err != nil {
			return actions, Fatal(err)
		}
		actions = append(actions, action)
	}

	if isoOptions.ModifyISO {
		action, err := v.SetISO(isoOptions)
		if err != nil {
			return actions, Fatal(err)
		}
		actions = append(actions, action)
	}

	if options.ModifyUSB {
		action, err := v.SetUSB(options)
		if err != nil {
			return actions, Fatal(err)
		}
		actions = append(actions, action)
	}

	if options.ModifyNIC {
		action, err := v.SetEthernet(options.MacAddress)
		if err != nil {
			return actions, Fatal(err)
		}
		actions = append(actions, action)
	}

	if options.ModifyTTY {
		action, err := v.SetSerial(options.SerialPipe, options.SerialClient, options.SerialV2V)
		if err != nil {
			return actions, Fatal(err)
		}
		actions = append(actions, action)
	}

	if options.ModifyVNC {
		action, err := v.SetVNC(options.VNCEnabled, options.VNCPort)
		if err != nil {
			return actions, Fatal(err)
		}
		actions = append(actions, action)
	}

	if options.ModifyClipboard {
		action, err := v.SetClipboard(options.ClipboardEnabled)
		if err != nil {
			return actions, Fatal(err)
		}
		actions = append(actions, action)
	}

	if options.ModifyShare {
		action, err := v.SetFileShare(options.FileShareEnabled, options.SharedHostPath, options.SharedGuestPath)
		if err != nil {
			return actions, Fatal(err)
		}
		actions = append(actions, action)
	}

	if options.ModifyTimeSync {
		action, err := v.SetTimeSync(options.HostTimeSync)
		if err != nil {
			return actions, Fatal(err)
		}
		actions = append(actions, action)
	}

	if options.ModifyTimeZone {
		action, err := v.SetGuestTimeZone(options.GuestTimeZone)
		if err != nil {
			return actions, Fatal(err)
		}
		actions = append(actions, action)
	}

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

func (v *VMX) SetName(name string) (string, error) {
	if v.debug {
		log.Printf("SetName(%s)\n", name)
	}
	v.removePrefix("displayName ")
	v.addLine(fmt.Sprintf(`displayName = "%s"`, name))
	return "Set display name " + name, nil
}

func (v *VMX) SetCpu(cpuCount int) (string, error) {
	if v.debug {
		log.Printf("SetCpuCount(%d)\n", cpuCount)
	}
	v.removePrefix("numvcpus ")
	v.addLine(fmt.Sprintf(`numvcpus = "%d"`, cpuCount))

	return fmt.Sprintf("Set cpu count %d", cpuCount), nil
}

func (v *VMX) SetMemory(memorySize string) (string, error) {
	if v.debug {
		log.Printf("SetMemory(%s)\n", memorySize)
	}
	v.removePrefix("memsize =")
	v.removePrefix("memory.maxsize =")
	size, err := SizeParse(memorySize)
	if err != nil {
		return "", Fatal(err)
	}
	v.addLine(fmt.Sprintf(`memsize = "%d"`, size/MB))

	return fmt.Sprintf("Set memory size %s", FormatSize(size)), nil
}

func (v *VMX) SetDisk(diskName string) (string, error) {
	if v.debug {
		log.Printf("SetDisk(%s)\n", diskName)
	}
	action := "Removed NVME disk"
	v.removePrefix("nvme0")
	if diskName != "" {
		v.addLine(`nvme0.present = "TRUE"`)
		v.addLine(fmt.Sprintf(`nvme0:0.fileName = "%s"`, diskName))
		v.addLine(`nvme0:0.present = "TRUE"`)
		action = "Set NVME disk " + diskName
	}
	return action, nil
}

func (v *VMX) SetFloppy(enabled bool) (string, error) {
	if v.debug {
		log.Printf("SetFloppy(%v)\n", enabled)
	}
	v.removePrefix("floppy0")
	if enabled {
		return "", Fatalf("unsupported: floppy enable: '%v'", enabled)
	}
	v.lines = append(v.lines, `floppy0.present = "FALSE"`)
	return "Disabled floppy device", nil
}

func (v *VMX) SetGuestTimeZone(zone string) (string, error) {
	if v.debug {
		log.Printf("SetGuestTimeZone('%s')\n", zone)
	}
	v.removePrefix("guestTimeZone")
	if zone == "" {
		return "Removed guest time zone", nil
	}
	v.lines = append(v.lines, fmt.Sprintf(`guestTimeZone = "%s"`, zone))
	return fmt.Sprintf("Set guest time zone: '%s'", zone), nil
}

func (v *VMX) SetEFI(efi bool) (string, error) {
	if v.debug {
		log.Printf("SetEFI(%v)\n", efi)
	}
	v.removePrefix("firmware =")
	if efi {
		v.addLine(`firmware = "efi"`)
		return "Set EFI firmware", nil
	}
	return "Set BIOS firmware", nil
}

func (v *VMX) SetISO(options *IsoOptions) (string, error) {

	if v.debug {
		log.Printf("SetISO(%+v)\n", *options)
	}

	if options.ModifyBootConnected {
		for _, line := range v.lines {
			m := ISO_FILENAME_PATTERN.FindStringSubmatch(line)
			if len(m) == 2 {
				options.IsoFile = m[1]
			}
			m = ISO_PRESENT_PATTERN.FindStringSubmatch(line)
			if len(m) == 2 {
				options.IsoPresent = m[1] == "TRUE"
			}
		}
		//log.Printf("ModifyBootConected: %+v\n", *options)
	}

	v.removePrefix("ide1:0.")
	if !options.IsoPresent {
		v.addLine(`ide1:0.present = "FALSE"`)
		return "Removed boot ISO", nil
	}
	v.addLine(`ide1:0.present = "TRUE"`)
	v.addLine(`ide1:0.deviceType = "cdrom-image"`)

	normalized, err := PathNormalize(options.IsoFile)
	if err != nil {
		return "", Fatal(err)
	}
	hostPath, err := PathFormat(v.hostOS, normalized)
	if err != nil {
		return "", Fatal(err)
	}
	v.addLine(`ide1:0.fileName = "` + hostPath + `"`)
	var atBoot string
	if options.IsoBootConnected {
		v.addLine(`ide1:0.startConnected = "TRUE"`)
		atBoot = "connected"
	} else {
		v.addLine(`ide1:0.startConnected = "FALSE"`)
		atBoot = "disconnected"
	}
	return fmt.Sprintf("Set boot ISO '%s' [%s]", normalized, atBoot), nil
}

// FIXME: more NIC options could be modified
func (v *VMX) SetEthernet(mac string) (string, error) {
	if v.debug {
		log.Printf("SetEthernet(%s)\n", mac)
	}
	v.removePrefix("ethernet0.")
	if mac == "" {
		v.addLine(`ethernet0.present = "FALSE"`)
		return "Removed ethernet device", nil
	}

	v.addLine(`ethernet0.present = "TRUE"`)
	v.addLine(`ethernet0.virtualDev = "e1000"`)
	if mac == "auto" {
		v.addLine(`ethernet0.addressType = "generated"`)
		return "Set auto-generated MAC address", nil
	}

	if MAC_PATTERN.MatchString(mac) {
		v.addLine(`ethernet0.address = "` + mac + `"`)
		v.addLine(`ethernet0.addressType = "static"`)
		return fmt.Sprintf("Set MAC address: %s", mac), nil
	}

	return "", Fatalf("invalid MAC address: '%s'", mac)

}

func (v *VMX) SetSerial(pipe string, isClient, isV2V bool) (string, error) {
	if v.debug {
		log.Printf("SetSerial(%s, %v, %v)\n", pipe, isClient, isV2V)
	}
	v.removePrefix("serial0.")
	if pipe == "" {
		v.addLine(`serial0.present = "FALSE"`)
		return "Removed serial device", nil
	}

	v.addLine(`serial0.present = "TRUE"`)
	v.addLine(`serial0.fileType = "pipe"`)

	normalized, err := PathNormalize(pipe)
	if err != nil {
		return "", Fatal(err)
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
				return "", Fatalf("invalid named pipe format: '%s'", pipe)
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

	return fmt.Sprintf("Set tty %s %s pipe %s", ttyMode, ttyEnd, hostPipe), nil
}

// fixme: VNC password
func (v *VMX) SetVNC(enabled bool, port int) (string, error) {
	if v.debug {
		log.Printf("SetVNC(%v, %d)\n", enabled, port)
	}
	v.removePrefix("RemoteDisplay.vnc.")
	if !enabled {
		v.addLine(`RemoteDisplay.vnc.enabled = "FALSE"`)
		return "Disabled VNC", nil
	}
	v.addLine(`RemoteDisplay.vnc.enabled = "TRUE"`)
	if port != 5900 {
		v.addLine(fmt.Sprintf(`RemoteDisplay.vnc.port = "%d"`, port))
	}
	return fmt.Sprintf("Enabled VNC on port %d", port), nil
}

func (v *VMX) SetClipboard(enable bool) (string, error) {
	if v.debug {
		log.Printf("SetClipboard(%v)\n", enable)
	}
	v.removePrefix("isolation.tools.copy")
	v.removePrefix("isolation.tools.pase")
	v.removePrefix("isolation.tools.dnd")

	value := "TRUE"
	action := "Disabled clipboard"
	if enable {
		value = "FALSE"
		action = "Enabled clipboard"
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
		return "Disabled filesystem share", nil
	}

	formatted, err := PathnameFormat(v.hostOS, hostPath)
	if err != nil {
		return "", Fatal(err)
	}
	hostPath = formatted

	if hostPath == "" {
		return "", Fatalf("missing filesystem share host path")
	}

	if guestPath == "" {
		return "", Fatalf("missing filesystem share guest path")
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
	return fmt.Sprintf("Enabled filesystem share: host=%s guest=%s", hostPath, guestPath), nil
}

func (v *VMX) SetTimeSync(enable bool) (string, error) {
	if v.debug {
		log.Printf("SetTimeSync(%v)\n", enable)
	}

	v.removePrefix("tools.syncTime")
	v.removePrefix("time.synchronize")
	if !enable {
		v.addLine(`tools.syncTime = "FALSE"`)
		return "Disabled host time sync", nil
	}
	v.addLine(`tools.syncTime = "TRUE"`)
	v.addLine(`time.synchronize.continue = "TRUE"`)
	v.addLine(`time.synchronize.restore = "TRUE"`)
	v.addLine(`time.synchronize.resume.disk = "TRUE"`)
	v.addLine(`time.synchronize.shrink = "TRUE"`)
	v.addLine(`time.synchronize.tools.startup = "TRUE"`)
	return "Enabled host time sync", nil
}

func (v *VMX) SetUSB(options *CreateOptions) (string, error) {
	if v.debug {
		log.Printf("SetUSB: %s\n", FormatJSON(*options))
	}

	v.removePrefix("usb.generic.allow")
	v.removePrefix("usb.autoConnect")
	v.removePrefix("usb.quirks")

	if options.AllowHID {
		v.addLine(`usb.generic.allowHID = "TRUE"`)
	}
	if options.AllowCCID {
		v.addLine(`usb.generic.allowCCID = "TRUE"`)
	}

	devices := map[string]string{
		"device0": options.Device0,
		"device1": options.Device1,
	}
	for label, ids := range devices {

		if ids != "" {
			if !USB_ID_PATTERN.MatchString(ids) {
				return "", Fatalf("unexpected format: USB %s: %s", label, ids)
			}
			vid, pid, ok := strings.Cut(ids, ":")
			if !ok {
				return "", Fatalf("failed parsing USB %s: %s", label, ids)
			}
			v.addLine(fmt.Sprintf(`usb.quirks.%s = "0x%s:0x%s allow"`, label, vid, pid))
			v.addLine(fmt.Sprintf(`usb.autoConnect.%s = "vid:%s pid:%s autoclean:0"`, label, vid, pid))

		}
	}
	return "Configured USB devices", nil
}
