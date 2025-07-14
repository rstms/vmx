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
	"runtime"
	"strings"
	"time"
)

const Version = "0.0.7"

type VID struct {
	Id   string
	Path string
	Name string
}

type VMState struct {
	Name       string
	PowerState string
	IpAddress  string
}

type VM struct {
	Id   string
	Path string
	Name string

	CpuCount   int
	RamSizeMb  int
	DiskSizeMb int

	MacAddress string
	IpAddress  string

	IsoAttached      bool
	IsoAttachOnStart bool
	IsoPath          string

	SerialAttached bool
	SerialPath     string

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
	CpuCount          int
	MemorySize        int64
	DiskType          VDiskType
	DiskSize          int64
	EFIBoot           bool
	HostTimeSync      bool
	GuestTimeZone     string
	EnableDragAndDrop bool
	EnableClipboard   bool
	MacAddress        string
	IsoSource         string
	soAttached        bool
	Start             *StartOptions
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

type ListOptions struct {
	Running bool
	Detail  bool
}

type QueryType int

const (
	QueryTypeConfig = iota
	QueryTypeState
	QueryTypeAll
)

type Controller interface {
	List(string, ListOptions) ([]VM, error)
	Create(string, CreateOptions) (VM, error)
	Destroy(string, DestroyOptions) error
	Start(string, StartOptions) error
	Stop(string, StopOptions) error
	Get(string) (VM, error)
	GetProperty(string, string) (string, error)
	SetProperty(string, string, string) error
	LocalExec(string) (int, []string, []string, error)
	RemoteExec(string) (int, []string, []string, error)
	Upload(string, string, string) error
	Download(string, string, string) error
	Close() error
}

type vmctl struct {
	Hostname string
	Username string
	KeyFile  string
	Path     string
	api      *APIClient
	relay    *Relay
	Shell    string
	Local    string
	Remote   string
	debug    bool
	verbose  bool
	Version  string
}

// return true if VMWare Workstation Host is localhost
func isLocal() (bool, error) {
	remote := viper.GetString("hostname")
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

func detectRemoteOS() (string, error) {
	vars, _, err := Run("ssh", "windows", "env")
	if err != nil {
		return "", err
	}
	for _, line := range vars {
		if strings.HasPrefix(line, "OS=Windows") {
			return "windows", nil
		}
	}
	olines, _, err := Run("ssh", "", "uname")
	if err != nil {
		return "", err
	}
	if len(olines) != 1 {
		return "", fmt.Errorf("unexpected uname response: %v", olines)
	}
	return strings.ToLower(olines[0]), nil
}

func NewController() (Controller, error) {

	_, keyfile, err := GetViperPath("private_key")
	if err != nil {
		return nil, err
	}

	v := vmctl{
		Hostname: viper.GetString("hostname"),
		Username: viper.GetString("username"),
		KeyFile:  keyfile,
		Path:     viper.GetString("path"),
		verbose:  viper.GetBool("verbose"),
		debug:    viper.GetBool("debug"),
		Version:  Version,
	}

	relayConfig := viper.GetString("relay")
	if relayConfig != "" {
		r, err := NewRelay(relayConfig)
		if err != nil {
			return nil, err
		}
		v.relay = r
	}

	client, err := newVMRestClient()
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
		v.Shell = "ssh"
		remote, err := detectRemoteOS()
		if err != nil {
			return nil, err
		}
		v.Remote = remote
	}
	return &v, nil
}

func (v *vmctl) Close() error {
	if v.relay != nil {
		return v.relay.Close()
	}
	return nil
}

func (v *vmctl) List(name string, options ListOptions) ([]VM, error) {
	log.Printf("List: %s %+v\n", name, options)

	vids := []VID{}

	if name == "" && options.Running {
		// we only need the running vms, so spoof vids with only the Name using vmrun output
		_, olines, _, err := v.RemoteExec("vmrun list")
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
	log.Printf("Create: %s %+v\n", name, options)
	return VM{}, fmt.Errorf("Error: %s", "unimplemented")
}

func (v *vmctl) Destroy(vid string, options DestroyOptions) error {
	log.Printf("Destroy: %s %+v\n", vid, options)
	return fmt.Errorf("Error: %s", "unimplemented")
}

func (v *vmctl) checkPowerState(vm *VM, command, state string) (bool, error) {
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
	fields := []string{}
	remoteShell := viper.GetString("remote_shell")
	if remoteShell != "" {
		fields = append(fields, remoteShell, "--")
	}
	var visibility string
	if options.FullScreen {
		if v.Remote == "windows" {
			fields = append(fields, "start")
		}
		fields = append(fields, []string{"vmware", "-n", "-q", "-X", path}...)
		visibility = "fullscreen"
	} else {
		// TODO: add '-vp password' to vmrun command for encrypted VMs
		fields = append(fields, []string{"vmrun", "-T", "ws", "start", path}...)
		if options.Background {
			visibility = "background"
			fields = append(fields, "nogui")
		} else {
			visibility = "windowed"
			fields = append(fields, "gui")
		}
	}

	err = v.setStretch(&vm)
	if err != nil {
		return err
	}

	if v.verbose {
		fmt.Printf("[%s] requesting %s start...\n", vm.Name, visibility)
	}

	command := strings.Join(fields, " ")
	_, _, _, err = v.RemoteExec(command)
	if err != nil {
		return err
	}
	if v.verbose {
		fmt.Printf("[%s] start request complete\n", vm.Name)
	}

	if options.Wait {
		err := v.WaitPowerState(vm.Id, "poweredOn")
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
	switch state {
	case "poweredOn", "poweredOff", "paused", "suspended":
		return nil
	default:
		return fmt.Errorf("unknown power state: %s", state)
	}
}

func (v *vmctl) WaitPowerState(vid, state string) error {
	if v.debug {
		log.Printf("WaitPowerState(%s, %s)\n", vid, state)
	}
	err := v.validatePowerState(state)
	if err != nil {
		return err
	}
	vm, err := v.api.GetVM(vid)
	if err != nil {
		return err
	}
	if v.verbose {
		fmt.Printf("[%s] awaiting %s...\n", vm.Name, state)
	}
	start := time.Now()
	timeout_seconds := viper.GetInt64("timeout")
	timeout := time.Duration(timeout_seconds) * time.Second
	for {
		err := v.api.GetPowerState(&vm)
		if err != nil {
			return err
		}
		if vm.PowerState == state {
			if v.verbose {
				fmt.Printf("[%s] %s\n", vm.Name, state)
			}
			return nil
		}
		if timeout_seconds != 0 {
			if time.Since(start) > timeout {
				return fmt.Errorf("[%s] timed out awaiting power state %s", vm.Name, state)
			}
		}
		time.Sleep(1000 * time.Millisecond)
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
	_, _, _, err = v.RemoteExec(command)
	if err != nil {
		return err
	}
	if v.verbose {
		fmt.Printf("[%s] %s request complete\n", vm.Name, action)
	}
	if options.Wait {
		err := v.WaitPowerState(vm.Id, "poweredOff")
		if err != nil {
			return err
		}
	}
	return nil
}

func (v *vmctl) Get(vid string) (VM, error) {
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

	switch property {
	case "vmx":
		data, err := v.ReadFile(&vm, vm.Name+".vmx")
		if err != nil {
			return "", err
		}
		return string(data), nil

	case "power", "PowerState":
		err := v.api.GetPowerState(&vm)
		if err != nil {
			return "", err
		}
		return vm.PowerState, nil

	case "IpAddress":
		err = v.getIpAddress(&vm)
		if err != nil {
			return "", err
		}
		return vm.IpAddress, nil

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

	switch property {
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
	if queryType == QueryTypeConfig || queryType == QueryTypeAll {
		err := v.api.GetConfig(vm)
		if err != nil {
			return err
		}
	}
	if queryType == QueryTypeConfig || queryType == QueryTypeAll {
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

func (v *vmctl) mapVM(vm *VM) (map[string]string, error) {
	var vmap map[string]string
	data, err := FormatJSON(vm)
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

	// create map of empty VM to check if property exists
	vmap, err := v.mapVM(&VM{})
	if err != nil {
		return "", false, err
	}

	_, ok := vmap[property]
	if !ok {
		return "", false, nil
	}

	err = v.queryVM(vm, QueryTypeAll)
	if err != nil {
		return "", false, err
	}

	// create map of populated VM to return value
	vmap, err = v.mapVM(vm)
	if err != nil {
		return "", false, err
	}

	value, ok := vmap[property]
	return value, ok, nil
}

func (v *vmctl) SetProperty(vid, property, value string) error {
	log.Printf("SetProperty(%s, %s, %s)\n", vid, property, value)
	vm, err := v.api.GetVM(vid)
	if err != nil {
		return err
	}
	if v.verbose {
		fmt.Printf("[%s] setting %s=%s\n", vm.Name, property, value)
	}
	if property == "vmx" {
		return v.WriteFile(&vm, vm.Name+".vmx", []byte(value))
	} else {
            // FIXME: handle setting VM property keys

		err = v.api.SetParam(&vm, property, value)
		if err != nil {
			return err
		}
	}
	return nil
}

func (v *vmctl) ReadFile(vm *VM, filename string) (string, error) {
	if v.debug {
		log.Printf("ReadFile(%s, %s)\n", vm.Name, filename)
	}
	tempFile, err := os.CreateTemp("", "vmx_read.*")
	if err != nil {
		return "", err
	}
	localPath := tempFile.Name()
	err = tempFile.Close()
	if err != nil {
		return "", err
	}
	defer os.Remove(localPath)

	err = v.DownloadFile(vm, localPath, filename)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(localPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (v *vmctl) WriteFile(vm *VM, filename string, data []byte) error {
	if v.debug {
		log.Printf("WriteFile(%s, %s, (%d bytes))\n", vm.Name, filename, len(data))
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
	err = v.UploadFile(vm, localPath, filename)
	if err != nil {
		return err
	}
	return nil
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

	path, err := PathFormat(v.Remote, hostPath)
	if err != nil {
		return err
	}
	remoteSource := fmt.Sprintf("%s@%s:%s", v.Username, v.Hostname, path)
	args := []string{"-i", v.KeyFile, remoteSource, localPath}
	_, _, _, err = v.exec("scp", args, "")
	return err
}

func (v *vmctl) Upload(vid, localPath, filename string) error {
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

	path, err := PathFormat(v.Remote, hostPath)
	if err != nil {
		return err
	}
	remoteTarget := fmt.Sprintf("%s@%s:%s", v.Username, v.Hostname, path)
	args := []string{"-i", v.KeyFile, localPath, remoteTarget}
	_, _, _, err = v.exec("scp", args, "")
	return err
}

func (v *vmctl) getIpAddress(vm *VM) error {
	path, err := PathFormat(v.Remote, vm.Path)
	if err != nil {
		return err
	}
	_, olines, _, err := v.RemoteExec("vmrun getGuestIpAddress " + path)
	if err != nil {
		return err
	}
	if len(olines) > 0 {
		vm.IpAddress = olines[0]
	}
	return nil
}

func (v *vmctl) LocalExec(command string) (int, []string, []string, error) {
	log.Printf("localExec('%s')\n", command)
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
	return v.exec(shell, args, "")
}

func (v *vmctl) RemoteExec(command string) (int, []string, []string, error) {
	log.Printf("RemoteExec('%s') shell=%s\n", command, v.Shell)
	switch v.Shell {
	case "ssh":
		args := []string{"-q", "-i", v.KeyFile, v.Username + "@" + v.Hostname}
		if v.Remote == "windows" {
			args = append(args, command)
			command = ""
		}
		return v.exec(v.Shell, args, command)
	case "sh":
		return v.exec(v.Shell, []string{}, command)
	case "cmd":
		return v.exec(v.Shell, []string{"/c", command}, "")
	}
	return 255, []string{}, []string{}, fmt.Errorf("unexpected shell: %s", v.Shell)
}

func (v *vmctl) exec(command string, args []string, stdin string) (int, []string, []string, error) {
	log.Printf("exec('%s', '%v') stdin='%s'\n", command, args, stdin)
	olines := []string{}
	elines := []string{}
	exitCode := 0
	cmd := exec.Command(command, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if len(stdin) > 0 {
		cmd.Stdin = bytes.NewBuffer([]byte(stdin + "\n"))
	}
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	switch err.(type) {
	case *exec.ExitError:
		exitCode = cmd.ProcessState.ExitCode()
		err = nil
	}
	if err != nil {
		return 1, olines, elines, err
	}
	log.Printf("exit code: %d\n", exitCode)
	ostr := strings.TrimSpace(stdout.String())
	if ostr != "" {
		olines = strings.Split(ostr, "\n")
		if v.debug {
			for i, line := range olines {
				log.Printf("stdout[%d] %s\n", i, line)
			}
		}
	}
	estr := strings.TrimSpace(stderr.String())
	if estr != "" {
		elines = strings.Split(estr, "\n")
		for i, line := range elines {
			log.Printf("stderr[%d] %s\n", i, line)
		}
	}
	return exitCode, olines, elines, nil
}
