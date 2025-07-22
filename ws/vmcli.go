package ws

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

type VMConfig map[string]any

type vmcli struct {
	v      *vmctl
	ByPath map[string]VID
	ByName map[string]VID
	ById   map[string]VID
}

func NewCliClient(v *vmctl) *vmcli {
	c := vmcli{v: v}
	return &c
}

func (c *vmcli) exec(vm *VM, command string, result any) error {
	path, err := PathFormat(c.v.Remote, vm.Path)
	if err != nil {
		return err
	}
	olines, err := c.v.RemoteExec(fmt.Sprintf("vmcli %s %s", command, path), nil)
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
	c.Reset()
	vids := []VID{}
	for _, path := range c.v.Roots {
		err := c.getPathVIDs(path)
		if err != nil {
			return vids, err
		}
	}
	for _, vid := range c.ByPath {
		vids = append(vids, vid)
	}
	return vids, nil
}

func (c *vmcli) getPathVIDs(path string) error {
	hostPath, err := PathFormat(c.v.Remote, path)
	if err != nil {
		return nil
	}
	files := make(map[string]bool)
	switch c.v.Remote {
	case "windows":
		command := "dir /B /AD " + hostPath
		dirs, err := c.v.RemoteExec(command, nil)
		if err != nil {
			return err
		}
		for _, dir := range dirs {
			dir = strings.TrimSpace(dir)
			if dir != "" {
				files[filepath.Join(path, dir, dir+".vmx")] = true
			}
		}
	default:
		command := fmt.Sprintf("find %s -maxdepth 2 -type f -name '*.vmx'", hostPath)
		lines, err := c.v.RemoteExec(command, nil)
		if err != nil {
			return err
		}
		for _, line := range lines {
			files[line] = true
		}
	}
	for file, _ := range files {
		path, err := PathNormalize(file)
		if err != nil {
			return err
		}
		name, err := PathToName(file)
		if err != nil {
			return err
		}
		vid := VID{
			Name: name,
			Path: path,
			Id:   base64.StdEncoding.EncodeToString([]byte(path)),
		}
		c.ById[vid.Id] = vid
		c.ByName[vid.Name] = vid
		c.ByPath[vid.Path] = vid
	}
	return nil
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
	vm.CpuCount, err = c.GetInt(config, "numvcpus", true)
	if err != nil {
		return err
	}
	vm.RamSize, err = c.GetSize(config, "memsize", true)
	if err != nil {
		return err
	}
	vm.IsoFile, err = c.GetPath(config, "ide1:0.filename", false)
	if err != nil {
		return err
	}
	vm.IsoAttached, err = c.GetBool(config, "ide1:0.present", false)
	if err != nil {
		return err
	}
	vm.IsoAttachOnStart, err = c.GetBool(config, "ide1:0.startConnected", false)
	if err != nil {
		return err
	}
	err = c.GetMacAddress(vm, config)
	if err != nil {
		return err
	}
	vm.SerialAttached, err = c.GetBool(config, "serial0.present", false)
	if err != nil {
		return err
	}
	vm.SerialPipe, err = c.GetPath(config, "serial0.fileName", false)
	if err != nil {
		return err
	}
	vm.VncEnabled, err = c.GetBool(config, "RemoteDisplay.vnc.enabled", false)
	if err != nil {
		return err
	}
	vm.VncPort, err = c.GetInt(config, "RemoteDisplay.vnc.port", false)
	if err != nil {
		return err
	}
	copyDisabled, err := c.GetBool(config, "isolation.tools.copy.disable", false)
	if err != nil {
		return err
	}
	pasteDisabled, err := c.GetBool(config, "isolation.tools.paste.disable", false)
	if err != nil {
		return err
	}
	dndDisabled, err := c.GetBool(config, "isolation.tools.dnd.disable", false)
	if err != nil {
		return err
	}
	shareDisabled, err := c.GetBool(config, "isolation.tools.hgfs.disable", false)
	if err != nil {
		return err
	}
	vm.FileShareEnabled = !shareDisabled
	vm.ClipboardEnabled = true
	if copyDisabled && pasteDisabled && dndDisabled {
		vm.ClipboardEnabled = false
	}

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
	command := fmt.Sprintf("configParams SetEntry %s %s", name, value)
	err := c.exec(vm, command, nil)
	if err != nil {
		return err
	}
	return nil
}

func (c *vmcli) QueryPowerState(vm *VM) error {
	var state struct{ PowerState string }
	err := c.exec(vm, "power query -f json", &state)
	if err != nil {
		return err
	}
	vm.PowerState = state.PowerState
	vm.Running = state.PowerState != "off"
	return nil
}

func (c *vmcli) GetParams(vm *VM) (*VMConfig, error) {
	var params VMConfig
	err := c.exec(vm, "configParams query -f json", &params)
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

func (c *vmcli) GetSize(config *VMConfig, key string, required bool) (string, error) {
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

func (c *vmcli) GetInt(config *VMConfig, key string, required bool) (int, error) {
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

func (c *vmcli) GetString(config *VMConfig, key string, required bool) (string, error) {
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

func (c *vmcli) GetPath(config *VMConfig, key string, required bool) (string, error) {
	value, err := c.GetString(config, key, required)
	if err != nil {
		return "", err
	}
	path, err := PathFormat(c.v.Remote, value)
	if err != nil {
		return "", err
	}
	return path, nil
}

func (c *vmcli) GetBool(config *VMConfig, key string, required bool) (bool, error) {
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
	addressType, err := c.GetString(config, key, false)
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
	addr, err := c.GetString(config, key, true)
	if err != nil {
		return err
	}
	vm.MacAddress = addr
	return nil
}

func (c *vmcli) GetIsoOptions(vm *VM, options *IsoOptions) error {
	config, err := c.GetParams(vm)
	if err != nil {
		return err
	}
	options.ModifyISO = true
	options.IsoPresent, err = c.GetBool(config, "ide1:0.present", false)
	if err != nil {
		return err
	}
	options.IsoFile, err = c.GetPath(config, "ide1:0.fileName", false)
	if err != nil {
		return err
	}
	options.IsoBootConnected, err = c.GetBool(config, "ide1:0.startConnected", false)
	if err != nil {
		return err
	}
	return nil
}

func (c *vmcli) GetIsoStartConnected(vm *VM) (bool, error) {
	config, err := c.GetParams(vm)
	if err != nil {
		return false, err
	}
	connected, err := c.GetBool(config, "ide1:0.startConnected", false)
	if err != nil {
		return false, err
	}
	return connected, nil
}

func (c *vmcli) SetIsoStartConnected(vm *VM, connected bool) error {
	label := "ide1:0"
	command := fmt.Sprintf("disk setStartConnected %s %v", label, connected)
	err := c.exec(vm, command, nil)
	if err != nil {
		return err
	}
	return nil
}

func (c *vmcli) SetIsoOptions(vm *VM, options *IsoOptions) error {
	label := "ide1:0"

	path, err := PathFormat(c.v.Remote, options.IsoFile)
	if err != nil {
		return err
	}

	command := fmt.Sprintf("disk setPresent %s %v", label, options.IsoPresent)
	err = c.exec(vm, command, nil)
	if err != nil {
		return err
	}

	if !options.IsoPresent {
		return nil
	}

	command = fmt.Sprintf("disk setBackingInfo %s cdrom_image %s false", label, path)
	err = c.exec(vm, command, nil)
	if err != nil {
		return err
	}

	command = fmt.Sprintf("disk setStartConnected %s %v", label, options.IsoBootConnected)
	err = c.exec(vm, command, nil)
	if err != nil {
		return err
	}
	return nil
}
