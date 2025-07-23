package ws

import (
	"log"
)

func (v *vmctl) Modify(vid string, options CreateOptions, isoOptions IsoOptions) (*[]string, error) {
	if v.debug {
		log.Printf("Modify(%s, options, isoOptions)\n", vid)
		out, err := FormatJSON(&options)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("CreateOptions: %s\n", out)
		out, err = FormatJSON(&isoOptions)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("IsoOptions: %s\n", out)
	}
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
	vmx, err := InitVMX(v.Remote, vm.Name, hostData)
	if err != nil {
		return nil, err
	}

	if isoOptions.ModifyISO {
		if isoOptions.IsoFile != "" {
			err := v.CheckISODownload(&vm, &isoOptions)
			if err != nil {
				return nil, err
			}
			formatted, err := FormatIsoPathname(v.IsoPath, isoOptions.IsoFile)
			if err != nil {
				return nil, err
			}
			isoOptions.IsoFile = formatted
		}
	}

	actions, err := vmx.Configure(&options, &isoOptions)

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
