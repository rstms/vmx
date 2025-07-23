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
		var currentIsoOptions IsoOptions
		err := v.cli.GetIsoOptions(&vm, &currentIsoOptions)
		if err != nil {
			return "", err
		}

		savedBootConnected = currentIsoOptions.IsoBootConnected

		//log.Printf("start: current ISO Options: %+v\n", currentIsoOptions)
		//log.Printf("start: new ISO Options: %+v\n", isoOptions)

		if !isoOptions.ModifyBootConnected {
			if isoOptions.IsoPresent != currentIsoOptions.IsoPresent {
				msg := fmt.Sprintf("[%s] setting ISO present: %v", vm.Name, isoOptions.IsoPresent)
				if v.verbose {
					fmt.Println(msg)
				}
				log.Println(msg)
			}
			if isoOptions.IsoPresent && (isoOptions.IsoFile != currentIsoOptions.IsoFile) {
				msg := fmt.Sprintf("[%s] setting ISO file: %s", vm.Name, isoOptions.IsoFile)
				if v.verbose {
					fmt.Println(msg)
				}
				log.Println(msg)
			}
		}
		if !isoOptions.IsoBootConnected != currentIsoOptions.IsoBootConnected {
			msg := fmt.Sprintf("[%s] setting ISO boot connected: %v", vm.Name, isoOptions.IsoBootConnected)
			if v.verbose {
				fmt.Println(msg)
			}
			log.Println(msg)
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
		fmt.Printf("[%s] Requesting %s start\n", vm.Name, visibility)
	}

	err = v.RemoteSpawn(command, nil)
	if err != nil {
		return "", err
	}
	if v.verbose {
		fmt.Printf("[%s] Start request complete\n", vm.Name)
	}

	if options.Wait {
		err := v.Wait(vid, "on")
		if err != nil {
			return "", err
		}

		if isoOptions.ModifyISO {
			if savedBootConnected != isoOptions.IsoBootConnected {
				msg := fmt.Sprintf("[%s] Restoring ISO boot-connected: %v\n", vm.Name, savedBootConnected)
				if v.verbose {
					fmt.Println(msg)
				}
				log.Println(msg)
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
		fmt.Printf("[%s] Requesting %s\n", vm.Name, action)
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
