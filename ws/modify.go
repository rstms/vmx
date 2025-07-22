package ws

import (
	"fmt"
	"log"
)

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
