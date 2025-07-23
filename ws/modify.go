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
	vmx, err := InitVMX(v.Remote, vm.Name, hostData)
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

		isoPathname, err := FormatIsoPathname(v.IsoPath, isoOptions.IsoFile)
		if err != nil {
			return nil, err
		}
		if v.debug {
			log.Printf("isoPathname=%s\n", isoPathname)
		}
		action, err := vmx.SetISO(isoOptions.IsoPresent, isoOptions.IsoBootConnected, isoPathname)
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

	// FIXME: add unimplemented options.ModifyXXXXX from CreateOptions

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
