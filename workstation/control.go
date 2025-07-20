package workstation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/spf13/viper"
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

const Version = "0.0.16"

var WINDOWS_ENV_PATTERN = regexp.MustCompile(`^WINDIR=.*WINDOWS.*`)

type VID struct {
	Id   string
	Path string
	Name string
}

type VMState struct {
	Name       string `json:"name"`
	PowerState string `json: "power_state"`
	IpAddress  string `json: ip"`
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

	EnableCopy            bool
	EnablePaste           bool
	EnableDragAndDrop     bool
	EnableFilesystemShare bool

	Running    bool
	PowerState string
}

type CreateOptions struct {
	GuestOS                string
	CpuCount               int
	MemorySize             string
	DiskSize               string
	DiskPreallocated       bool
	DiskSingleFile         bool
	EFIBoot                bool
	HostTimeSync           bool
	GuestTimeZone          string
	DisableDragAndDrop     bool
	DisableClipboard       bool
	DisableFilesystemShare bool
	MacAddress             string
	IsoPresent             bool
	IsoFile                string
	IsoBootConnected       bool
	SerialPipe             string
	SerialClient           bool
	SerialAppMode          bool
	VNCEnabled             bool
	VNCPort                int
	SharedHostPath         string
	SharedGuestPath        string
	ModifyNIC              bool
	ModifyISO              bool
	ModifyTTY              bool
	ModifyVNC              bool
	ModifyEFI              bool
	ModifyShare            bool
}

func NewCreateOptions() *CreateOptions {
	return &CreateOptions{
		CpuCount:               1,
		MemorySize:             "1G",
		DiskSize:               "16G",
		GuestOS:                "other",
		DisableDragAndDrop:     true,
		DisableClipboard:       true,
		DisableFilesystemShare: true,
		MacAddress:             "auto",
		SerialAppMode:          true,
		VNCPort:                5900,
	}
}

type DestroyOptions struct {
	Force bool
	Wait  bool
}

type StartOptions struct {
	Background bool
	FullScreen bool
	Wait       bool
}

type StopOptions struct {
	PowerOff bool
	Wait     bool
}

type FilesOptions struct {
	Detail bool
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
	Show(string, ShowOptions) ([]VM, error)
	Create(string, CreateOptions) (VM, error)
	Destroy(string, DestroyOptions) error
	Start(string, StartOptions) error
	Stop(string, StopOptions) error
	Get(string) (VM, error)
	GetProperty(string, string) (string, error)
	SetProperty(string, string, string) error
	LocalExec(string, *int) ([]string, error)
	RemoteExec(string, *int) ([]string, error)
	RemoteSpawn(string, *int) error
	Upload(string, string, string) error
	Download(string, string, string) error
	Files(string, FilesOptions) ([]string, []VMFile, error)
	Wait(string, string) error
	Modify(string, CreateOptions) (*[]string, error)
	Close() error
}

type vmctl struct {
	Hostname string
	Username string
	KeyFile  string
	Path     string
	IsoPath  string
	api      *VMRestClient
	winexec  *WinExecClient
	relay    *Relay
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
	remote := viper.GetString("host")
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

	_, keyfile, err := GetViperPath("ssh_key")
	if err != nil {
		return nil, err
	}

	v := vmctl{
		Hostname: viper.GetString("host"),
		Username: viper.GetString("user"),
		KeyFile:  keyfile,
		verbose:  viper.GetBool("verbose"),
		debug:    viper.GetBool("debug"),
		Version:  Version,
	}

	viper.SetDefault("vmware_path", "/var/vmware")
	v.Path, err = PathNormalize(viper.GetString("vmware_path"))
	if err != nil {
		return nil, err
	}
	viper.SetDefault("iso_path", "/var/vmware/iso")
	v.IsoPath, err = PathNormalize(viper.GetString("iso_path"))
	if err != nil {
		return nil, err
	}

	relayConfig := viper.GetString("ssh_relay")
	if relayConfig != "" {
		r, err := NewRelay(relayConfig)
		if err != nil {
			return nil, err
		}
		v.relay = r
	}

	client, err := NewVMRestClient()
	if err != nil {
		return nil, err
	}
	v.api = client

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
		if viper.GetString("shell") == "winexec" {
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
	if v.relay != nil {
		return v.relay.Close()
	}
	return nil
}

func (v *vmctl) Files(vid string, options FilesOptions) ([]string, []VMFile, error) {
	if v.debug {
		log.Printf("Files(%s, %+v)\n", vid, options)
	}
	lines := []string{}
	files := []VMFile{}

	sep := string(filepath.Separator)
	var path string
	if options.Iso {
		path = FormatIsoPath(v.IsoPath, vid)
	} else if strings.Contains(vid, sep) {
		path = vid
	} else if vid == "" {
		path = v.Path
	} else {
		vm, err := v.api.GetVM(vid)
		if err != nil {
			return lines, files, err
		}
		path, _ = filepath.Split(vm.Path)
	}

	path = strings.TrimRight(path, sep)

	hostPath, err := PathFormat(v.Remote, path)
	if err != nil {
		return lines, files, err
	}

	var command string
	if v.Remote == "windows" {
		if options.Detail {
			command = "dir /-C"
		} else {
			command = "dir /B"
		}
	} else {
		if options.Detail {
			command = "ls -l"
		} else {
			command = "ls"
		}
	}
	lines, err = v.RemoteExec(command+" "+hostPath, nil)
	if err != nil {
		return lines, files, err
	}
	if options.Detail {
		files, err = ParseFileList(v.Remote, lines)
		if err != nil {
			return lines, files, err
		}
	} else {
		trimmed := make([]string, len(lines))
		files = make([]VMFile, len(lines))
		for i, line := range lines {
			trimmed[i] = strings.TrimSpace(line)
			files[i] = VMFile{Name: trimmed[i]}
		}
		lines = trimmed
	}
	return lines, files, nil
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
		v, err := v.api.GetVIDs()
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
			vm, err := v.api.GetVM(vid.Name)
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

func (v *vmctl) Create(name string, options CreateOptions) (VM, error) {
	var vm VM

	if v.debug {
		log.Printf("Create: %s %+v\n", name, options)
	}

	// check for existing instance
	_, err := v.api.GetVM(name)
	if err == nil {
		return vm, fmt.Errorf("create failed, instance '%s' exists", name)
	}

	// display create options
	if v.verbose {
		ostr, err := FormatJSON(&options)
		if err != nil {
			return vm, err
		}
		fmt.Printf("[%s] create options: %s\n", name, ostr)
	}

	vm.Name = name
	vm.Path = filepath.Join(viper.GetString("vmware_path"), name, name+".vmx")

	// create instance directory
	dir, _ := filepath.Split(vm.Path)
	hostPath, err := PathFormat(v.Remote, dir)
	if err != nil {
		return vm, err
	}
	_, err = v.RemoteExec("mkdir "+hostPath, nil)
	if err != nil {
		return vm, err
	}

	if options.IsoFile != "" {
		path, err := PathFormat(v.Remote, FormatIsoPathname(v.IsoPath, options.IsoFile))
		if err != nil {
			return vm, err
		}
		options.IsoFile = path
	}

	//fmt.Printf("Create: options.IsoFile=%s\n", options.IsoFile)

	// write vmx file
	vmx, err := GenerateVMX(v.Remote, name, &options)
	if err != nil {
		return vm, err
	}
	data, err := vmx.Read()
	if err != nil {
		return vm, err
	}
	err = v.WriteHostFile(&vm, vm.Name+".vmx", data)
	if err != nil {
		return vm, err
	}

	// create vmdk disk
	pcd, err := PathChdirCommand(v.Remote, hostPath)
	if err != nil {
		return vm, err
	}
	command := pcd

	diskSize, err := SizeParse(options.DiskSize)
	if err != nil {
		return vm, err
	}
	diskSizeMB := int64(diskSize / MB)

	//fmt.Printf("options.DiskSize: %s\n", options.DiskSize)
	//fmt.Printf("diskSize: %d\n", diskSize)
	//fmt.Printf("diskSizeMB: %d\n", diskSizeMB)

	diskType := ParseDiskType(options.DiskSingleFile, options.DiskPreallocated)

	command += fmt.Sprintf("vmware-vdiskmanager -c -s %dMB -a nvme -t %d %s.vmdk", diskSizeMB, diskType, name)

	result, err := v.RemoteExec(command, nil)
	if err != nil {
		return vm, err
	}
	if v.verbose {
		fmt.Printf("[%s] %s\n", name, result)
	}

	return vm, nil
}

func (v *vmctl) Destroy(vid string, options DestroyOptions) error {
	if v.debug {
		log.Printf("Destroy: %s %+v\n", vid, options)
	}
	vm, err := v.Get(vid)
	if err != nil {
		return err
	}
	err = v.api.GetPowerState(&vm)
	if err != nil {
		return err
	}
	if vm.PowerState != "poweredOff" {
		if options.Force {
			err := v.Stop(vid, StopOptions{PowerOff: true, Wait: true})
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
	err := v.api.GetPowerState(vm)
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
	err = v.api.GetPowerState(vm)
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

func (v *vmctl) Start(vid string, options StartOptions) error {
	if v.debug {
		log.Printf("Start(%s, %+v)\n", vid, options)
	}
	vm, err := v.api.GetVM(vid)
	if err != nil {
		return err
	}
	ok, err := v.checkPowerState(&vm, "start", "poweredOn")
	if err != nil {
		return err
	}
	if ok {
		return nil
	}

	path, err := PathFormat(v.Remote, vm.Path)
	if err != nil {
		return err
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

	err = v.setStretch(&vm)
	if err != nil {
		return err
	}

	if v.verbose {
		fmt.Printf("[%s] requesting %s start...\n", vm.Name, visibility)
	}

	err = v.RemoteSpawn(command, nil)
	if err != nil {
		return err
	}
	if v.verbose {
		fmt.Printf("[%s] start request complete\n", vm.Name)
	}

	if options.Wait {
		err := v.Wait(vid, "poweredOn")
		if err != nil {
			return err
		}
	}
	return nil
}

func (v *vmctl) setStretch(vm *VM) error {
	var stretch string
	action := "unchanged"
	if viper.GetBool("stretch") {
		stretch = "TRUE"
		action = "enabled"
	}
	if viper.GetBool("no_stretch") {
		stretch = "FALSE"
		action = "disabled"
	}
	if v.debug {
		log.Printf("setStretch(%s) %s\n", vm.Name, action)
	}
	if stretch != "" {
		err := v.api.SetParam(vm, "gui.EnableStretchGuest", stretch)
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
	case "poweredOn", "poweredOff", "paused", "suspended":
		return nil
	default:
		return fmt.Errorf("unknown power state: %s", state)
	}
}

func (v *vmctl) queryPowerState(vid string) (string, error) {
	v.api.Reset()
	vm, err := v.api.GetVM(vid)
	if err != nil {
		return "", err
	}
	err = v.api.GetPowerState(&vm)
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
		state = "poweredOn"
	case "down", "off", "stopped":
		state = "poweredOff"
	}
	err := v.validatePowerState(state)
	if err != nil {
		return err
	}

	if v.verbose {
		fmt.Printf("[%s] awaiting %s...\n", vid, state)
	}
	start := time.Now()
	interval_seconds := viper.GetInt64("interval")
	interval := time.Duration(interval_seconds) * time.Second
	timeout_seconds := viper.GetInt64("timeout")
	timeout := time.Duration(timeout_seconds) * time.Second
	checkPower := true
	running := false
	for {
		if (state == "poweredOn") && !running {
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

func (v *vmctl) Stop(vid string, options StopOptions) error {
	if v.debug {
		log.Printf("Stop(%s, %+v)\n", vid, options)
	}
	vm, err := v.api.GetVM(vid)
	if err != nil {
		return err
	}

	ok, err := v.checkPowerState(&vm, "stop", "poweredOff")
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	path, err := PathFormat(v.Remote, vm.Path)
	if err != nil {
		return err
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
		return err
	}
	if v.verbose {
		fmt.Printf("[%s] %s request complete\n", vm.Name, action)
	}
	if options.Wait {
		err := v.Wait(vid, "poweredOff")
		if err != nil {
			return err
		}
	}
	return nil
}

func (v *vmctl) Get(vid string) (VM, error) {
	if v.debug {
		log.Printf("Get(%s)\n", vid)
	}
	vm, err := v.api.GetVM(vid)
	if err != nil {
		return VM{}, err
	}
	return vm, nil
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
		err := v.api.GetPowerState(&vm)
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
		value, err := v.api.GetParam(&vm, "ethernet0.generatedAddress")
		if err != nil {
			return "", err
		}
		return value, nil

	case "state", "status":
		err := v.api.GetState(&vm)
		if err != nil {
			return "", err
		}
		err = v.getIpAddress(&vm)
		if err != nil {
			return "", err
		}
		state := VMState{
			Name:       vm.Name,
			PowerState: vm.PowerState,
			IpAddress:  vm.IpAddress,
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
	value, err = v.api.GetParam(&vm, property)
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
		err := v.api.GetConfig(vm)
		if err != nil {
			return err
		}
		_, _, err = v.getDisks(vm)
		if err != nil {
			return err
		}
	}
	if queryType == QueryTypeState || queryType == QueryTypeAll {
		err := v.api.GetState(vm)
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
	vm, err := v.api.GetVM(vid)
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

			case "MacAddress":
				return fmt.Errorf("Use 'update nic' to modify %s", key)

			case "IsoFile", "IsoAttached", "IsoAttachOnStart":
				return fmt.Errorf("Use 'update iso' to modify %s", key)

			case "SerialAttached", "SerialPipe":
				return fmt.Errorf("Use 'update tty' to modify %s", key)

			case "VncEnabled", "VncPort":
				return fmt.Errorf("Use 'update vnc' to modify %s", key)

			case "EnableFileSystemShare":
				return fmt.Errorf("Use 'update share' to modify %s", key)

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

			case "EnableCopy":
				property = "isolation.tools.copy.disable"
				v, err := FormatVMXBool(value)
				if err != nil {
					return err
				}
				value = v
			case "EnablePaste":
				property = "isolation.tools.paste.disable"
				v, err := FormatVMXBool(value)
				if err != nil {
					return err
				}
				value = v
			case "EnableDragAndDrop":
				property = "isolation.tools.dnd.disable"
				v, err := FormatVMXBool(value)
				if err != nil {
					return err
				}
				value = v

			}
			err := v.requirePowerState(&vm, "poweredOff", fmt.Sprintf("modify '%s'", key))
			if err != nil {
				return err
			}
		}
		err = v.api.SetParam(&vm, property, value)
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
	vm, err := v.api.GetVM(vid)
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
	vm, err := v.api.GetVM(vid)
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

func (v *vmctl) Modify(vid string, options CreateOptions) (*[]string, error) {
	if v.debug {
		log.Printf("Modify(%s, %+v)\n", vid, options)
		out, err := FormatJSON(&options)
		if err != nil {
			return nil, err
		}
		log.Println(out)
	}
	actions := []string{}
	vm, err := v.api.GetVM(vid)
	if err != nil {
		return nil, err
	}

	err = v.requirePowerState(&vm, "poweredOff", "modify the instance")
	if err != nil {
		return nil, err
	}

	vmxFilename := vm.Name + ".vmx"
	hostData, err := v.ReadHostFile(&vm, vmxFilename)
	if err != nil {
		return nil, err
	}
	vmx, err := GenerateVMX(v.Remote, vm.Name, NewCreateOptions())
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

	if options.ModifyISO {
		if v.debug {
			log.Printf("ModifyISO: options.IsoFile=%s v.IsoPath=%s\n", options.IsoFile, v.IsoPath)
		}
		path := FormatIsoPathname(v.IsoPath, options.IsoFile)
		if path == "" {
			return nil, fmt.Errorf("failed formatting ISO pathname: %s", options.IsoFile)
		}
		if v.debug {
			log.Printf("normalized=%s\n", path)
		}
		action, err := vmx.SetISO(options.IsoPresent, options.IsoBootConnected, path)
		if err != nil {
			return nil, err
		}
		actions = append(actions, action)
	}

	if options.ModifyTTY {
		if v.debug {
			log.Printf("ModifyTTY: pipe=%s client=%v app=%v\n", options.SerialPipe, options.SerialClient, options.SerialAppMode)
		}
		action, err := vmx.SetSerial(options.SerialPipe, options.SerialClient, options.SerialAppMode)
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

	if options.ModifyShare {
		if v.debug {
			log.Printf("ModifyShare: host=%s guest=%s\n", options.SharedHostPath, options.SharedGuestPath)
		}
		action, err := vmx.SetShare(options.SharedHostPath, options.SharedGuestPath)
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
