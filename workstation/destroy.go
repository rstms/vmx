package workstation

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
)

type DestroyOptions struct {
	Force bool
	Wait  bool
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
	hostPath, err := PathFormat(v.Remote, dir)
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
