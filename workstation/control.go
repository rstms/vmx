package workstation

import (
	"bytes"
	"fmt"
	"github.com/spf13/viper"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

const Version = "0.0.3"

type VID struct {
	Id   string
	Path string
}

type VM struct {
	VID
	Name string

	Running   bool
	IpAddress string

	CpuCount   int
	RamSizeMb  int
	DiskSizeMb int
	MacAddress string

	IsoAttached bool
	IsoPath     string

	SerialAttached bool
	SerialPath     string

	VncEnabled bool
	VncPort    int
	VncAddress string
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
	GUI        bool
	FullScreen bool
	Wait       bool
}

type StopOptions struct {
	PowerOff bool
	Wait     bool
}

type Controller interface {
	List(string, bool, bool) ([]VM, error)
	Create(string, CreateOptions) (VM, error)
	Destroy(string, DestroyOptions) error
	Start(string, StartOptions) error
	Stop(string, StopOptions) error
	Get(string, string) (any, error)
	Set(string, string, any) error
	Exec(string) (int, []string, []string, error)
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

func (v *vmctl) List(name string, detail, all bool) ([]VM, error) {
	vmList := []VM{}
	runningVmxFiles := make(map[string]bool)

	if !all {
		code, olines, _, err := v.Exec("vmrun list")
		if err != nil {
			return vmList, err
		}
		if code != 0 {
			return vmList, fmt.Errorf("vmrun list exited: %d", code)
		}
		for _, line := range olines {
			if !strings.HasPrefix(line, "Total running VMs:") {
				runningVmxFiles[line] = true
			}
		}
	}

	vids, err := v.api.GetVIDs()
	if err != nil {
		return vmList, err
	}

	log.Printf("runningVmxFiles: %+v\n", runningVmxFiles)

	for _, vid := range vids {
		log.Printf("vid: %+v\n", vid)
		/*
			if all || vmxFiles[vid.VmxPath] {
				vm, err := v.api.GetVM(&vmvmid.Id, vmid.VmxPath)
				if err != nil {
					return vmList, err
				}
				vmList = append(vmList, vm)
			}
		*/
	}
	return vmList, nil
}

func (v *vmctl) Create(name string, options CreateOptions) (VM, error) {
	log.Printf("Create: %s %+v\n", name, options)
	return VM{}, fmt.Errorf("Error: %s", "unimplemented")
}

func (v *vmctl) Destroy(name string, options DestroyOptions) error {
	log.Printf("Destroy: %s %+v\n", name, options)
	return fmt.Errorf("Error: %s", "unimplemented")
}

func (v *vmctl) Start(name string, options StartOptions) error {
	log.Printf("Start: %s %+v\n", name, options)
	return fmt.Errorf("Error: %s", "unimplemented")
}

func (v *vmctl) Stop(name string, options StopOptions) error {
	log.Printf("Stop: %s %+v\n", name, options)
	return fmt.Errorf("Error: %s", "unimplemented")
}

func (v *vmctl) Get(name, property string) (any, error) {
	log.Printf("Get: %s %+v\n", name, property)
	return nil, fmt.Errorf("Error: %s", "unimplemented")
}

func (v *vmctl) Set(name, property string, value any) error {
	log.Printf("Set: %s %s %+v\n", name, property, value)
	return fmt.Errorf("Error: %s", "unimplemented")
}

func (v *vmctl) Exec(command string) (int, []string, []string, error) {
	switch v.Shell {
	case "ssh":
		args := []string{"-q", "-i", v.KeyFile, v.Username + "@" + v.Hostname}
		if v.Remote == "windows" {
			args = append(args, command)
			command = ""
		}
		return v.shellExec(args, command)
	case "sh":
		return v.shellExec([]string{}, command)
	case "cmd":
		return v.shellExec([]string{"/c", command}, "")
	}
	return 255, []string{}, []string{}, fmt.Errorf("unexpected shell: %s", v.Shell)
}

func (v *vmctl) shellExec(args []string, stdin string) (int, []string, []string, error) {
	olines := []string{}
	elines := []string{}
	exitCode := 255
	cmd := exec.Command(v.Shell, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if len(stdin) > 0 {
		cmd.Stdin = bytes.NewBuffer([]byte(stdin + "\n"))
	}
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if v.debug {
		log.Printf("cmd: %+v", cmd)
		if len(stdin) > 0 {
			log.Printf("stdin: '%s'\n", stdin)
		}
	}
	err := cmd.Run()
	if err != nil {
		return exitCode, olines, elines, err
	}
	exitCode = cmd.ProcessState.ExitCode()
	if v.verbose {
		log.Printf("exit code: %d\n", exitCode)

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
	estr := strings.TrimSpace(stderr.String())
	if estr != "" {
		elines = strings.Split(estr, "\n")
		for i, line := range elines {
			log.Printf("stderr[%d] %s\n", i, line)
		}
	}
	return exitCode, olines, elines, nil
}
