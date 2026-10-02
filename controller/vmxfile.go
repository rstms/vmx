package controller

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
var VID_PATTERN = regexp.MustCompile(`^vid[:-]:(?:0x)*([[:xdigit:]]{4})$`)
var PID_PATTERN = regexp.MustCompile(`^pid[:-](?:0x)*([[:xdigit:]]{4})$`)
var AUTOCLEAN_PATTERN = regexp.MustCompile(`^autoclean[:-]([01])$`)
var VIDPID_PATTERN = regexp.MustCompile(`^(?:0x)*([[:xdigit:]]{4})[:-](?:0x)*([[:xdigit:]]{4})$`)
var VIDONLY_PATTERN = regexp.MustCompile(`^(?:0x)*([[:xdigit:]]{4})$`)
var HEX4_PATTERN = regexp.MustCompile(`^[[:xdigit:]]{4}$`)

const USB_DEVICE_COUNT = 4

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

type USBDevice struct {
	VID       string
	PID       string
	Autoclean bool
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
		action, err := v.SetMemory(options.MemorySize, options.VramSize)

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
	log.Printf("addLine: %s\n", line)
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

func (v *VMX) SetMemory(memorySize, vramSize string) (string, error) {
	if v.debug {
		log.Printf("SetMemory(%s, %s)\n", memorySize, vramSize)
	}
	v.removePrefix("svga.graphicsMemoryKB =")
	v.removePrefix("memsize =")
	v.removePrefix("memory.maxsize =")
	size, err := SizeParse(memorySize)
	if err != nil {
		return "", Fatal(err)
	}
	v.addLine(fmt.Sprintf(`memsize = "%d"`, size/MB))

	vsize, err := SizeParse(vramSize)
	if err != nil {
		return "", Fatal(err)
	}
	v.addLine(fmt.Sprintf(`svga.graphicsMemoryKB = "%d"`, vsize/KB))

	return fmt.Sprintf("Set memory size %s / vram size %s", FormatSize(size), FormatSize(vsize)), nil
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
		return "boot=EFI", nil
	}
	return "boot=BIOS", nil
}

func (v *VMX) SetISO(options *IsoOptions) (string, error) {

	if v.debug {
		log.Printf("SetISO(%+v)\n", *options)
	}

	if options.IsoBootConnected {
		options.IsoPresent = true
	}

	v.removePrefix("sata0")
	if !options.IsoPresent {
		return "Removed boot ISO", nil
	}
	v.addLine(`sata0.present = "TRUE"`)

	isoList := []string{}
	var atBoot string
	for i, file := range options.IsoFiles {
		v.addLine(fmt.Sprintf(`sata0:%d.present = "TRUE"`, i))
		v.addLine(fmt.Sprintf(`sata0:%d.deviceType = "cdrom-image"`, i))

		normalized, err := PathNormalize(file)
		if err != nil {
			return "", Fatal(err)
		}
		hostPath, err := PathFormat(v.hostOS, normalized)
		if err != nil {
			return "", Fatal(err)
		}
		v.addLine(fmt.Sprintf(`sata0:%d.fileName = "`+hostPath+`"`, i))

		if options.IsoBootConnected {
			v.addLine(fmt.Sprintf(`sata0:%d.startConnected = "TRUE"`, i))
			atBoot = "connected"
		} else {
			v.addLine(fmt.Sprintf(`sata0:%d.startConnected = "FALSE"`, i))
			atBoot = "disconnected"
		}
		isoList = append(isoList, normalized)
	}
	return fmt.Sprintf("Set boot ISO '%s' [%s]", strings.Join(isoList, ";"), atBoot), nil
}

// FIXME: more NIC options could be modified
func (v *VMX) SetEthernet(mac string) (string, error) {
	if v.debug {
		log.Printf("SetEthernet(%s)\n", mac)
	}
	v.removePrefix("ethernet")
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
	v.removePrefix("serial")
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
		return "share=disabled", nil
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
	return fmt.Sprintf("share=enabled host=%s guest=%s", hostPath, guestPath), nil
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

// https://knowledge.broadcom.com/external/article?legacyId=1648
// https://support.yubico.com/s/article/Troubleshooting-device-passthrough-with-VMware-Workstation-and-VMware-Fusion

func (v *VMX) SetUSB(options *CreateOptions) (string, error) {
	if v.debug {
		log.Printf("SetUSB: %s\n", FormatJSON(*options))
	}

	action := "usb="

	v.removePrefix("usb")

	prefix := "usb"

	switch options.USBVersion {
	case 0:
		return action + "disabled", nil
	case 2:
		v.addLine(`usb.present = "TRUE"`)
		action += "2.0"
	case 3:
		action += "3.2"
		v.addLine(`usb.present = "TRUE"`)
		v.addLine(`usb_xhci.present = "TRUE"`)
		prefix = "usb_xhci"
	default:
		return "", Fatalf("unexpected USB version: %d", options.USBVersion)
	}

	action += fmt.Sprintf(" restrict=%v", options.RestrictUSB)
	if options.RestrictUSB {
		v.addLine(`usb.restrictions.defaultAllow = "FALSE"`)
	} else {
		v.addLine(`usb.restrictions.defaultAllow = "TRUE"`)
	}

	action += fmt.Sprintf(" hid=%v", options.AllowHID)
	if options.AllowHID {
		v.addLine(`usb.generic.allowHID = "TRUE"`)
		v.addLine(`usb.generic.allowLastHID = "TRUE"`)
	} else {
		v.addLine(`usb.generic.allowHID = "FALSE"`)
		v.addLine(`usb.generic.allowLastHID = "FALSE"`)
	}

	action += fmt.Sprintf(" ccid=%v", options.AllowCCID)
	if options.AllowCCID {
		v.addLine(`usb.generic.allowCCID = "TRUE"`)
		// possibly outdated VMX
		//v.addLine(`usb.ccid.disable = "FALSE"`)
	} else {
		v.addLine(`usb.generic.allowCCID = "FALSE"`)
	}

	for i := 0; i < USB_DEVICE_COUNT; i++ {
		var value string
		switch i {
		case 0:
			value = options.Device0
		case 1:
			value = options.Device1
		case 2:
			value = options.Device2
		case 3:
			value = options.Device3
		}
		if value != "" {
			device, err := ParseUSBDevice(value)
			if err != nil {
				return "", Fatal(err)
			}
			v.addLine(device.FormatAutoconnectLine(prefix, i))
			v.addLine(device.FormatQuirksLine(i))
			action += fmt.Sprintf(" device%d='%v'", i, device)
		}
	}

	return action, nil
}

func ParseUSBDevice(value string) (*USBDevice, error) {
	device := USBDevice{}
	fields := strings.Split(value, " ")
	//log.Printf("fields=%+v\n", fields)
	for _, field := range fields {
		vidMatch := VID_PATTERN.FindStringSubmatch(field)
		pidMatch := PID_PATTERN.FindStringSubmatch(field)
		autocleanMatch := AUTOCLEAN_PATTERN.FindStringSubmatch(field)
		vidPidMatch := VIDPID_PATTERN.FindStringSubmatch(field)
		vidOnlyMatch := VIDONLY_PATTERN.FindStringSubmatch(field)
		if len(vidMatch) > 1 {
			device.VID = vidMatch[len(vidMatch)-1]
		} else if len(pidMatch) > 1 {
			device.PID = pidMatch[len(pidMatch)-1]
		} else if len(vidPidMatch) > 2 {
			device.VID = vidPidMatch[1]
			device.PID = vidPidMatch[2]
		} else if len(vidOnlyMatch) > 1 {
			device.VID = vidOnlyMatch[1]
		} else if len(autocleanMatch) > 1 {
			switch strings.ToLower(autocleanMatch[len(autocleanMatch)-1]) {
			case "0", "false":
				device.Autoclean = false
			case "", "1", "true":
				device.Autoclean = true
			default:
				return nil, Fatalf("unexpected autoclean value '%s' in USB config '%s'", field, value)
			}
		} else {
			return nil, Fatalf("unexpected field '%s' in USB config '%s'", field, value)
		}
	}

	if !HEX4_PATTERN.MatchString(device.VID) {
		return nil, Fatalf("invalid VID in USB config '%s'", value)
	}
	if device.PID != "" {
		if !HEX4_PATTERN.MatchString(device.PID) {
			return nil, Fatalf("invalid PID in USB config '%s'", value)
		}
	}
	return &device, nil
}

func (d *USBDevice) FormatAutoconnectLine(prefix string, index int) string {
	return fmt.Sprintf(`%s.autoconnect.device%d = "%s"`, prefix, index, d.String())
}

func (d *USBDevice) FormatQuirksLine(index int) string {
	address := "0x" + d.VID
	if d.PID != "" {
		address += ":0x" + d.PID
	}
	return fmt.Sprintf(`usb.quirks.device%d = "%s allow"`, index, address)
}

func (d *USBDevice) String() string {
	text := "vid:0x" + d.VID
	if d.PID != "" {
		text += (" pid:0x" + d.PID)
	}
	if d.Autoclean {
		text += " autoclean:1"
	} else {
		text += " autoclean:0"
	}
	return text
}
