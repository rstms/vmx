package workstation

import (
	"encoding/json"
	"fmt"
	"strings"
)

type vmcli struct {
	v *vmctl
}

func NewCliClient(v *vmctl) *vmcli {
	return &vmcli{v: v}
}

func (c *vmcli) exec(vm *VM, command string, result any) error {
	path, err := PathFormat(c.v.Remote, vm.Path)
	if err != nil {
		return err
	}
	olines, err := c.v.RemoteExec(command+" "+path, nil)
	if err != nil {
		return err
	}
	stdout := strings.Join(olines, "\n")
	err = json.Unmarshal([]byte(stdout), result)
	if err != nil {
		return err
	}
	return nil
}

func (c *vmcli) QueryPowerState(vm *VM) error {
	var state struct{ PowerState string }
	err := c.exec(vm, "vmcli power query -f json", &state)
	if err != nil {
		return err
	}
	vm.PowerState = state.PowerState
	vm.Running = state.PowerState != "off"
	return nil
}

func (c *vmcli) GetConfig(vm *VM) (map[string]string, error) {
	var config map[string]string
	err := c.exec(vm, "vmcli configParams query -f json", &config)
	if err != nil {
		return map[string]string{}, err
	}
	return config, nil
}

func (c *vmcli) GetMacAddress(vm *VM) error {
	config, err := c.GetConfig(vm)
	if err != nil {
		return err
	}
	key := "ethernet0.addressType"
	addressType, ok := config[key]
	if !ok {
		return fmt.Errorf("config value not found: %s", key)
	}
	key = "ethernet0.generatedAddress"
	if addressType == "static" {
		key = "ethernet0.address"
	}
	address, ok := config[key]
	if !ok {
		return fmt.Errorf("config value not found: %s", key)
	}
	vm.MacAddress = address
	return nil
}
