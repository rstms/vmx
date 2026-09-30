package controller

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
	if true || v.debug {
		log.Printf("Start(%s, options, isoOptions)\noptions: %s\nisoOptions: %s\n",
			vid,
			FormatJSON(options),
			FormatJSON(isoOptions),
		)
	}
	vm, err := v.cli.GetVM(vid)
	if err != nil {
		return "", Fatal(err)
	}
	ok, err := v.checkPowerState(&vm, "start", "on")
	if err != nil {
		return "", Fatal(err)
	}
	if ok {
		return "already started", nil
	}

	state := "disconnected"
	isoOptions.ModifyISO = true
	if isoOptions.IsoBootConnected || isoOptions.IsoPresent {
		isoOptions.IsoPresent = true
		isoOptions.IsoBootConnected = true
		state = fmt.Sprintf("connected: %v", isoOptions.IsoFiles)
	}
	msg := fmt.Sprintf("[%s] starting with ISO %s", vid, state)
	if v.verbose {
		fmt.Println(msg)
	}
	log.Println(msg)

	_, err = v.Modify(vid, CreateOptions{}, isoOptions)
	if err != nil {
		return "", Fatal(err)
	}

	path, err := PathnameFormat(v.Remote, vm.Path)
	if err != nil {
		return "", Fatal(err)
	}
	command := ""
	args := []string{}
	env := []string{}
	var visibility string
	if options.FullScreen {
		if v.Remote == "windows" {
			command = "cmd"
			args = append(args, "/c", "start", "vmware", ">nul", "2>nul", "-n", "-q", "-X", path)
		} else {
			command = "vmware"
			args = append(args, "-n", "-q", "-X", path)
			env = append(env, "DISPLAY=:0")
		}
		visibility = "fullscreen"
		options.ModifyStretch = true
		options.StretchEnabled = true
	} else {
		// TODO: add '-vp password' to vmrun command for encrypted VMs
		if v.Remote == "windows" {
			command = "cmd"
			args = append(args, "/c", "start", "/MIN", "vmrun", "-T", "ws", "start", path)
		} else {
			command = "vmrun"
			args = append(args, "-T", "ws", "start", path)
		}
		if options.Background {
			visibility = "background"
			args = append(args, "nogui")
		} else {
			visibility = "windowed"
			args = append(args, "gui")
			env = append(env, "DISPLAY=:0")
		}
	}

	if options.ModifyStretch {
		err = v.setStretch(&vm, options.StretchEnabled)
		if err != nil {
			return "", Fatal(err)
		}
	}

	if v.verbose {
		fmt.Printf("[%s] Requesting %s start\n", vm.Name, visibility)
	}

	err = v.RemoteSpawn(command, args, &env, nil)
	if err != nil {
		return "", Fatal(err)
	}
	if v.verbose {
		fmt.Printf("[%s] Start request complete\n", vm.Name)
	}

	status := "start pending"
	if options.Wait {
		err := v.Wait(vid, "on")
		if err != nil {
			return "", Fatal(err)
		}
		status = "started"
	}
	return status, nil
}

func (v *vmctl) disconnectISO(vid string) error {
	isoOptions := IsoOptions{
		ModifyISO:        true,
		IsoPresent:       false,
		IsoBootConnected: false,
	}
	_, err := v.Modify(vid, CreateOptions{}, isoOptions)
	if err != nil {
		return Fatal(err)
	}
	msg := fmt.Sprintf("[%s] ISO disconnected\n", vid)
	log.Println(msg)
	if v.verbose {
		fmt.Println(msg)
	}
	return nil
}

func (v *vmctl) Stop(vid string, options StopOptions) (string, error) {
	if v.debug {
		log.Printf("Stop(%s, %+v)\n", vid, options)
	}
	vm, err := v.cli.GetVM(vid)
	if err != nil {
		return "", Fatal(err)
	}

	ok, err := v.checkPowerState(&vm, "stop", "off")
	if err != nil {
		return "", Fatal(err)
	}
	if ok {
		err := v.disconnectISO(vid)
		if err != nil {
			return "", Fatal(err)
		}
		return "already stopped", nil
	}
	path, err := PathnameFormat(v.Remote, vm.Path)
	if err != nil {
		return "", Fatal(err)
	}
	// FIXME: may need -vp PASSWORD here for encrypted instances
	command := "vmrun"
	args := []string{"-T", "ws", "stop", path}
	action := "shutdown"
	if options.PowerOff {
		action = "forced power down"
	}
	if v.verbose {
		fmt.Printf("[%s] Requesting %s\n", vm.Name, action)
	}

	_, err = v.RemoteExec(command, args, nil, nil)
	if err != nil {
		return "", Fatal(err)
	}

	if v.verbose {
		fmt.Printf("[%s] %s request complete\n", vm.Name, action)
	}

	status := "stop pending"
	if options.Wait {
		err := v.Wait(vid, "off")
		if err != nil {
			return "", Fatal(err)
		}
		status = "stopped"
	}
	err = v.disconnectISO(vid)
	if err != nil {
		return "", Fatal(err)
	}
	return status, nil
}
