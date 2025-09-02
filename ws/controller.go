package ws

import (
	"encoding/json"
	"fmt"
	"github.com/rstms/winexec/client"
	"log"
	"os"
	"os/user"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const Version = "0.1.18"

var WINDOWS_ENV_PATTERN = regexp.MustCompile(`^WINDIR=.*WINDOWS.*`)
var ENCRYPTED_VM_ERROR = regexp.MustCompile(`Something went wrong while getting password from stdin`)
var NONEXISTENT_VM_ERROR = regexp.MustCompile(`VMX : '[^']*' does not exist!`)

const DEFAULT_INTERVAL_SECONDS = 3
const DEFAULT_TIMEOUT_SECONDS = 300

type VID struct {
	Id   string
	Path string
	Name string
}

type VMState struct {
	Name       string
	Path       string
	Id         string
	MacAddress string
	IpAddress  string
	PowerState string
	Result     string
}

type VMFile struct {
	Name   string `json:"name,omitzero"`
	Length uint64 `json:"length,omitzero"`
}

type VM struct {
	Id   string
	Path string
	Name string

	CpuCount int
	RamSize  string
	DiskSize string

	MacAddress string
	IpAddress  string

	IsoAttached      bool
	IsoAttachOnStart bool
	IsoFile          string

	SerialAttached bool
	SerialPipe     string

	VncEnabled bool
	VncPort    int

	ClipboardEnabled bool
	FileShareEnabled bool

	Running    bool
	PowerState string
	Encrypted  bool
}

type QueryType int

const (
	QueryTypeConfig = iota
	QueryTypeState
	QueryTypeAll
)

type ExecMode int

const (
	CheckExitCode = iota
	IgnoreExitCode
)

type Controller interface {
	Create(string, CreateOptions, IsoOptions) (string, error)
	Get(string) (VM, error)
	Modify(string, CreateOptions, IsoOptions) (*[]string, error)
	Start(string, StartOptions, IsoOptions) (string, error)
	Stop(string, StopOptions) (string, error)
	Destroy(string, DestroyOptions) error
	Show(string, ShowOptions) (*[]VMState, error)
	GetProperty(string, string) (string, error)
	SetProperty(string, string, string) error
	Upload(string, string, string) error
	Download(string, string, string) error
	Files(string, FilesOptions) ([]string, error)
	Wait(string, string) error
	SendKeys(string, string) error
	Close() error
	GetState(string) (*VMState, error)
}

type vmctl struct {
	Hostname        string
	Username        string
	KeyFile         string
	Roots           []string
	IsoPath         string
	winexec         *client.WinexecClient
	cli             *vmcli
	Shell           string
	Local           string
	Remote          string
	debug           bool
	verbose         bool
	Version         string
	vmkey           map[string]string
	IntervalSeconds int64
	TimeoutSeconds  int64
}

// return true if VMWare Workstation Host is localhost
func (v *vmctl) isLocal() (bool, error) {
	if v.Hostname == "" || v.Hostname == "localhost" || v.Hostname == "127.0.0.1" {
		return true, nil
	}
	fqdn, err := os.Hostname()
	if err != nil {
		return false, Fatal(err)
	}
	if v.Hostname == fqdn {
		return true, nil
	}
	host, _, ok := strings.Cut(fqdn, ".")
	if ok {
		if v.Hostname == host {
			return true, nil
		}
	}
	return false, nil
}

func (v *vmctl) detectRemoteOS() (string, error) {
	if v.debug {
		log.Println("detectRemoteOS")
	}
	olines, err := v.exec("ssh", append(v.sshArgs(), "env"), "", nil)
	if err != nil {
		return "", Fatal(err)
	}
	for _, line := range olines {
		if WINDOWS_ENV_PATTERN.MatchString(strings.ToUpper(line)) {
			return "windows", nil
		}
	}
	olines, err = v.exec("ssh", append(v.sshArgs(), "uname"), "", nil)
	if err != nil {
		return "", Fatal(err)
	}
	if len(olines) != 1 {
		return "", Fatalf("unexpected uname response: %v", olines)
	}
	return strings.ToLower(olines[0]), nil
}

func NewVMXController() (Controller, error) {

	var prefix string
	if ProgramName() != "vmx" {
		prefix = "vmx."
	}

	user, err := user.Current()
	if err != nil {
		return nil, Fatal(err)
	}

	ViperSetDefault(prefix+"host", "localhost")
	ViperSetDefault(prefix+"vmware_roots", []string{"/var/vmware"})
	ViperSetDefault(prefix+"iso_path", "/var/vmware/iso")
	ViperSetDefault(prefix+"disable_keepalives", true)
	ViperSetDefault(prefix+"interval_seconds", DEFAULT_INTERVAL_SECONDS)
	ViperSetDefault(prefix+"timeout_seconds", DEFAULT_TIMEOUT_SECONDS)
	ViperSetDefault(prefix+"user", user.Username)

	v := vmctl{
		Hostname:        ViperGetString(prefix + "host"),
		Username:        ViperGetString(prefix + "user"),
		KeyFile:         ViperGetString(prefix + "ssh_key"),
		verbose:         ViperGetBool(prefix + "verbose"),
		debug:           ViperGetBool(prefix + "debug"),
		Version:         Version,
		IntervalSeconds: ViperGetInt64(prefix + "interval_seconds"),
		TimeoutSeconds:  ViperGetInt64(prefix + "timeout_seconds"),
	}

	roots := ViperGetStringSlice(prefix + "vmware_roots")
	v.Roots = make([]string, len(roots))
	for i, root := range roots {
		normalized, err := PathNormalize(root)
		if err != nil {
			return nil, Fatal(err)
		}
		v.Roots[i] = normalized
	}
	path, err := PathNormalize(ViperGetString(prefix + "iso_path"))
	if err != nil {
		return nil, Fatal(err)
	}
	v.IsoPath = path

	v.cli = NewCliClient(&v)

	v.Local = runtime.GOOS
	local, err := v.isLocal()
	if err != nil {
		return nil, Fatal(err)
	}
	if local {
		v.Remote = v.Local
		switch v.Local {
		case "windows":
			v.Shell = "cmd"
		default:
			v.Shell = "sh"
		}
	} else {
		v.Shell = ViperGetString(prefix + "shell")
		switch v.Shell {
		case "winexec":
			w, err := client.NewWinexecClient()
			if err != nil {
				return nil, Fatal(err)
			}
			v.winexec = w
			v.Remote = "windows"
		case "ssh":
			v.Shell = "ssh"
			remote, err := v.detectRemoteOS()
			if err != nil {
				return nil, Fatal(err)
			}
			if v.debug {
				log.Printf("detected remote os: %s\n", remote)
			}
			v.Remote = remote
		}
	}
	v.mapVMKeys()
	return &v, nil
}

func (v *vmctl) Close() error {
	if v.debug {
		log.Println("Close")
	}
	return nil
}

func (v *vmctl) requirePowerState(vm *VM, state, action string) error {
	err := v.cli.QueryPowerState(vm)
	if err != nil {
		return Fatal(err)
	}
	if vm.PowerState != state {
		return Fatalf("Power state '%s' is required to %s", state, action)
	}
	return nil
}

func (v *vmctl) checkPowerState(vm *VM, command, state string) (bool, error) {
	if v.debug {
		log.Printf("checkPowerState(%s, %s, %s)\n", vm.Name, command, state)
	}
	err := v.validatePowerState(state)
	if err != nil {
		return false, Fatal(err)
	}
	err = v.cli.QueryPowerState(vm)
	if err != nil {
		return false, Fatal(err)
	}
	if vm.PowerState == state {
		log.Printf("[%s] ignoring %s in power state %s", vm.Name, command, vm.PowerState)
		if v.verbose {
			fmt.Printf("[%s] %s\n", vm.Name, vm.PowerState)
		}
		return true, nil
	}
	return false, nil
}

func (v *vmctl) setStretch(vm *VM, enabled bool) error {
	stretch := "FALSE"
	action := "no_stretch"
	if enabled {
		stretch = "TRUE"
		action = "enabled"
	}
	if v.debug {
		log.Printf("setStretch(%s) %s\n", vm.Name, action)
	}
	if stretch != "" {
		err := v.cli.SetParam(vm, "gui.EnableStretchGuest", stretch)
		if err != nil {
			return Fatal(err)
		}
		if v.verbose {
			fmt.Printf("[%s] display stretch %s\n", vm.Name, action)
		}
	}
	return nil
}

func (v *vmctl) validatePowerState(state string) error {
	if v.debug {
		log.Printf("validatePowerState(%s)\n", state)
	}
	switch state {
	case "on", "off", "paused", "suspended":
		return nil
	default:
		return Fatalf("unknown power state: %s", state)
	}
}

func (v *vmctl) queryPowerState(vid string) (string, error) {
	v.cli.Reset()
	vm, err := v.cli.GetVM(vid)
	if err != nil {
		return "", Fatal(err)
	}
	err = v.cli.QueryPowerState(&vm)
	if err != nil {
		return "", Fatal(err)
	}
	return vm.PowerState, nil
}

func (v *vmctl) Wait(vid, state string) error {
	if v.debug {
		log.Printf("Wait(%s, %s)\n", vid, state)
	}
	switch strings.ToLower(state) {
	case "up", "on", "running":
		state = "on"
	case "down", "off", "stopped":
		state = "off"
	case "suspended":
		state = "suspended"
	}
	err := v.validatePowerState(state)
	if err != nil {
		return Fatal(err)
	}

	if v.verbose {
		fmt.Printf("[%s] Awaiting power state: %s\n", vid, state)
	}
	start := time.Now()
	interval := time.Duration(v.IntervalSeconds) * time.Second
	timeout := time.Duration(v.TimeoutSeconds) * time.Second
	checkPower := true
	running := false
	for {
		if (state == "on") && !running {
			// if waiting for poweredOn, ensure vmrun shows the instance before querying with vmrest API
			checkPower = false
			vms, err := v.Show("", ShowOptions{Running: true})
			if err != nil {
				return Fatal(err)
			}
			for _, vm := range *vms {
				if vm.Name == vid {
					// set checkPower after the next sleep
					running = true
				}
			}
		}

		if checkPower {
			newState, err := v.queryPowerState(vid)
			if err != nil {
				return Fatal(err)
			}

			if newState == state {
				if v.verbose {
					fmt.Printf("[%s] Detected power %s\n", vid, state)
				}
				return nil
			}
		}
		if v.TimeoutSeconds != 0 {
			if time.Since(start) > timeout {
				return Fatalf("[%s] Timed out awaiting power state %s", vid, state)
			}
		}
		time.Sleep(interval)
		if running {
			checkPower = true
		}
	}
}

func (v *vmctl) Get(vid string) (VM, error) {
	if v.debug {
		log.Printf("Get(%s)\n", vid)
	}
	vm, err := v.cli.GetVM(vid)
	if err != nil {
		return VM{}, Fatal(err)
	}
	return vm, nil
}

func (v *vmctl) GetState(vid string) (*VMState, error) {

	vm, err := v.Get(vid)
	if err != nil {
		return nil, Fatal(err)
	}

	err = v.queryVM(&vm, QueryTypeState)
	if err != nil {
		return nil, Fatal(err)
	}
	state := VMState{
		Name:       vm.Name,
		Path:       vm.Path,
		Id:         vm.Id,
		MacAddress: vm.MacAddress,
		IpAddress:  vm.IpAddress,
		PowerState: vm.PowerState,
	}
	return &state, nil
}

func (v *vmctl) GetProperty(vid, property string) (string, error) {
	if v.verbose {
		log.Printf("GetProperty(%s, %s)\n", vid, property)
	}
	vm, err := v.Get(vid)
	if err != nil {
		return "", Fatal(err)
	}

	switch strings.ToLower(property) {
	case "vmx":
		data, err := v.ReadHostFile(&vm, vm.Name+".vmx")
		if err != nil {
			return "", Fatal(err)
		}
		return string(data), nil

	case "power", "powerstate":
		err := v.cli.QueryPowerState(&vm)
		if err != nil {
			return "", Fatal(err)
		}
		return vm.PowerState, nil

	case "ip", "ipaddress", "ipaddr":
		err = v.getIpAddress(&vm)
		if err != nil {
			return "", Fatal(err)
		}
		return vm.IpAddress, nil

	case "disk", "disks", "diskinfo", "disksize", "disksizemb", "diskcapacity":
		disks, ok, err := v.getDisks(&vm)
		if err != nil {
			return "", Fatal(err)
		}
		if !ok {
			return "", Fatalf("[%s] no disks found", vm.Name)
		}
		switch strings.ToLower(property) {
		case "disksize", "disksizemb":
			return fmt.Sprintf("%v", vm.DiskSize), nil
		case "diskcapacity":
			return fmt.Sprintf("%v", disks[0].Capacity), nil
		case "disk":
			return FormatJSON(disks[0]), nil
		case "diskinfo":
			return FormatJSON(disks), nil
		default:
			return "", Fatalf("[%s] unexpected property: %s", vm.Name, property)
		}

	case "mac", "macaddr", "macaddress":
		err := v.cli.GetMacAddress(&vm, nil)
		if err != nil {
			return "", Fatal(err)
		}
		return vm.MacAddress, nil

	case "state":
		state, err := v.GetState(vid)
		if err != nil {
			return "", Fatal(err)
		}
		return FormatJSON(state), nil
	}

	switch strings.ToLower(property) {
	case "config":
		err = v.queryVM(&vm, QueryTypeConfig)
		if err != nil {
			return "", Fatal(err)
		}
		return FormatJSON(&vm), nil
	case "all", "detail", "":
		err := v.queryVM(&vm, QueryTypeAll)
		if err != nil {
			return "", Fatal(err)
		}
		return FormatJSON(&vm), nil
	}

	// try property as a VM key
	value, ok, err := v.queryVMProperty(&vm, property)
	if err != nil {
		return "", Fatal(err)
	}
	if ok {
		return value, nil
	}

	// try property as a VMX key
	value, err = v.cli.GetParam(&vm, property)
	if err != nil {
		return "", Fatal(err)
	}
	return value, nil
}

func (v *vmctl) queryVM(vm *VM, queryType QueryType) error {
	if v.debug {
		log.Printf("queryVM(%s, %d)\n", vm.Name, queryType)
	}
	if queryType == QueryTypeConfig || queryType == QueryTypeAll {
		err := v.cli.GetConfig(vm)
		if err != nil {
			return Fatal(err)
		}
		_, _, err = v.getDisks(vm)
		if err != nil {
			return Fatal(err)
		}
	}
	if queryType == QueryTypeState || queryType == QueryTypeAll {
		err := v.cli.QueryPowerState(vm)
		if err != nil {
			if checkEncryptedError(vm, err) {
				vm.Encrypted = true
				return nil
			}
			return Fatal(err)
		}
		err = v.cli.GetMacAddress(vm, nil)
		if err != nil {
			return Fatal(err)
		}
		err = v.getIpAddress(vm)
		if err != nil {
			return Fatal(err)
		}
	}
	return nil
}

func (v *vmctl) mapVMKeys() error {
	if v.debug {
		log.Println("mapVMkeys()")
	}
	vmap, err := v.toMap(&VM{})
	if err != nil {
		return Fatal(err)
	}
	v.vmkey = make(map[string]string)
	for k, _ := range vmap {
		v.vmkey[strings.ToLower(k)] = k
	}
	return nil
}

// convert VM struct to map[string]any
func (v *vmctl) toMap(vm *VM) (map[string]any, error) {
	var vmap map[string]any
	if v.debug {
		log.Printf("mapVM(%s)\n", vm.Name)
	}
	data, err := json.Marshal(vm)
	if err != nil {
		return vmap, Fatal(err)
	}
	err = json.Unmarshal([]byte(data), &vmap)
	if err != nil {
		return vmap, Fatal(err)
	}
	return vmap, nil
}

func (v *vmctl) queryVMProperty(vm *VM, property string) (string, bool, error) {
	if v.debug {
		log.Printf("queryVMProperty(%s, %s)\n", vm.Name, property)
	}

	key, ok := v.vmkey[strings.ToLower(property)]
	if !ok {
		return "", false, nil
	}

	err := v.queryVM(vm, QueryTypeAll)
	if err != nil {
		return "", false, Fatal(err)
	}

	vmap, err := v.toMap(vm)
	if err != nil {
		return "", false, Fatal(err)
	}

	value, ok := vmap[key]
	if ok {
		data, err := json.Marshal(value)
		if err != nil {
			return "", false, Fatal(err)
		}
		return string(data), true, nil
		//return fmt.Sprintf("%v", value), true, nil
	}
	return "", false, nil
}

func FormatVMXBool(value string) (string, error) {
	switch strings.ToLower(value) {
	case "true", "1", "t", "on", "yes", "y", "enable", "enabled":
		return "TRUE", nil
	case "false", "0", "f", "off", "no", "n", "disable", "disabled":
		return "TRUE", nil
	}
	return "", Fatalf("cannot format '%s' as boolean", value)
}

func (v *vmctl) SetProperty(vid, property, value string) error {
	if v.debug {
		log.Printf("SetProperty(%s, %s, %s)\n", vid, property, value)
	}
	vm, err := v.cli.GetVM(vid)
	if err != nil {
		return Fatal(err)
	}
	if v.verbose {
		fmt.Printf("[%s] setting %s=%s\n", vm.Name, property, value)
	}

	if property == "vmx" {
		return v.WriteHostFile(&vm, vm.Name+".vmx", []byte(value))
	} else {
		key, ok := v.vmkey[strings.ToLower(property)]
		if ok {
			property = key
			switch property {
			case "Id", "Path", "Name", "IpAddress":
				return Fatalf("Property '%s' is read-only", key)

			case "DiskSize":
				return Fatalf("Use host utility vmware-vdiskmanager to modify %s", key)

			case "Running", "PowerState":
				return Fatalf("Use 'start', 'stop', or 'kill' to modify %s", key)

			case "MacAddress", "IsoFile", "IsoAttached", "IsoBootConnected", "SerialAttched", "SerialPipe", "VncEnabled", "VncPort", "FileShareEnabled", "ClipboardEnabled":
				return Fatalf("Use modify command to change %s", key)

			case "CpuCount":
				property = "numvcpus"

			case "RamSize":
				property = "memsize"
				size, err := SizeParse(value)
				if err != nil {
					return Fatal(err)
				}
				value = fmt.Sprintf("%d", size/MB)

			case "GuestTimeZone":
				property = "guestTimeZone"

			case "GuestOS":
				property = "guestOS"

			}
			err := v.requirePowerState(&vm, "off", fmt.Sprintf("modify '%s'", key))

			if err != nil {
				return Fatal(err)
			}
		}
		err = v.cli.SetParam(&vm, property, value)
		if err != nil {
			return Fatal(err)
		}
	}
	return nil
}

func (v *vmctl) getIpAddress(vm *VM) error {
	if v.debug {
		log.Printf("getIpAddress(%s)\n", vm.Name)
	}
	path, err := PathnameFormat(v.Remote, vm.Path)
	if err != nil {
		return Fatal(err)
	}
	var exitCode int
	olines, err := v.RemoteExec("vmrun getGuestIpAddress "+path, &exitCode)
	if err != nil {
		return Fatal(err)
	}
	if v.debug {
		log.Printf("getIpAddress: exitCode: %d\n", exitCode)
	}
	if len(olines) > 0 {
		addr := olines[0]
		if strings.HasPrefix(addr, "Error:") {
			addr = ""
		}
		vm.IpAddress = addr
	}
	if vm.IpAddress == "" && vm.MacAddress != "" {
		addr, err := v.ArpQuery(vm)
		if err != nil {
			return Fatal(err)
		}
		vm.IpAddress = addr
	}
	return nil
}

func (v *vmctl) getDisks(vm *VM) ([]VMDisk, bool, error) {
	if v.debug {
		log.Printf("getDisk(%s)\n", vm.Name)
	}
	disks := []VMDisk{}
	vmxData, err := v.ReadHostFile(vm, fmt.Sprintf("%s.vmx", vm.Name))
	if err != nil {
		return disks, false, Fatal(err)
	}

	var found bool
	vmdks, err := ScanVMX(vmxData)
	if err != nil {
		return disks, false, Fatal(err)
	}
	for device, filename := range vmdks {
		vmdkData, err := v.ReadHostFile(vm, filename)
		if err != nil {
			return disks, false, Fatal(err)
		}
		disk, err := NewVMDisk(device, filename, vmdkData)
		if err != nil {
			return disks, false, Fatal(err)
		}
		if !found {
			vm.DiskSize = disk.Size
		}
		found = true
		disks = append(disks, *disk)
	}
	if !found {
		fmt.Printf("[%s] WARNING: no vmdk disks detected in vmx file", vm.Name)
	}
	return disks, found, nil
}

func checkEncryptedError(vm *VM, err error) bool {
	if ENCRYPTED_VM_ERROR.MatchString(fmt.Sprintf("%v", err)) {
		log.Printf("WARNING: %s is encrypted\n", vm.Name)
		vm.Encrypted = true
		return true
	}
	return false
}
