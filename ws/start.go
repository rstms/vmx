package ws

import (
	"fmt"
	"log"
)

type StartOptions struct {
	Background     bool
	FullScreen     bool
	Wait           bool
	ModifyStretch  bool
	StretchEnabled bool
}

type StopOptions struct {
	PowerOff bool
	Wait     bool
}

func (v *vmctl) Start(vid string, options StartOptions, isoOptions IsoOptions) (string, error) {
	if v.debug {
		odump, err := FormatJSON(options)
		if err != nil {
			log.Fatal(err)
		}
		idump, err := FormatJSON(isoOptions)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Start(%s, options, isoOptions)\noptions: %s\nisoOptions: %s\n", vid, odump, idump)
	}
	vm, err := v.cli.GetVM(vid)
	if err != nil {
		return "", err
	}
	ok, err := v.checkPowerState(&vm, "start", "on")
	if err != nil {
		return "", err
	}
	if ok {
		return "already started", nil
	}

	var savedBootConnected bool
	if isoOptions.ModifyISO {
		savedBootConnected, err = v.cli.GetIsoStartConnected(&vm)
		if err != nil {
			return "", err
		}
		_, err = v.Modify(vid, CreateOptions{}, isoOptions)
		if err != nil {
			return "", err
		}
	}

	path, err := PathnameFormat(v.Remote, vm.Path)
	if err != nil {
		return "", err
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

	if options.ModifyStretch {
		err = v.setStretch(&vm, options.StretchEnabled)
		if err != nil {
			return "", err
		}
	}

	if v.verbose {
		fmt.Printf("[%s] requesting %s start...\n", vm.Name, visibility)
	}

	err = v.RemoteSpawn(command, nil)
	if err != nil {
		return "", err
	}
	if v.verbose {
		fmt.Printf("[%s] start request complete\n", vm.Name)
	}

	if options.Wait {
		err := v.Wait(vid, "on")
		if err != nil {
			return "", err
		}

		if isoOptions.ModifyISO {
			if savedBootConnected != isoOptions.IsoBootConnected {
				log.Printf("[%s] restoring saved ISO BootConnected state: %v\n", vm.Name, savedBootConnected)
				err := v.cli.SetIsoStartConnected(&vm, savedBootConnected)
				if err != nil {
					return "", err
				}
			}
		}

		return "started", nil
	}
	return "start pending", nil
}

func (v *vmctl) Stop(vid string, options StopOptions) (string, error) {
	if v.debug {
		log.Printf("Stop(%s, %+v)\n", vid, options)
	}
	vm, err := v.cli.GetVM(vid)
	if err != nil {
		return "", err
	}

	ok, err := v.checkPowerState(&vm, "stop", "off")
	if err != nil {
		return "", err
	}
	if ok {
		return "already stopped", nil
	}
	path, err := PathnameFormat(v.Remote, vm.Path)
	if err != nil {
		return "", err
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
		return "", err
	}

	if v.verbose {
		fmt.Printf("[%s] %s request complete\n", vm.Name, action)
	}
	if options.Wait {
		err := v.Wait(vid, "off")
		if err != nil {
			return "", err
		}
		return "stopped", nil
	}
	return "stop pending", nil
}
