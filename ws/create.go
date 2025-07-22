package ws

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
)

type CreateOptions struct {
	GuestOS    string
	CpuCount   int
	MemorySize string

	DiskSize         string
	DiskPreallocated bool
	DiskSingleFile   bool

	ModifyTimeSync bool
	HostTimeSync   bool

	ModifyTimeZone bool
	GuestTimeZone  string

	ModifyClipboard  bool
	ClipboardEnabled bool

	ModifyShare      bool
	FileShareEnabled bool
	SharedHostPath   string
	SharedGuestPath  string

	ModifyNIC  bool
	MacAddress string

	ModifyTTY    bool
	SerialPipe   string
	SerialClient bool
	SerialV2V    bool

	ModifyVNC  bool
	VNCEnabled bool
	VNCPort    int

	ModifyEFI bool
	EFIBoot   bool

	Wait bool
}

type DestroyOptions struct {
	Force bool
	Wait  bool
}

func NewCreateOptions() *CreateOptions {
	return &CreateOptions{
		CpuCount:   1,
		MemorySize: "1G",
		DiskSize:   "16G",
		GuestOS:    "other",
		MacAddress: "auto",
		VNCPort:    5900,
	}
}

func (v *vmctl) Create(name string, options CreateOptions, isoOptions IsoOptions) (VM, error) {
	var vm VM

	if v.debug {
		log.Printf("Create(name='%s', options='%+v' isoOptions='%+v'\n", name, options, isoOptions)
	}

	// check for existing instance
	_, err := v.cli.GetVM(name)
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
	vm.Path = filepath.Join(v.Roots[0], name, name+".vmx")

	// create instance directory
	dir, _ := filepath.Split(vm.Path)
	hostPath, err := PathFormat(v.Remote, dir)
	if err != nil {
		return vm, err
	}
	mkdirCommand := "mkdir " + hostPath
	//log.Printf("create mkdir command: %s\n", mkdirCommand)
	_, err = v.RemoteExec(mkdirCommand, nil)
	if err != nil {
		return vm, err
	}

	if isoOptions.IsoFile != "" {
		path, err := PathnameFormat(v.Remote, FormatIsoPathname(v.IsoPath, isoOptions.IsoFile))
		if err != nil {
			return vm, err
		}
		isoOptions.IsoFile = path
	}

	//fmt.Printf("Create: options.IsoFile=%s\n", options.IsoFile)

	// write vmx file
	vmx, err := GenerateVMX(v.Remote, name, &options, &isoOptions)
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

	if options.Wait {
		err := v.Wait(name, "off")
		if err != nil {
			return vm, err
		}

		_, err = v.Start(name, StartOptions{Background: true, Wait: true}, IsoOptions{})
		if err != nil {
			return vm, err
		}
		_, err = v.Stop(name, StopOptions{Wait: true})
		if err != nil {
			return vm, err
		}

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
	hostPath, err := PathnameFormat(v.Remote, dir)
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
