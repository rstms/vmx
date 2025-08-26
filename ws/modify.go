package ws

import (
	"log"
)

func (v *vmctl) Modify(vid string, options CreateOptions, isoOptions IsoOptions) (*[]string, error) {
	if v.debug {
		log.Printf("Modify(%s, options, isoOptions)\n", vid)
		copts := FormatJSON(&options)
		log.Printf("CreateOptions: %s\n", copts)
		iopts := FormatJSON(&isoOptions)
		log.Printf("IsoOptions: %s\n", iopts)
	}
	vm, err := v.cli.GetVM(vid)
	if err != nil {
		return nil, Fatal(err)
	}

	err = v.requirePowerState(&vm, "off", "modify the instance")

	if err != nil {
		return nil, Fatal(err)
	}

	vmxFilename := vm.Name + ".vmx"
	hostData, err := v.ReadHostFile(&vm, vmxFilename)
	if err != nil {
		return nil, Fatal(err)
	}
	vmx, err := InitVMX(v.Remote, vm.Name, hostData)
	if err != nil {
		return nil, Fatal(err)
	}

	if isoOptions.ModifyISO {
		if isoOptions.IsoFile != "" {
			err := v.CheckISODownload(&vm, &isoOptions)
			if err != nil {
				return nil, Fatal(err)
			}
			formatted, err := FormatIsoPathname(v.IsoPath, isoOptions.IsoFile)
			if err != nil {
				return nil, Fatal(err)
			}
			isoOptions.IsoFile = formatted
		}
	}

	actions, err := vmx.Configure(&options, &isoOptions)

	editedData, err := vmx.Read()
	if err != nil {
		return nil, Fatal(err)
	}
	err = v.WriteHostFile(&vm, vmxFilename, editedData)
	if err != nil {
		return nil, Fatal(err)
	}

	return &actions, nil
}
