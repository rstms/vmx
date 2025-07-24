package ws

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/user"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const Version = "0.1.2"

var WINDOWS_ENV_PATTERN = regexp.MustCompile(`^WINDIR=.*WINDOWS.*`)
var ENCRYPTED_VM_ERROR = regexp.MustCompile(`Something went wrong while getting password from stdin`)
var NONEXISTENT_VM_ERROR = regexp.MustCompile(`VMX : '[^']*' does not exist!`)

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
	Hostname string
	Username string
	KeyFile  string
	Roots    []string
	IsoPath  string
	winexec  *WinExecClient
	cli      *vmcli
	Shell    string
	Local    string
	Remote   string
	debug    bool
	verbose  bool
	Version  string
	vmkey    map[string]string
}

// return true if VMWare Workstation Host is localhost
func isLocal() (bool, error) {
	remote := ViperGetString("host")
	if remote == "" || remote == "localhost" || remote == "127.0.0.1" {
		return true, nil
	}
	host, err := os.Hostname()
	if err != nil {
		return false, err
	}
	if host == remote {
		return true, nil
	}
	return false, nil
}

func (v *vmctl) detectRemoteOS() (string, error) {
	if v.debug {
		log.Println("detectRemoteOS")
	}
	olines, err := v.exec("ssh", append(v.sshArgs(), "env"), "", nil)
	if err != nil {
		return "", err
	}
	for _, line := range olines {
		if WINDOWS_ENV_PATTERN.MatchString(strings.ToUpper(line)) {
			return "windows", nil
		}
	}
	olines, err = v.exec("ssh", append(v.sshArgs(), "uname"), "", nil)
	if err != nil {
		return "", err
	}
	if len(olines) != 1 {
		return "", fmt.Errorf("unexpected uname response: %v", olines)
	}
	return strings.ToLower(olines[0]), nil
}

func NewController() (Controller, error) {

	ViperInit("vmx.")
	ViperSetDefault("vmware_roots", []string{"/var/vmware"})
	ViperSetDefault("iso_path", "/var/vmware/iso")
	ViperSetDefault("certs_path", "/var/vmware/iso/certs")
	ViperSetDefault("disable_keepalives", true)
	ViperSetDefault("idle_conn_timeout", 5)
	ViperSetDefault("iso_download.command", "curl --location --silent")
	ViperSetDefault("iso_download.ca_flag", "--cacert")
	ViperSetDefault("iso_download.client_cert_flag", "--cert")
	ViperSetDefault("iso_download.client_key_flag", "--key")
	ViperSetDefault("iso_download.filename_flag", "--output")
	ViperSetDefault("host", "localhost")
	user, err := user.Current()
	if err != nil {
		return &vmctl{}, err
	}
	ViperSetDefault("user", user.Username)

	v := vmctl{
		Hostname: ViperGetString("host"),
		Username: ViperGetString("user"),
		KeyFile:  os.ExpandEnv(ViperGetString("ssh_key")),
		verbose:  ViperGetBool("verbose"),
		debug:    ViperGetBool("debug"),
		Version:  Version,
	}

	roots := ViperGetStringSlice("vmware_roots")
	v.Roots = make([]string, len(roots))
	for i, root := range roots {
		normalized, err := PathNormalize(os.ExpandEnv(root))
		if err != nil {
			return nil, err
		}
		v.Roots[i] = normalized
	}
	path, err := PathNormalize(os.ExpandEnv(ViperGetString("iso_path")))
	if err != nil {
		return nil, err
	}
	v.IsoPath = path

	v.cli = NewCliClient(&v)

	v.Local = runtime.GOOS
	local, err := isLocal()
	if err != nil {
		return nil, err
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
		if ViperGetString("shell") == "winexec" {
			w, err := NewWinExecClient()
			if err != nil {
				return nil, err
			}
			v.winexec = w
			v.Shell = "winexec"
			v.Remote = "windows"
		} else {
			v.Shell = "ssh"
			remote, err := v.detectRemoteOS()
			if err != nil {
				return nil, err
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
		return err
	}
	if vm.PowerState != state {
		return fmt.Errorf("Power state '%s' is required to %s", state, action)
	}
	return nil
}

func (v *vmctl) checkPowerState(vm *VM, command, state string) (bool, error) {
	if v.debug {
		log.Printf("checkPowerState(%s, %s, %s)\n", vm.Name, command, state)
	}
	err := v.validatePowerState(state)
	if err != nil {
		return false, err
	}
	err = v.cli.QueryPowerState(vm)
	if err != nil {
		return false, err
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
			return err
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
		return fmt.Errorf("unknown power state: %s", state)
	}
}

func (v *vmctl) queryPowerState(vid string) (string, error) {
	v.cli.Reset()
	vm, err := v.cli.GetVM(vid)
	if err != nil {
		return "", err
	}
	err = v.cli.QueryPowerState(&vm)
	if err != nil {
		return "", err
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
		return err
	}

	if v.verbose {
		fmt.Printf("[%s] Awaiting power state: %s\n", vid, state)
	}
	start := time.Now()
	interval_seconds := ViperGetInt64("interval")
	interval := time.Duration(interval_seconds) * time.Second
	timeout_seconds := ViperGetInt64("timeout")
	timeout := time.Duration(timeout_seconds) * time.Second
	checkPower := true
	running := false
	for {
		if (state == "on") && !running {
			// if waiting for poweredOn, ensure vmrun shows the instance before querying with vmrest API
			checkPower = false
			vms, err := v.Show("", ShowOptions{Running: true})
			if err != nil {
				return err
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
				return err
			}

			if newState == state {
				if v.verbose {
					fmt.Printf("[%s] Detected power %s\n", vid, state)
				}
				return nil
			}
		}
		if timeout_seconds != 0 {
			if time.Since(start) > timeout {
				return fmt.Errorf("[%s] Timed out awaiting power state %s", vid, state)
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
		return VM{}, err
	}
	return vm, nil
}

func (v *vmctl) GetState(vid string) (*VMState, error) {

	vm, err := v.Get(vid)
	if err != nil {
		return nil, err
	}

	err = v.queryVM(&vm, QueryTypeState)
	if err != nil {
		return nil, err
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
		return "", err
	}

	switch strings.ToLower(property) {
	case "vmx":
		data, err := v.ReadHostFile(&vm, vm.Name+".vmx")
		if err != nil {
			return "", err
		}
		return string(data), nil

	case "power", "powerstate":
		err := v.cli.QueryPowerState(&vm)
		if err != nil {
			return "", err
		}
		return vm.PowerState, nil

	case "ip", "ipaddress", "ipaddr":
		err = v.getIpAddress(&vm)
		if err != nil {
			return "", err
		}
		return vm.IpAddress, nil

	case "disk", "disks", "diskinfo", "disksize", "disksizemb", "diskcapacity":
		disks, ok, err := v.getDisks(&vm)
		if err != nil {
			return "", err
		}
		if !ok {
			return "", fmt.Errorf("[%s] no disks found", vm.Name)
		}
		if property == "disksize" || property == "disksizemb" {
			return fmt.Sprintf("%v", vm.DiskSize), nil
		}
		if property == "diskcapacity" {
			return fmt.Sprintf("%v", disks[0].Capacity), nil
		}
		if property == "disk" {
			ret, err := FormatJSON(disks[0])
			if err != nil {
				return "", err
			}
			return ret, nil
		}
		ret, err := FormatJSON(disks)
		if err != nil {
			return "", err
		}
		return ret, nil

	case "mac", "macaddr", "macaddress":
		err := v.cli.GetMacAddress(&vm, nil)
		if err != nil {
			return "", err
		}
		return vm.MacAddress, nil

	case "state":
		state, err := v.GetState(vid)
		if err != nil {
			return "", err
		}
		ret, err := FormatJSON(state)
		if err != nil {
			return "", err
		}
		return ret, nil
	}

	switch strings.ToLower(property) {
	case "config":
		err = v.queryVM(&vm, QueryTypeConfig)
		if err != nil {
			return "", err
		}
		ret, err := FormatJSON(&vm)
		if err != nil {
			return "", err
		}
		return ret, nil
	case "all", "detail", "":
		err := v.queryVM(&vm, QueryTypeAll)
		if err != nil {
			return "", err
		}
		ret, err := FormatJSON(&vm)
		if err != nil {
			return "", err
		}
		return ret, nil
	}

	// try property as a VM key
	value, ok, err := v.queryVMProperty(&vm, property)
	if err != nil {
		return "", err
	}
	if ok {
		return value, nil
	}

	// try property as a VMX key
	value, err = v.cli.GetParam(&vm, property)
	if err != nil {
		return "", err
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
			return err
		}
		_, _, err = v.getDisks(vm)
		if err != nil {
			return err
		}
	}
	if queryType == QueryTypeState || queryType == QueryTypeAll {
		err := v.cli.QueryPowerState(vm)
		if err != nil {
			if checkEncryptedError(vm, err) {
				vm.Encrypted = true
				return nil
			}
			return err
		}
		err = v.cli.GetMacAddress(vm, nil)
		if err != nil {
			return err
		}
		err = v.getIpAddress(vm)
		if err != nil {
			return err
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
		return err
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
		return vmap, err
	}
	err = json.Unmarshal([]byte(data), &vmap)
	if err != nil {
		return vmap, err
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
		return "", false, err
	}

	vmap, err := v.toMap(vm)
	if err != nil {
		return "", false, err
	}

	value, ok := vmap[key]
	if ok {
		data, err := json.Marshal(value)
		if err != nil {
			return "", false, err
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
	return "", fmt.Errorf("cannot format '%s' as boolean", value)
}

func (v *vmctl) SetProperty(vid, property, value string) error {
	if v.debug {
		log.Printf("SetProperty(%s, %s, %s)\n", vid, property, value)
	}
	vm, err := v.cli.GetVM(vid)
	if err != nil {
		return err
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
				return fmt.Errorf("Property '%s' is read-only", key)

			case "DiskSize":
				return fmt.Errorf("Use host utility vmware-vdiskmanager to modify %s", key)

			case "Running", "PowerState":
				return fmt.Errorf("Use 'start', 'stop', or 'kill' to modify %s", key)

			case "MacAddress", "IsoFile", "IsoAttached", "IsoBootConnected", "SerialAttched", "SerialPipe", "VncEnabled", "VncPort", "FileShareEnabled", "ClipboardEnabled":
				return fmt.Errorf("Use modify command to change %s", key)

			case "CpuCount":
				property = "numvcpus"

			case "RamSize":
				property = "memsize"
				size, err := SizeParse(value)
				if err != nil {
					return err
				}
				value = fmt.Sprintf("%d", size/MB)

			case "GuestTimeZone":
				property = "guestTimeZone"

			case "GuestOS":
				property = "guestOS"

			}
			err := v.requirePowerState(&vm, "off", fmt.Sprintf("modify '%s'", key))

			if err != nil {
				return err
			}
		}
		err = v.cli.SetParam(&vm, property, value)
		if err != nil {
			return err
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
		return err
	}
	var exitCode int
	olines, err := v.RemoteExec("vmrun getGuestIpAddress "+path, &exitCode)
	if err != nil {
		return err
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
			return err
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
		return disks, false, err
	}

	var found bool
	vmdks, err := ScanVMX(vmxData)
	if err != nil {
		return disks, false, err
	}
	for device, filename := range vmdks {
		vmdkData, err := v.ReadHostFile(vm, filename)
		if err != nil {
			return disks, false, err
		}
		disk, err := NewVMDisk(device, filename, vmdkData)
		if err != nil {
			return disks, false, err
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
