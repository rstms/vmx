package workstation

import (
	"fmt"
	"log"
	"path/filepath"
)

type CreateOptions struct {
	GuestOS    string
	CpuCount   int
	MemorySize string

	DiskSize         string
	DiskPreallocated bool
	DiskSingleFile   bool

	HostTimeSync  bool
	GuestTimeZone string

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
		log.Printf("Create: %s %+v\n", name, options)
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
	vm.Path = filepath.Join(ViperGetString("vmware_path"), name, name+".vmx")

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

	if isoOptions.IsoFile != "" {
		path, err := PathFormat(v.Remote, FormatIsoPathname(v.IsoPath, isoOptions.IsoFile))
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
