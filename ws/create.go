package ws

import (
	"fmt"
	"log"
	"path"
	"strings"
)

type CreateOptions struct {
	ModifyName bool
	Name       string

	ModifyGuestOS bool
	GuestOS       string

	ModifyCpu bool
	CpuCount  int

	ModifyMemory bool
	MemorySize   string

	ModifyDisk       bool
	DiskName         string
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

	ModifyFloppy bool

	Wait bool
}

type DestroyOptions struct {
	Force bool
	Wait  bool
}

func NewCreateOptions() *CreateOptions {
	return &CreateOptions{
		ModifyName:      true,
		ModifyCpu:       true,
		CpuCount:        1,
		ModifyMemory:    true,
		MemorySize:      "1G",
		ModifyDisk:      true,
		DiskSize:        "16G",
		ModifyEFI:       true,
		ModifyTimeSync:  true,
		ModifyTimeZone:  true,
		GuestTimeZone:   "UTC",
		ModifyGuestOS:   true,
		GuestOS:         "other",
		ModifyNIC:       true,
		MacAddress:      "auto",
		ModifyClipboard: true,
		ModifyFloppy:    true,
		VNCPort:         5900,
	}
}

func (v *vmctl) Create(name string, options CreateOptions, isoOptions IsoOptions) (string, error) {

	if v.debug {
		log.Printf("Create(name='%s', options='%+v' isoOptions='%+v'\n", name, options, isoOptions)
	}

	// check for existing instance
	_, err := v.cli.GetVM(name)
	if err == nil {
		return "", fmt.Errorf("create failed, instance '%s' exists", name)
	}

	// log create options
	ostr, err := FormatJSON(&options)
	if err != nil {
		return "", err
	}
	log.Printf("create: %s\n%s\n", name, ostr)

	vm, err := v.cli.Create(name, options.GuestOS)
	if err != nil {
		return "", err
	}

	err = v.CreateDisk(vm, options)
	if err != nil {
		return "", err
	}

	options.Name = vm.Name
	options.DiskName = vm.Name + ".vmdk"

	actions, err := v.Modify(vm.Name, options, isoOptions)
	if err != nil {
		return "", err
	}

	if v.verbose {
		for _, action := range *actions {
			fmt.Printf("[%s] %s\n", vm.Name, action)
		}
	}

	if options.Wait {
		err := v.Wait(name, "off")
		if err != nil {
			return "", err
		}
		_, err = v.Start(name, StartOptions{Background: true, Wait: true}, IsoOptions{})
		if err != nil {
			return "", err
		}
		_, err = v.Stop(name, StopOptions{Wait: true})
		if err != nil {
			return "", err
		}
		return "created", nil
	}

	return "create pending", nil
}

func (v *vmctl) CreateDisk(vm *VM, options CreateOptions) error {

	vmDir, _ := path.Split(vm.Path)

	hostDiskFile, err := PathFormat(v.Remote, path.Join(vmDir, vm.Name+".vmdk"))
	if err != nil {
		return err
	}

	// DANGER, WILL ROBINSON! - delete vmdk file created by 'vmcli VM Create'
	var command string
	switch v.Remote {
	case "windows":
		command = "del " + hostDiskFile
	default:
		command = "rm " + hostDiskFile
	}

	_, err = v.RemoteExec(command, nil)
	if err != nil {
		return err
	}

	diskSize, err := SizeParse(options.DiskSize)
	if err != nil {
		return err
	}
	diskSizeMB := int64(diskSize / MB)

	diskType := ParseDiskType(options.DiskSingleFile, options.DiskPreallocated)

	command = fmt.Sprintf("vmware-vdiskmanager -c -s %dMB -a nvme -t %d %s", diskSizeMB, diskType, hostDiskFile)

	olines, err := v.RemoteExec(command, nil)
	if err != nil {
		return err
	}
	if v.verbose {
		for _, line := range olines {
			fmt.Printf("[%s] %s\n", vm.Name, line)
		}
	}

	return nil

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
	dir, _ := path.Split(vm.Path)
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
