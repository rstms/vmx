package workstation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const Version = "0.0.24"

var WINDOWS_ENV_PATTERN = regexp.MustCompile(`^WINDIR=.*WINDOWS.*`)
var VMX_PATTERN = regexp.MustCompile(`^.*\.[vV][mM][xX]$`)
var ISO_PATTERN = regexp.MustCompile(`^.*\.[iI][sS][oO]$`)
var ALL_PATTERN = regexp.MustCompile(`.*`)

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
}

type DestroyOptions struct {
	Force bool
	Wait  bool
}

type StartOptions struct {
	Background     bool
	FullScreen     bool
	Wait           bool
	ModifyStretch  bool
	StretchEnabled bool
}

type StopOptions struct {
	PowerOff bool
	Wait     bool
}

type FilesOptions struct {
	Detail bool
	All    bool
	Iso    bool
}

type ShowOptions struct {
	Running bool
	Detail  bool
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
	Create(string, CreateOptions, IsoOptions) (VM, error)
	Get(string) (VM, error)
	Modify(string, CreateOptions, IsoOptions) (*[]string, error)
	Start(string, StartOptions, IsoOptions) (string, error)
	Stop(string, StopOptions) (string, error)
	Destroy(string, DestroyOptions) error
	Show(string, ShowOptions) ([]VM, error)
	GetProperty(string, string) (string, error)
	SetProperty(string, string, string) error
	Upload(string, string, string) error
	Download(string, string, string) error
	Files(string, FilesOptions) ([]string, error)
	Wait(string, string) error
	SendKeys(string, string) error
	Close() error
	GetStatus(string) (*VMState, error)
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

	v := vmctl{
		Hostname: ViperGetString("host"),
		Username: ViperGetString("user"),
		KeyFile:  os.ExpandEnv(ViperGetString("ssh_key")),
		verbose:  ViperGetBool("verbose"),
		debug:    ViperGetBool("debug"),
		Version:  Version,
	}

	ViperSetDefault("vmware_roots", []string{"/var/vmware"})
	roots := ViperGetStringSlice("vmware_roots")
	v.Roots = make([]string, len(roots))
	for i, root := range roots {
		normalized, err := PathNormalize(os.ExpandEnv(root))
		if err != nil {
			return nil, err
		}
		v.Roots[i] = normalized
	}
	ViperSetDefault("iso_path", "/var/vmware/iso")
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

func (v *vmctl) Show(name string, options ShowOptions) ([]VM, error) {
	if v.debug {
		log.Printf("Show(%s, %+v)\n", name, options)
	}

	vids := []VID{}

	if name == "" && options.Running {
		// we only need the running vms, so spoof vids with only the Name using vmrun output
		olines, err := v.RemoteExec("vmrun list", nil)
		if err != nil {
			return []VM{}, err
		}
		for _, line := range olines {
			if !strings.HasPrefix(line, "Total running VMs:") {
				runningName, err := PathToName(line)
				if err != nil {
					return []VM{}, err
				}
				vids = append(vids, VID{Name: runningName})
			}
		}
	} else {
		// set vids from API
		v, err := v.cli.GetVIDs()
		if err != nil {
			return []VM{}, err
		}
		vids = v
	}

	selected := []VID{}
	for _, vid := range vids {
		if name == "" || (strings.ToLower(name) == strings.ToLower(vid.Name)) {
			selected = append(selected, vid)
		}
	}

	vms := make([]VM, len(selected))
	for i, vid := range selected {
		if options.Detail {
			vm, err := v.cli.GetVM(vid.Name)
			if err != nil {
				return []VM{}, err
			}
			err = v.queryVM(&vm, QueryTypeAll)
			if err != nil {
				return []VM{}, err
			}
			vms[i] = vm
		} else {
			vms[i] = VM{Name: vid.Name}
		}
	}
	return vms, nil
}

func (v *vmctl) Destroy(vid string, options DestroyOptions) error {
	if v.debug {
		log.Printf("Destroy: %s %+v\n", vid, options)
	}
	vm, err := v.Get(vid)
	if err != nil {
		return err
	}
	err = v.cli.QueryPowerState(&vm)
	if err != nil {
		return err
	}
	if vm.PowerState != "off" {
		if options.Force {
			_, err := v.Stop(vid, StopOptions{PowerOff: true, Wait: true})
			if err != nil {
				return err
			}
		} else {
			return fmt.Errorf("[%s] --kill required; power state is '%s'", vm.Name, vm.PowerState)
		}

	}
	dir, _ := filepath.Split(vm.Path)
	hostPath, err := PathFormat(v.Remote, dir)
	if err != nil {
		return err
	}
	hostPath = strings.TrimRight(hostPath, "/\\")
	var command string
	switch v.Remote {
	case "windows":
		command = "rmdir /S /Q " + hostPath
	default:
		command = "rm -rf " + hostPath
	}
	_, err = v.RemoteExec(command, nil)
	if err != nil {
		return err
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

func (v *vmctl) Start(vid string, options StartOptions, isoOptions IsoOptions) (string, error) {
	if v.debug {
		log.Printf("Start(%s, %+v)\n", vid, options)
	}
	vm, err := v.cli.GetVM(vid)
	if err != nil {
		return "", err
	}
	ok, err := v.checkPowerState(&vm, "start", "on")
	if err != nil {
		return "", err
	}
	if ok {
		return "already started", nil
	}

	var savedBootConnected bool
	if isoOptions.ModifyISO {
		savedBootConnected, err = v.cli.GetIsoStartConnected(&vm)
		if err != nil {
			return "", err
		}
		_, err = v.Modify(vid, CreateOptions{}, isoOptions)
		if err != nil {
			return "", err
		}
	}

	path, err := PathFormat(v.Remote, vm.Path)
	if err != nil {
		return "", err
	}
	command := ""
	var visibility string
	if options.FullScreen {
		if v.Remote == "windows" {
			command = "start vmware >nul 2>nul -n -q -X " + path
		} else {
			command = "vmware -n -q -X " + path + "&"
		}
		visibility = "fullscreen"
	} else {
		// TODO: add '-vp password' to vmrun command for encrypted VMs
		command = "vmrun -T ws start " + path
		if options.Background {
			visibility = "background"
			command += " nogui"
		} else {
			visibility = "windowed"
			command += " gui"
		}
	}

	if options.ModifyStretch {
		err = v.setStretch(&vm, options.StretchEnabled)
		if err != nil {
			return "", err
		}
	}

	if v.verbose {
		fmt.Printf("[%s] requesting %s start...\n", vm.Name, visibility)
	}

	err = v.RemoteSpawn(command, nil)
	if err != nil {
		return "", err
	}
	if v.verbose {
		fmt.Printf("[%s] start request complete\n", vm.Name)
	}

	if options.Wait {
		err := v.Wait(vid, "on")
		if err != nil {
			return "", err
		}

		if isoOptions.ModifyISO {
			log.Printf("[%s] saved ISO BootConnected state: %v\n", vm.Name, savedBootConnected)
			if savedBootConnected != isoOptions.IsoBootConnected {
				err := v.cli.SetIsoStartConnected(&vm, savedBootConnected)
				if err != nil {
					return "", err
				}
			}
		}

		return "started", nil
	}
	return "start pending", nil
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
		fmt.Printf("[%s] awaiting %s...\n", vid, state)
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
			for _, vm := range vms {
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
					fmt.Printf("[%s] %s\n", vid, state)
				}
				return nil
			}
		}
		if timeout_seconds != 0 {
			if time.Since(start) > timeout {
				return fmt.Errorf("[%s] timed out awaiting power state %s", vid, state)
			}
		}
		time.Sleep(interval)
		if running {
			checkPower = true
		}
	}
}

func (v *vmctl) Stop(vid string, options StopOptions) (string, error) {
	if v.debug {
		log.Printf("Stop(%s, %+v)\n", vid, options)
	}
	vm, err := v.cli.GetVM(vid)
	if err != nil {
		return "", err
	}

	ok, err := v.checkPowerState(&vm, "stop", "off")
	if err != nil {
		return "", err
	}
	if ok {
		return "already stopped", nil
	}
	path, err := PathFormat(v.Remote, vm.Path)
	if err != nil {
		return "", err
	}
	// FIXME: may need -vp PASSWORD here for encrypted instances
	command := "vmrun -T ws stop " + path
	action := "shutdown"
	if options.PowerOff {
		action = "forced power down"
	}
	if v.verbose {
		fmt.Printf("[%s] requesting %s...\n", vm.Name, action)
	}

	_, err = v.RemoteExec(command, nil)
	if err != nil {
		return "", err
	}

	if v.verbose {
		fmt.Printf("[%s] %s request complete\n", vm.Name, action)
	}
	if options.Wait {
		err := v.Wait(vid, "off")
		if err != nil {
			return "", err
		}
		return "stopped", nil
	}
	return "stop pending", nil
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

func (v *vmctl) GetStatus(vid string) (*VMState, error) {

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

	case "state", "status":
		state, err := v.GetStatus(vid)
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
			return err
		}
		err = v.getIpAddress(vm)
		if err != nil {
			return err
		}
		err = v.cli.GetMacAddress(vm, nil)
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

func (v *vmctl) ReadHostFile(vm *VM, filename string) ([]byte, error) {
	if v.debug {
		log.Printf("ReadHostFile(%s, %s)\n", vm.Name, filename)
	}
	tempFile, err := os.CreateTemp("", "vmx_read.*")
	if err != nil {
		return []byte{}, err
	}
	localPath := tempFile.Name()
	err = tempFile.Close()
	if err != nil {
		return []byte{}, err
	}
	defer os.Remove(localPath)

	err = v.DownloadFile(vm, localPath, filename)
	if err != nil {
		return []byte{}, err
	}
	return os.ReadFile(localPath)
}

func (v *vmctl) WriteHostFile(vm *VM, filename string, data []byte) error {
	if v.debug {
		log.Printf("WriteHostFile(%s, %s, (%d bytes))\n", vm.Name, filename, len(data))
	}
	tempFile, err := os.CreateTemp("", "vmx_write.*")
	if err != nil {
		return err
	}
	localPath := tempFile.Name()
	err = tempFile.Close()
	if err != nil {
		return err
	}
	defer os.Remove(localPath)
	err = os.WriteFile(localPath, data, 0600)
	if err != nil {
		return err
	}
	return v.UploadFile(vm, localPath, filename)
}

func (v *vmctl) copyFile(dstPath, srcPath string) error {
	if v.debug {
		log.Printf("copyFile(%s, %s)\n", dstPath, srcPath)
	}

	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dst.Close()
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()
	_, err = io.Copy(dst, src)
	return err

}

func (v *vmctl) Download(vid, localPath, filename string) error {
	if v.debug {
		log.Printf("Download(%s, %s, %s)\n", vid, localPath, filename)
	}
	vm, err := v.cli.GetVM(vid)
	if err != nil {
		return err
	}
	return v.DownloadFile(&vm, localPath, filename)
}

func (v *vmctl) DownloadFile(vm *VM, localPath, filename string) error {
	if v.debug {
		log.Printf("DownloadFile(%s, %s, %s)\n", vm.Name, localPath, filename)
	}

	if strings.ContainsAny(filename, ":/\\") {
		return fmt.Errorf("invalid characters in '%s'", filename)
	}

	hostDir, _ := filepath.Split(vm.Path)
	hostPath := filepath.Join(hostDir, filename)

	local, err := isLocal()
	if err != nil {

		return err
	}
	if local {
		hostPath, err := PathFormat(v.Local, hostPath)
		if err != nil {
			return err
		}
		return v.copyFile(localPath, hostPath)
	}

	path, err := PathFormat("scp", hostPath)
	if err != nil {
		return err
	}
	remoteSource := fmt.Sprintf("%s@%s:%s", v.Username, v.Hostname, path)
	args := []string{"-i", v.KeyFile, remoteSource, localPath}
	_, err = v.exec("scp", args, "", nil)
	return err
}

func (v *vmctl) Upload(vid, localPath, filename string) error {
	if v.debug {
		log.Printf("Upload(%s, %s, %s)\n", vid, localPath, filename)
	}
	vm, err := v.cli.GetVM(vid)
	if err != nil {
		return err
	}
	return v.UploadFile(&vm, localPath, filename)
}

func (v *vmctl) UploadFile(vm *VM, localPath, filename string) error {
	if v.debug {
		log.Printf("UploadFile(%s, %s, %s)\n", vm.Name, localPath, filename)
	}
	if strings.ContainsAny(filename, ":/\\") {
		return fmt.Errorf("invalid characters in '%s'", filename)
	}
	hostDir, _ := filepath.Split(vm.Path)
	hostPath := filepath.Join(hostDir, filename)
	local, err := isLocal()
	if err != nil {
		return err
	}
	if local {
		hostPath, err := PathFormat(v.Local, hostPath)
		if err != nil {
			return err
		}
		return v.copyFile(hostPath, localPath)
	}

	path, err := PathFormat("scp", hostPath)
	if err != nil {
		return err
	}
	remoteTarget := fmt.Sprintf("%s@%s:%s", v.Username, v.Hostname, path)
	args := []string{"-i", v.KeyFile, localPath, remoteTarget}
	_, err = v.exec("scp", args, "", nil)
	return err
}

func (v *vmctl) getIpAddress(vm *VM) error {
	if v.debug {
		log.Printf("getIpAddress(%s)\n", vm.Name)
	}
	path, err := PathFormat(v.Remote, vm.Path)
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

func (v *vmctl) LocalExec(command string, exitCode *int) ([]string, error) {
	if v.debug {
		log.Printf("LocalExec('%s', %v)\n", command, exitCode)
	}
	var shell string
	var args []string
	if v.Local == "windows" {
		shell = "cmd"
		args = []string{"/c", command}
	} else {
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/sh"
		}
		args = []string{"-c", command}
	}
	return v.exec(shell, args, "", exitCode)
}

func (v *vmctl) sshArgs() []string {
	return []string{"-q", "-i", v.KeyFile, v.Username + "@" + v.Hostname}
}

func (v *vmctl) RemoteExec(command string, exitCode *int) ([]string, error) {
	if v.debug {
		log.Printf("RemoteExec('%s', %v)\n", command, exitCode)
	}
	switch v.Shell {
	case "winexec":
		stdout, _, err := v.winexec.Exec("cmd", []string{"/c", command}, exitCode)
		if err != nil {
			return []string{}, err
		}
		return strings.Split(strings.TrimSpace(stdout), "\n"), nil
	case "ssh":
		args := v.sshArgs()
		if v.Remote == "windows" {
			args = append(args, command)
			command = ""
		}
		return v.exec(v.Shell, args, command, exitCode)
	case "sh":
		return v.exec(v.Shell, []string{}, command, exitCode)
	case "cmd":
		return v.exec(v.Shell, []string{"/c", command}, "", exitCode)
	}
	return []string{}, fmt.Errorf("unexpected shell: %s", v.Shell)
}

func (v *vmctl) RemoteSpawn(command string, exitCode *int) error {
	if v.debug {
		log.Printf("RemoteSpawn('%s', %v)\n", command, exitCode)
	}
	switch v.Shell {
	case "winexec":
		return v.winexec.Spawn(command, exitCode)
	case "ssh":
		args := v.sshArgs()
		if v.Remote == "windows" {
			args = append(args, command)
			command = ""
		}
		_, err := v.exec(v.Shell, args, command, exitCode)
		return err
	case "sh":
		return v.spawn("/bin/sh", command, exitCode)
	case "cmd":
		return v.spawn("cmd", command, exitCode)
	}
	return fmt.Errorf("unexpected shell: %s", v.Shell)
}

func (v *vmctl) spawn(shell, command string, exitCode *int) error {
	if v.debug {
		log.Printf("spawn('%s', %v)\n", command, exitCode)
	}
	stdin := ""
	args := []string{}
	if shell == "cmd" {
		args = []string{"/c", fmt.Sprintf("start /MIN %s", command)}
	} else {
		stdin = command + "&"
	}
	cmd := exec.Command(shell, args...)
	if len(stdin) > 0 {
		cmd.Stdin = bytes.NewBuffer([]byte(stdin + "\n"))
	} else {
		cmd.Stdin = nil
	}
	cmd.Stdout = nil
	cmd.Stderr = nil
	err := cmd.Run()
	switch e := err.(type) {
	case nil:
		if exitCode != nil {
			*exitCode = 0
		}
	case *exec.ExitError:
		if exitCode == nil {
			err = fmt.Errorf("Process '%s' exited %d", cmd, e.ProcessState.ExitCode())
		} else {
			*exitCode = e.ProcessState.ExitCode()
			log.Printf("WARNING: process '%s' exited %d\n", cmd, *exitCode)
			err = nil
		}
	}
	return nil
}

// note: if exitCode is nil, exit != 0 is an error, otherwise the exit code will be set
func (v *vmctl) exec(command string, args []string, stdin string, exitCode *int) ([]string, error) {
	if v.debug {
		log.Printf("exec('%s', %v, '%s', %v)\n", command, args, stdin, exitCode)
	}
	olines := []string{}
	elines := []string{}
	cmd := exec.Command(command, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if len(stdin) > 0 {
		cmd.Stdin = bytes.NewBuffer([]byte(stdin + "\n"))
	}
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	estr := strings.TrimSpace(stderr.String())
	if estr != "" {
		elines = strings.Split(estr, "\n")
		for i, line := range elines {
			log.Printf("stderr[%d] %s\n", i, line)
		}
	}
	ostr := strings.TrimSpace(stdout.String())
	if ostr != "" {
		olines = strings.Split(ostr, "\n")
		if v.debug {
			for i, line := range olines {
				log.Printf("stdout[%d] %s\n", i, line)
			}
		}
	}

	switch e := err.(type) {
	case nil:
		if exitCode != nil {
			*exitCode = 0
		}
	case *exec.ExitError:
		if exitCode == nil {
			err = fmt.Errorf("Process '%s' exited %d\n%s", cmd, e.ProcessState.ExitCode(), stderr.String())
		} else {
			*exitCode = e.ProcessState.ExitCode()
			log.Printf("WARNING: process '%s' exited %d\n%s", cmd, *exitCode, stderr.String())
			err = nil
		}
	}

	return olines, err
}

func (v *vmctl) Modify(vid string, options CreateOptions, isoOptions IsoOptions) (*[]string, error) {
	if v.debug {
		log.Printf("Modify(%s, %+v, %+v)\n", vid, options, isoOptions)
		out, err := FormatJSON(&options)
		if err != nil {
			return nil, err
		}
		log.Printf("CreateOptions: %s\n", out)
		out, err = FormatJSON(&isoOptions)
		if err != nil {
			return nil, err
		}
		log.Printf("IsoOptions: %s\n", out)
	}
	actions := []string{}
	vm, err := v.cli.GetVM(vid)
	if err != nil {
		return nil, err
	}

	err = v.requirePowerState(&vm, "off", "modify the instance")

	if err != nil {
		return nil, err
	}

	vmxFilename := vm.Name + ".vmx"
	hostData, err := v.ReadHostFile(&vm, vmxFilename)
	if err != nil {
		return nil, err
	}
	vmx, err := GenerateVMX(v.Remote, vm.Name, NewCreateOptions(), nil)
	if err != nil {
		return nil, err
	}
	err = vmx.Write(hostData)
	if err != nil {
		return nil, err
	}

	if options.ModifyNIC {
		action, err := vmx.SetEthernet(options.MacAddress)
		if err != nil {
			return nil, err
		}
		actions = append(actions, action)
	}

	if isoOptions.ModifyISO {
		if v.debug {
			log.Printf("ModifyISO: options.IsoFile=%s v.IsoPath=%s\n", isoOptions.IsoFile, v.IsoPath)
		}

		path := FormatIsoPathname(v.IsoPath, isoOptions.IsoFile)
		if path == "" {
			return nil, fmt.Errorf("failed formatting ISO pathname: %s", isoOptions.IsoFile)
		}
		if v.debug {
			log.Printf("normalized=%s\n", path)
		}
		action, err := vmx.SetISO(isoOptions.IsoPresent, isoOptions.IsoBootConnected, path)
		if err != nil {
			return nil, err
		}
		actions = append(actions, action)
	}

	if options.ModifyTTY {
		if v.debug {
			log.Printf("ModifyTTY: pipe=%s client=%v v2v=%v\n", options.SerialPipe, options.SerialClient, options.SerialV2V)
		}
		action, err := vmx.SetSerial(options.SerialPipe, options.SerialClient, options.SerialV2V)
		if err != nil {
			return nil, err
		}
		actions = append(actions, action)
	}

	if options.ModifyVNC {
		action, err := vmx.SetVNC(options.VNCEnabled, options.VNCPort)
		if err != nil {
			return nil, err
		}
		actions = append(actions, action)
	}

	if options.ModifyEFI {
		action, err := vmx.SetEFI(options.EFIBoot)
		if err != nil {
			return nil, err
		}
		actions = append(actions, action)
	}

	if options.ModifyClipboard {
		action, err := vmx.SetClipboard(options.ClipboardEnabled)
		if err != nil {
			return nil, err
		}
		actions = append(actions, action)
	}

	if options.ModifyShare {
		if v.debug {
			log.Printf("ModifyShare: enabled=%v host=%s guest=%s\n", options.FileShareEnabled, options.SharedHostPath, options.SharedGuestPath)
		}
		action, err := vmx.SetFileShare(options.FileShareEnabled, options.SharedHostPath, options.SharedGuestPath)
		if err != nil {
			return nil, err
		}
		actions = append(actions, action)
	}

	editedData, err := vmx.Read()
	if err != nil {
		return nil, err
	}
	err = v.WriteHostFile(&vm, vmxFilename, editedData)
	if err != nil {
		return nil, err
	}
	return &actions, nil
}
