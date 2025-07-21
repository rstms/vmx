package workstation

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/spf13/viper"
	"path/filepath"
	"strconv"
	"strings"
)

type VMConfig map[string]any

type vmcli struct {
	v       *vmctl
	ByPath  map[string]VID
	ByName  map[string]VID
	ById    map[string]VID
	verbose bool
	debug   bool
}

func NewCliClient(v *vmctl) *vmcli {
	c := vmcli{
		v:       v,
		verbose: viper.GetBool("verbose"),
		debug:   viper.GetBool("debug"),
	}
	return &c
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
	if result != nil {
		stdout := strings.Join(olines, "\n")
		err = json.Unmarshal([]byte(stdout), result)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *vmcli) GetVIDs() ([]VID, error) {
	vids := []VID{}
	path, err := PathFormat(c.v.Remote, c.v.Path)
	if err != nil {
		return vids, nil
	}
	files := make(map[string]bool)
	switch c.v.Remote {
	case "windows":
		command := "dir /B /AD " + path
		dirs, err := c.v.RemoteExec(command, nil)
		if err != nil {
			return vids, err
		}
		for _, dir := range dirs {
			dir = strings.TrimSpace(dir)
			if dir != "" {
				files[filepath.Join(c.v.Path, dir, dir+".vmx")] = true
			}
		}
	default:
		command := fmt.Sprintf("find %s -maxdepth 2 -type f -name '*.vmx'", path)
		lines, err := c.v.RemoteExec(command, nil)
		if err != nil {
			return vids, err
		}
		for _, line := range lines {
			files[line] = true
		}
	}
	c.Reset()
	for file, _ := range files {
		path, err := PathNormalize(file)
		if err != nil {
			return vids, err
		}
		name, err := PathToName(file)
		if err != nil {
			return vids, err
		}
		vid := VID{
			Name: name,
			Path: path,
			Id:   base64.StdEncoding.EncodeToString([]byte(path)),
		}
		c.ById[vid.Id] = vid
		c.ByName[vid.Name] = vid
		c.ByPath[vid.Path] = vid
		vids = append(vids, vid)
	}
	return vids, nil
}

func (c *vmcli) Reset() {
	c.ByPath = make(map[string]VID)
	c.ByName = make(map[string]VID)
	c.ById = make(map[string]VID)
}

// search for a VM by Name or Id
func (c *vmcli) IsVM(vid string) (bool, error) {
	if len(c.ById) == 0 {
		// refresh ID index
		_, err := c.GetVIDs()
		if err != nil {
			return false, err
		}
	}

	_, ok := c.ById[vid]
	if ok {
		// vid is a valid VM ID
		return true, nil
	}

	_, ok = c.ByName[vid]
	if ok {
		// vid is a valid VM Name
		return true, nil
	}
	return false, nil
}

// return VM ID by Name or ID; error if neither is found
func (c *vmcli) GetId(vid string) (string, error) {
	ok, err := c.IsVM(vid)
	if err != nil {
		return "", err
	}
	if ok {
		_, ok = c.ById[vid]
		if ok {
			// vid is a valid ID
			return vid, nil
		}

		v, ok := c.ByName[vid]
		if ok {
			// vid is a valid name, return ID
			return v.Id, nil
		}
		return "", fmt.Errorf("IsVM(%s) is true, but vid not in ById or ByName", vid)
	}
	return "", fmt.Errorf("VM not found: %s", vid)
}

func (c *vmcli) GetVM(vid string) (VM, error) {
	id, err := c.GetId(vid)
	if err != nil {
		return VM{}, err
	}
	v, ok := c.ById[id]
	if !ok {
		return VM{}, fmt.Errorf("ByID index failed: vid=%s, id=%s", vid, id)
	}
	vm := VM{Name: v.Name, Id: v.Id, Path: v.Path}
	return vm, nil
}

func (c *vmcli) GetConfig(vm *VM) error {

	config, err := c.GetParams(vm)
	if err != nil {
		return err
	}
	vm.CpuCount, err = c.IntValue(config, "numvcpus", true)
	if err != nil {
		return err
	}
	vm.RamSize, err = c.SizeValue(config, "memsize", true)
	if err != nil {
		return err
	}
	vm.IsoFile, err = c.PathValue(config, "ide1:0.filename", false)
	if err != nil {
		return err
	}
	vm.IsoAttached, err = c.BoolValue(config, "ide1:0.present", false)
	if err != nil {
		return err
	}
	vm.IsoAttachOnStart, err = c.BoolValue(config, "ide1:0.startConnected", false)
	if err != nil {
		return err
	}
	err = c.GetMacAddress(vm, config)
	if err != nil {
		return err
	}
	vm.SerialAttached, err = c.BoolValue(config, "serial0.present", false)
	if err != nil {
		return err
	}
	vm.SerialPipe, err = c.PathValue(config, "serial0.fileName", false)
	if err != nil {
		return err
	}
	vm.VncEnabled, err = c.BoolValue(config, "RemoteDisplay.vnc.enabled", false)
	if err != nil {
		return err
	}
	vm.VncPort, err = c.IntValue(config, "RemoteDisplay.vnc.port", false)
	if err != nil {
		return err
	}
	copyDisabled, err := c.BoolValue(config, "isolation.tools.copy.disable", false)
	if err != nil {
		return err
	}
	vm.EnableCopy = !copyDisabled
	pasteDisabled, err := c.BoolValue(config, "isolation.tools.paste.disable", false)
	if err != nil {
		return err
	}
	vm.EnablePaste = !pasteDisabled
	dndDisabled, err := c.BoolValue(config, "isolation.tools.dnd.disable", false)
	if err != nil {
		return err
	}
	vm.EnableDragAndDrop = !dndDisabled
	shareDisabled, err := c.BoolValue(config, "isolation.tools.hgfs.disable", false)
	if err != nil {
		return err
	}
	vm.EnableFilesystemShare = !shareDisabled

	return nil
}

func (c *vmcli) GetParam(vm *VM, name string) (string, error) {
	config, err := c.GetParams(vm)
	if err != nil {
		return "", err
	}
	value, err := value(config, name)
	if err != nil {
		return "", err
	}
	ret := fmt.Sprintf("%v", value)
	return strings.Trim(ret, `"`), nil
}

func (c *vmcli) SetParam(vm *VM, name, value string) error {
	command := fmt.Sprintf("vmcli configParams SetEntry %s %s", name, value)
	err := c.exec(vm, command, nil)
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

func (c *vmcli) GetParams(vm *VM) (*VMConfig, error) {
	var params VMConfig
	err := c.exec(vm, "vmcli configParams query -f json", &params)
	if err != nil {
		return nil, err
	}
	return &params, nil
}

func value(config *VMConfig, key string) (any, error) {
	v, ok := (*config)[key]
	if !ok {
		return nil, fmt.Errorf("config value not found: %s", key)
	}
	return v, nil
}

func (c *vmcli) SizeValue(config *VMConfig, key string, required bool) (string, error) {
	value, err := value(config, key)
	if err != nil {
		if required {
			return "", err
		}
		return "", nil
	}
	var size int64
	switch T := value.(type) {
	case int, uint, int32, uint32, int64, uint64:
		size = value.(int64)
	case string:
		s, err := strconv.ParseInt(value.(string), 10, 64)
		if err != nil {
			return "", err
		}
		size = s
	default:
		return "", fmt.Errorf("unexpected type (%v) for property: %s", T, key)
	}
	return FormatSize(size * MB), nil
}

func (c *vmcli) IntValue(config *VMConfig, key string, required bool) (int, error) {
	value, err := value(config, key)
	if err != nil {
		if required {
			return 0, err
		}
		return 0, nil
	}
	switch T := value.(type) {
	case int:
		return value.(int), nil
	case string:
		ivalue, err := strconv.Atoi(value.(string))
		if err != nil {
			return 0, err
		}
		return ivalue, nil
	default:
		return 0, fmt.Errorf("unexpected type (%v) for property: %s", T, key)
	}
}

func (c *vmcli) StringValue(config *VMConfig, key string, required bool) (string, error) {
	value, err := value(config, key)
	if err != nil {
		if required {
			return "", err
		}
		return "", nil
	}
	switch T := value.(type) {
	case string:
		return strings.Trim(value.(string), `"`), nil
	case int:
		return strconv.FormatInt(int64(value.(int)), 10), nil
	default:
		return "", fmt.Errorf("unexpected type (%v) for property: %s", T, key)
	}
}

func (c *vmcli) PathValue(config *VMConfig, key string, required bool) (string, error) {
	value, err := c.StringValue(config, key, required)
	if err != nil {
		return "", err
	}
	path, err := PathFormat(c.v.Remote, value)
	if err != nil {
		return "", err
	}
	return path, nil
}

func (c *vmcli) BoolValue(config *VMConfig, key string, required bool) (bool, error) {
	value, err := value(config, key)
	if err != nil {
		if required {
			return false, err
		}
		return false, nil
	}
	switch T := value.(type) {
	case bool:
		return value.(bool), nil
	case string:
		switch strings.Trim(value.(string), `"`) {
		case "TRUE":
			return true, nil
		case "FALSE":
			return false, nil
		default:
			return false, fmt.Errorf("unexpected value '%v' for property: %s", value, key)
		}
	case int:
		return value != 0, nil
	default:
		return false, fmt.Errorf("unexpected type (%v) for property: %s", T, key)
	}
}

func (c *vmcli) GetMacAddress(vm *VM, config *VMConfig) error {
	if config == nil {
		c, err := c.GetParams(vm)
		if err != nil {
			return err
		}
		config = c
	}
	key := "ethernet0.addressType"
	addressType, err := c.StringValue(config, key, false)
	if err != nil {
		return err
	}
	if addressType == "" {
		vm.MacAddress = ""
		return nil
	}
	key = "ethernet0.generatedAddress"
	if addressType == "static" {
		key = "ethernet0.address"
	}
	addr, err := c.StringValue(config, key, true)
	if err != nil {
		return err
	}
	vm.MacAddress = addr
	return nil
}
