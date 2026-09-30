package controller

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"path"
	"strconv"
	"strings"
)

const MAX_SATA_DEVICES = 9

type VMConfig map[string]any

type vmcli struct {
	v      *vmctl
	debug  bool
	ByPath map[string]*VID
	ByName map[string]*VID
	ById   map[string]*VID
}

func NewCliClient(v *vmctl) *vmcli {
	log.Printf("NewCliClient\n")
	c := vmcli{v: v, debug: ViperGetBool("debug")}
	return &c
}

func (c *vmcli) exec(vm *VM, args []string, result any) error {
	hostPath, err := PathnameFormat(c.v.Remote, vm.Path)
	if err != nil {
		return Fatal(err)
	}
	args = append([]string{hostPath}, args...)
	olines, err := c.v.RemoteExec("vmcli", args, nil, nil)
	if err != nil {
		return Fatal(err)
	}
	if result != nil {
		stdout := strings.Join(olines, "\n")
		err = json.Unmarshal([]byte(stdout), result)
		if err != nil {
			return Fatal(err)
		}
	}
	return nil
}

func (c *vmcli) execCommand(name, command string, args []string, lines int) error {
	if c.v.debug {
		log.Printf("[%s] %s %v\n", name, command, args)
	}
	olines, err := c.v.RemoteExec(command, args, nil, nil)
	if err != nil {
		return Fatal(err)
	}
	var count int
	if c.v.verbose && len(olines) > 0 {
		for _, line := range olines {
			line = strings.TrimSpace(line)
			if line != "" {
				if lines == 0 || count < lines {
					fmt.Printf("[%s] %s\n", name, line)
				}
				count += 1
			}
		}
	}
	return nil
}

func (c *vmcli) GetVIDs() ([]*VID, error) {
	c.Reset()
	vids := []*VID{}
	for _, rootPath := range c.v.Roots {
		err := c.getPathVIDs(rootPath)
		if err != nil {
			return vids, Fatal(err)
		}
	}
	for _, vid := range c.ByPath {
		vids = append(vids, vid)
	}
	return vids, nil
}

func (c *vmcli) getPathVIDs(vmPath string) error {
	hostPath, err := PathnameFormat(c.v.Remote, vmPath)
	if err != nil {
		return nil
	}
	files := make(map[string]bool)
	switch c.v.Shell {
	case "winexec":
		err := c.v.checkWinexec()
		if err != nil {
			return Fatal(err)
		}
		dirs, err := c.v.winexec.DirSubs(hostPath)
		if err != nil {
			return Fatal(err)
		}

		for _, dir := range dirs {
			dir = strings.TrimSpace(dir)
			if dir != "" {
				normalDir, err := PathNormalize(dir)
				if err != nil {
					return Fatal(err)
				}
				vmxFile := path.Join(vmPath, normalDir, normalDir+".vmx")
				exists, err := c.v.winexec.IsFile(vmxFile)
				if err != nil {
					return Fatal(err)
				}
				if exists {
					files[vmxFile] = true
				}
			}
		}
	default:
		if c.v.Remote == "windows" {
			return Fatalf("unexpected shell: %s", c.v.Shell)
		}
		args := []string{hostPath, "-maxdepth", "2", "-type", "f", "-name", "*.vmx"}
		lines, err := c.v.RemoteExec("find", args, nil, nil)
		if err != nil {
			return Fatal(err)
		}
		for _, line := range lines {
			files[line] = true
		}
	}
	for file, _ := range files {
		_, err := c.newVID(file)
		if err != nil {
			return Fatal(err)
		}
	}
	return nil
}

func (c *vmcli) newVID(pathname string) (*VID, error) {
	vmxPath, err := PathNormalize(pathname)
	if err != nil {
		return nil, Fatal(err)
	}
	name, err := PathToName(vmxPath)
	if err != nil {
		return nil, Fatal(err)
	}
	vid := VID{
		Name: name,
		Path: vmxPath,
		Id:   base64.StdEncoding.EncodeToString([]byte(vmxPath)),
	}
	current, ok := c.ById[vid.Id]
	if ok {
		return nil, Fatalf("VM exists: '%+v'", *current)
	}
	c.ById[vid.Id] = &vid
	c.ByName[vid.Name] = &vid
	c.ByPath[vid.Path] = &vid
	return &vid, nil
}

func (c *vmcli) Reset() {
	c.ByPath = make(map[string]*VID)
	c.ByName = make(map[string]*VID)
	c.ById = make(map[string]*VID)
}

// search for a VM by Name or Id
func (c *vmcli) IsVM(vid string) (bool, error) {
	if len(c.ById) == 0 {
		// refresh ID index
		_, err := c.GetVIDs()
		if err != nil {
			return false, Fatal(err)
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
		return "", Fatal(err)
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
		return "", Fatalf("IsVM(%s) is true, but vid not in ById or ByName", vid)
	}
	return "", Fatalf("VM not found: %s", vid)
}

func (c *vmcli) GetVM(vid string) (VM, error) {
	id, err := c.GetId(vid)
	if err != nil {
		return VM{}, Fatal(err)
	}
	v, ok := c.ById[id]
	if !ok {
		return VM{}, Fatalf("ByID index failed: vid=%s, id=%s", vid, id)
	}
	vm := VM{Name: v.Name, Id: v.Id, Path: v.Path}
	return vm, nil
}

func (c *vmcli) GetConfig(vm *VM) error {

	config, err := c.GetParams(vm)
	if err != nil {
		return Fatal(err)
	}
	vm.CpuCount, err = c.GetInt(config, "numvcpus", true)
	if err != nil {
		return Fatal(err)
	}
	vm.RamSize, err = c.GetSize(config, "memsize", true)
	if err != nil {
		return Fatal(err)
	}
	vm.IsoFile, err = c.GetPath(config, "ide1:0.fileName", false)
	if err != nil {
		return Fatal(err)
	}
	vm.IsoAttached, err = c.GetBool(config, "ide1:0.present", false)
	if err != nil {
		return Fatal(err)
	}
	vm.IsoAttachOnStart, err = c.GetBool(config, "ide1:0.startConnected", false)
	if err != nil {
		return Fatal(err)
	}
	err = c.GetMacAddress(vm, config)
	if err != nil {
		return Fatal(err)
	}
	vm.SerialAttached, err = c.GetBool(config, "serial0.present", false)
	if err != nil {
		return Fatal(err)
	}
	vm.SerialPipe, err = c.GetPath(config, "serial0.fileName", false)
	if err != nil {
		return Fatal(err)
	}
	vm.VncEnabled, err = c.GetBool(config, "RemoteDisplay.vnc.enabled", false)
	if err != nil {
		return Fatal(err)
	}
	vm.VncPort, err = c.GetInt(config, "RemoteDisplay.vnc.port", false)
	if err != nil {
		return Fatal(err)
	}
	copyDisabled, err := c.GetBool(config, "isolation.tools.copy.disable", false)
	if err != nil {
		return Fatal(err)
	}
	pasteDisabled, err := c.GetBool(config, "isolation.tools.paste.disable", false)
	if err != nil {
		return Fatal(err)
	}
	dndDisabled, err := c.GetBool(config, "isolation.tools.dnd.disable", false)
	if err != nil {
		return Fatal(err)
	}
	shareDisabled, err := c.GetBool(config, "isolation.tools.hgfs.disable", false)
	if err != nil {
		return Fatal(err)
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
		return "", Fatal(err)
	}
	value, err := value(config, name)
	if err != nil {
		return "", Fatal(err)
	}
	ret := fmt.Sprintf("%v", value)
	return strings.Trim(ret, `"`), nil
}

func (c *vmcli) SetParam(vm *VM, name, value string) error {
	err := c.exec(vm, []string{"configParams", "SetEntry", name, value}, nil)
	if err != nil {
		return Fatal(err)
	}
	return nil
}

func (c *vmcli) QueryPowerState(vm *VM) error {
	if c.debug {
		log.Printf("[%s] QueryPowerState\n", vm.Name)
	}
	var state struct{ PowerState string }
	err := c.exec(vm, []string{"power", "query", "-f", "json"}, &state)
	if err != nil {
		return Fatal(err)
	}
	vm.PowerState = state.PowerState
	vm.Running = state.PowerState != "off"
	return nil
}

func (c *vmcli) GetParams(vm *VM) (*VMConfig, error) {
	if c.debug {
		log.Printf("[%s] GetParams\n", vm.Name)
	}
	var params VMConfig
	err := c.exec(vm, []string{"configParams", "query", "-f", "json"}, &params)
	if err != nil {
		if checkEncryptedError(vm, err) {
			return &VMConfig{}, nil
		}
		return nil, Fatal(err)
	}
	return &params, nil
}

func value(config *VMConfig, key string) (any, error) {
	v, ok := (*config)[key]
	if !ok {
		return nil, Fatalf("config value not found: %s", key)
	}
	return v, nil
}

func (c *vmcli) GetSize(config *VMConfig, key string, required bool) (string, error) {
	value, err := value(config, key)
	if err != nil {
		if required {
			return "", Fatal(err)
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
			return "", Fatal(err)
		}
		size = s
	default:
		return "", Fatalf("unexpected type (%v) for property: %s", T, key)
	}
	return FormatSize(size * MB), nil
}

func (c *vmcli) GetInt(config *VMConfig, key string, required bool) (int, error) {
	value, err := value(config, key)
	if err != nil {
		if required {
			return 0, Fatal(err)
		}
		return 0, nil
	}
	switch T := value.(type) {
	case int:
		return value.(int), nil
	case string:
		ivalue, err := strconv.Atoi(value.(string))
		if err != nil {
			return 0, Fatal(err)
		}
		return ivalue, nil
	default:
		return 0, Fatalf("unexpected type (%v) for property: %s", T, key)
	}
}

func (c *vmcli) GetString(config *VMConfig, key string, required bool) (string, error) {
	value, err := value(config, key)
	if err != nil {
		if required {
			return "", Fatal(err)
		}
		return "", nil
	}
	switch T := value.(type) {
	case string:
		return strings.Trim(value.(string), `"`), nil
	case int:
		return strconv.FormatInt(int64(value.(int)), 10), nil
	default:
		return "", Fatalf("unexpected type (%v) for property: %s", T, key)
	}
}

func (c *vmcli) GetPath(config *VMConfig, key string, required bool) (string, error) {
	value, err := c.GetString(config, key, required)
	if err != nil {
		return "", Fatal(err)
	}
	normalized, err := PathNormalize(value)
	if err != nil {
		return "", Fatal(err)
	}
	return normalized, nil
}

func (c *vmcli) GetBool(config *VMConfig, key string, required bool) (bool, error) {
	value, err := value(config, key)
	if err != nil {
		if required {
			return false, Fatal(err)
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
			return false, Fatalf("unexpected value '%v' for property: %s", value, key)
		}
	case int:
		return value != 0, nil
	default:
		return false, Fatalf("unexpected type (%v) for property: %s", T, key)
	}
}

func (c *vmcli) GetMacAddress(vm *VM, config *VMConfig) error {
	if config == nil {
		c, err := c.GetParams(vm)
		if err != nil {
			return Fatal(err)
		}
		config = c
	}
	key := "ethernet0.addressType"
	addressType, err := c.GetString(config, key, false)
	if err != nil {
		return Fatal(err)
	}
	if addressType == "" {
		vm.MacAddress = ""
		return nil
	}
	key = "ethernet0.generatedAddress"
	if addressType == "static" {
		key = "ethernet0.address"
	}
	addr, err := c.GetString(config, key, false)
	if err != nil {
		return Fatal(err)
	}
	vm.MacAddress = addr
	return nil
}

func (c *vmcli) GetIsoOptions(vm *VM, options *IsoOptions) error {
	config, err := c.GetParams(vm)
	if err != nil {
		return Fatal(err)
	}
	options.ModifyISO = true
	options.IsoPresent = false
	options.IsoFiles = []string{}
	options.IsoBootConnected = false
	sataPresent, err := c.GetBool(config, "sata0.present", false)
	if err != nil {
		return Fatal(err)
	}
	if !sataPresent {
		return nil
	}
	// NOTE: any connected iso will set IsoBootConnected true
	for i := 0; i < MAX_SATA_DEVICES; i++ {
		devicePresent, err := c.GetBool(config, fmt.Sprintf("sata0.%d.present", i), false)
		if err != nil {
			return Fatal(err)
		}
		if !devicePresent {
			continue
		}
		deviceType, err := c.GetString(config, fmt.Sprintf("sata0.%d.deviceType", i), true)
		if err != nil {
			return Fatal(err)
		}
		if deviceType != "cdrom-image" {
			continue
		}
		file, err := c.GetPath(config, fmt.Sprintf("sata0.%d.fileName", i), true)
		if err != nil {
			return Fatal(err)
		}
		if file == "" {
			continue
		}
		options.IsoFiles = append(options.IsoFiles, file)
		connected, err := c.GetBool(config, fmt.Sprintf("sata0:%d.startConnected", i), false)
		if err != nil {
			return Fatal(err)
		}
		if connected {
			options.IsoBootConnected = true
		}
	}
	return nil
}

func (c *vmcli) GetIsoStartConnected(vm *VM) (bool, error) {
	options := IsoOptions{}
	err := c.GetIsoOptions(vm, &options)
	if err != nil {
		return false, Fatal(err)
	}
	return options.IsoBootConnected, nil
}

func (c *vmcli) SetIsoStartConnected(vm *VM, connected bool) error {
	value := fmt.Sprintf("%v", connected)

	config, err := c.GetParams(vm)
	if err != nil {
		return Fatal(err)
	}
	// set start-connected state on any configured cdrom-image sata devices
	for i := 0; i < MAX_SATA_DEVICES; i++ {
		devicePresent, err := c.GetBool(config, fmt.Sprintf("sata0.%d.present", i), false)
		if err != nil {
			return Fatal(err)
		}
		if !devicePresent {
			continue
		}
		deviceType, err := c.GetString(config, fmt.Sprintf("sata0.%d.deviceType", i), true)
		if err != nil {
			return Fatal(err)
		}
		if deviceType != "cdrom-image" {
			continue
		}
		err = c.exec(vm, []string{"disk", "setStartConnected", fmt.Sprintf("sata0:%d", i), value}, nil)
		if err != nil {
			return Fatal(err)
		}
	}
	return nil
}

func (c *vmcli) SetIsoOptions(vm *VM, options *IsoOptions) error {

	config, err := c.GetParams(vm)
	if err != nil {
		return Fatal(err)
	}

	for i := 0; i < MAX_SATA_DEVICES; i++ {

		isoPresent := fmt.Sprintf("%v", options.IsoPresent)

		devicePresent, err := c.GetBool(config, fmt.Sprintf("sata0.%d.present", i), false)
		if err != nil {
			return Fatal(err)
		}

		device := fmt.Sprintf("sata0:%d", i)

		switch {
		case i >= len(options.IsoFiles):
			if devicePresent {
				err := c.exec(vm, []string{"disk", "setPresent", device, "false"}, nil)
				if err != nil {
					return Fatal(err)
				}
			}
		default:
			err := c.exec(vm, []string{"disk", "setPresent", device, isoPresent}, nil)
			if err != nil {
				return Fatal(err)
			}
			if options.IsoPresent {

				hostISOPath, err := PathnameFormat(c.v.Remote, options.IsoFiles[i])
				if err != nil {
					return Fatal(err)
				}

				err = c.exec(vm, []string{"disk", "setBackingInfo", device, "cdrom_image", hostISOPath, "false"}, nil)
				if err != nil {
					return Fatal(err)
				}

				startConnected := fmt.Sprintf("%v", options.IsoBootConnected)
				err = c.exec(vm, []string{"disk", "setStartConnected", device, startConnected}, nil)
				if err != nil {
					return Fatal(err)
				}
			}
		}
	}
	return nil
}

func (c *vmcli) Create(name, guestOS string) (*VM, error) {
	if c.debug {
		log.Printf("Create(%s, %s)\n", name, guestOS)
	}

	// make a VID, which will fail if the instance exists
	vid, err := c.newVID(path.Join(c.v.Roots[0], name, name+".vmx"))
	if err != nil {
		return nil, Fatal(err)
	}

	guestFlag, guestValue, err := GuestOsParams(guestOS)
	if err != nil {
		return nil, Fatal(err)
	}

	// create a directory for the new instance
	dir, _ := path.Split(vid.Path)
	hostPath, err := PathFormat(c.v.Remote, dir)
	if err != nil {
		return nil, Fatal(err)
	}

	switch c.v.Shell {
	case "winexec":
		err := c.v.checkWinexec()
		if err != nil {
			return nil, Fatal(err)
		}
		exists, err := c.v.winexec.IsDir(hostPath)
		if err != nil {
			return nil, Fatal(err)
		}
		if exists {
			log.Printf("directory exists: %s\n", hostPath)
			entries, err := c.v.winexec.DirEntries(hostPath)
			if err != nil {
				return nil, Fatal(err)
			}
			if len(entries) == 0 {
				log.Printf("removing empty directory: %s\n", hostPath)
				err := c.v.winexec.RemoveAll(hostPath)
				if err != nil {
					return nil, Fatal(err)
				}
			}
		}
		err = c.v.winexec.MkdirAll(hostPath, 0700)
		if err != nil {
			return nil, Fatal(err)
		}
	default:
		if c.v.Remote == "windows" {
			return nil, Fatalf("unexpected shell: %s", c.v.Shell)
		}
		_, err = c.v.RemoteExec("mkdir", []string{hostPath}, nil, nil)
		if err != nil {
			return nil, Fatal(err)
		}
	}

	// use vmcli to create the VM instance
	err = c.execCommand(name, "vmcli", []string{"VM", "Create", "-n", name, "-d", hostPath, guestFlag, guestValue}, 1)
	if err != nil {
		return nil, Fatal(err)
	}

	vm, err := c.GetVM(name)
	if err != nil {
		return nil, Fatal(err)
	}
	return &vm, nil
}

func (c *vmcli) diskPathnames(vm *VM, diskName string) (string, string, error) {
	vmxPath, vmxFile := path.Split(vm.Path)
	if !strings.HasSuffix(vmxFile, ".vmx") {
		return "", "", Fatalf("unexpected VM path: %s", vm.Path)
	}
	if !strings.HasSuffix(diskName, ".vmdk") {
		return "", "", Fatalf("unexpected disk name: %s", diskName)
	}

	diskPathname := path.Join(vmxPath, diskName)
	hostPathname, err := PathnameFormat(c.v.Remote, diskPathname)
	if err != nil {
		return "", "", Fatal(err)
	}
	return diskPathname, hostPathname, nil
}

func (c *vmcli) CreateDisk(vm *VM, diskName, size string, singleFile, preallocated bool) error {
	err := c.DeleteDisk(vm, diskName)
	if err != nil {
		return Fatal(err)
	}
	_, hostPathname, err := c.diskPathnames(vm, diskName)
	if err != nil {
		return Fatal(err)
	}
	adapter := "lsilogic"
	diskType := ParseDiskType(singleFile, preallocated)
	diskTypeStr := fmt.Sprintf("%d", int(diskType))
	err = c.execCommand(vm.Name, "vmcli", []string{"Disk", "Create", "-f", hostPathname, "-a", adapter, "-s", size, "-t", diskTypeStr}, 0)
	if err != nil {
		return Fatal(err)
	}
	return nil
}

// DANGER, WILL ROBINSON! - delete the instance's virtual disk file
func (c *vmcli) DeleteDisk(vm *VM, diskName string) error {
	_, hostPathname, err := c.diskPathnames(vm, diskName)
	if err != nil {
		return Fatal(err)
	}
	log.Printf("DeleteDisk: vm=%s, diskName=%s hostPathname=%s\n", vm.Name, diskName, hostPathname)
	switch c.v.Shell {
	case "winexec":
		err := c.v.checkWinexec()
		if err != nil {
			return Fatal(err)
		}
		err = c.v.winexec.DeleteFile(hostPathname)
		if err != nil {
			return Fatal(err)
		}
	default:
		if c.v.Remote == "windows" {
			return Fatalf("unexpected shell: %s", c.v.Shell)
		}
		err = c.execCommand(vm.Name, "rm", []string{hostPathname}, 0)
		if err != nil {
			return Fatal(err)
		}
	}
	return nil
}
