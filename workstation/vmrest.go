package workstation

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os/user"

	"github.com/spf13/viper"
)

const VMREST_CONTENT_TYPE = "application/vnd.vmware.vmw.rest-v1+json"
const VMREST_PORT = 8697

type VMRestClient struct {
	api     *APIClient
	ByPath  map[string]VID
	ByName  map[string]VID
	ById    map[string]VID
	verbose bool
	debug   bool
}

func NewVMRestClient() (*VMRestClient, error) {
	url := viper.GetString("vmrest_url")
	headers := make(map[string]string)
	headers["Content-Type"] = VMREST_CONTENT_TYPE
	headers["Accept"] = VMREST_CONTENT_TYPE
	username := viper.GetString("vmrest_username")
	if username == "" {
		username = viper.GetString("username")
		if username == "" {
			user, err := user.Current()
			if err != nil {
				return nil, err
			}
			username = user.Username
		}
	}
	password := viper.GetString("vmrest_password")
	headers["Authorization"] = "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))
	api, err := NewAPIClient(url, "", "", "", &headers)
	if err != nil {
		return nil, err
	}
	client := VMRestClient{
		api:     api,
		verbose: viper.GetBool("verbose"),
		debug:   viper.GetBool("debug"),
	}
	client.Reset()
	return &client, nil
}

func (r *VMRestClient) Reset() {
	r.ByPath = make(map[string]VID)
	r.ByName = make(map[string]VID)
	r.ById = make(map[string]VID)
}

// GET /vms, generate index maps, return slice of VIDS
func (r *VMRestClient) GetVIDs() ([]VID, error) {
	path := "vms"
	var response []VID

	if r.verbose {
		log.Println("GetVIDs")
	}

	_, err := r.api.Get(path, &response)
	if err != nil {
		return []VID{}, fmt.Errorf("GET %s request failed: %v\n", path, err)
	}

	r.Reset()
	vids := make([]VID, len(response))
	for i, vid := range response {
		path, err := PathNormalize(vid.Path)
		if err != nil {
			return []VID{}, err
		}
		vid.Path = path
		name, err := PathToName(path)
		if err != nil {
			return []VID{}, err
		}
		vid.Name = name
		r.ById[vid.Id] = vid
		r.ByName[vid.Name] = vid
		r.ByPath[vid.Path] = vid
		vids[i] = vid
	}
	if r.verbose {
		LogJSON("GetVIDs returning: ", &vids)
	}
	return vids, nil
}

// search for a VM by Name or Id
func (r *VMRestClient) IsVM(vid string) (bool, error) {
	if len(r.ById) == 0 {
		// refresh ID index
		_, err := r.GetVIDs()
		if err != nil {
			return false, err
		}
	}

	_, ok := r.ById[vid]
	if ok {
		// vid is a valid VM ID
		return true, nil
	}

	_, ok = r.ByName[vid]
	if ok {
		// vid is a valid VM Name
		return true, nil
	}
	return false, nil
}

// return VM ID by Name or ID; error if neither is found
func (r *VMRestClient) GetId(vid string) (string, error) {
	ok, err := r.IsVM(vid)
	if err != nil {
		return "", err
	}
	if ok {
		_, ok = r.ById[vid]
		if ok {
			// vid is a valid ID
			return vid, nil
		}

		v, ok := r.ByName[vid]
		if ok {
			// vid is a valid name, return ID
			return v.Id, nil
		}
		return "", fmt.Errorf("IsVM(%s) is true, but vid not in ById or ByName", vid)
	}
	return "", fmt.Errorf("VM not found: %s", vid)
}

func (r *VMRestClient) GetVM(vid string) (VM, error) {
	id, err := r.GetId(vid)
	if err != nil {
		return VM{}, err
	}
	v, ok := r.ById[id]
	if !ok {
		return VM{}, fmt.Errorf("ByID index failed: vid=%s, id=%s", vid, id)
	}
	vm := VM{Name: v.Name, Id: v.Id, Path: v.Path}
	return vm, nil
}

// GET /vms/ID, add CpuCount, RamSizeMb to VM
func (r *VMRestClient) GetVMCpuRam(vm *VM) error {

	path := fmt.Sprintf("vms/%s", vm.Id)

	var response VmRestGetVmCpuRamResponse

	if r.verbose {
		log.Printf("GetVMCpuRam: %s\n", path)
	}

	_, err := r.api.Get(path, &response)
	if err != nil {
		return fmt.Errorf("GET %s request failed: %v\n", path, err)
	}

	if r.verbose {
		LogJSON("VmRestGetVmCpuRam", &response)
	}

	vm.CpuCount = response.Cpu.Processors
	vm.RamSize = FormatSize(int64(response.Memory) * MB)

	if r.verbose {
		LogJSON("GetVMCpuRam returning", vm)
	}

	return nil
}

// populate VM with configuration
func (r *VMRestClient) GetConfig(vm *VM) error {

	if r.verbose {
		log.Printf("GetConfig(%s)\n", vm.Id)
	}

	path := fmt.Sprintf("vms/%s/restrictions", vm.Id)

	var response VmRestGetVmRestrictionsResponse
	_, err := r.api.Get(path, &response)
	if err != nil {
		return fmt.Errorf("GET %s request failed: %v\n", path, err)
	}

	if r.verbose {
		LogJSON("VmRestGetVmRestrictionsResponse", &response)
	}

	if vm.Id != response.ID {
		return fmt.Errorf("response ID mismatch: expected=%s response=%s", vm.Id, response.ID)
	}
	vm.CpuCount = response.Cpu.Processors
	vm.RamSize = FormatSize(int64(response.Memory) * MB)
	vm.IsoFile = ""
	vm.IsoAttached = false
	vm.IsoAttachOnStart = false
	if len(response.CddvdList.Devices) > 0 {
		captured := false
		for _, device := range response.CddvdList.Devices {
			path := device.DevicePath
			if path != "" {
				if !captured {
					path, err = PathNormalize(path)
					if err != nil {
						return err
					}
					vm.IsoFile = path
					vm.IsoAttached = response.CddvdList.Devices[0].ConnectionStatus == 1
					vm.IsoAttachOnStart = response.CddvdList.Devices[0].StartConnected
					captured = true
				} else {
					log.Printf("WARNING: CD/DVD device %d not captured: %v", device.Index, device)
				}
			}
		}
	}
	vm.MacAddress = ""
	if len(response.NicList.Nics) > 0 {
		captured := false
		for _, nic := range response.NicList.Nics {
			if !captured {
				vm.MacAddress = nic.MacAddress
				captured = true
			} else {
				log.Printf("WARNING: NIC device %d not captured: %v", nic.Index, nic)
			}
		}
	}
	vm.SerialAttached = false
	vm.SerialPipe = ""
	if len(response.SerialPortList.Devices) > 0 {
		captured := false
		for _, port := range response.SerialPortList.Devices {
			if !captured {
				vm.SerialAttached = true
				// FIXME: catch serial pipe name here
				vm.SerialPipe = ""
				captured = true
			} else {
				log.Printf("WARNING: serial port %d not captured: %v", port.Index, port)
			}
		}
	}
	vm.VncEnabled = response.RemoteVnc.VncEnabled
	vm.VncPort = response.RemoteVnc.VncPort
	vm.EnableCopy = !response.GuestIsolation.CopyDisabled
	vm.EnablePaste = !response.GuestIsolation.PasteDisabled
	vm.EnableDragAndDrop = !response.GuestIsolation.DndDisabled
	vm.EnableFilesystemShare = !response.GuestIsolation.HgfsDisabled

	if r.verbose {
		LogJSON("GetConfig returning", &vm)
	}
	return nil
}

func (r *VMRestClient) GetParam(vm *VM, name string) (string, error) {
	var response map[string]any
	path := fmt.Sprintf("vms/%s/params/%s", vm.Id, url.PathEscape(name))
	_, err := r.api.Get(path, &response)
	if err != nil {
		return "", fmt.Errorf("GET %s request failed: %v\n", path, err)
	}
	if response["name"] == "" {
		return "", fmt.Errorf("unkown property: '%s'", name)
	}
	data, err := json.Marshal(response["value"])
	if err != nil {
		return "", err
	}
	return string(data), nil
	//return fmt.Sprintf("%v", response["value"]), nil
}

func (r *VMRestClient) SetParam(vm *VM, name, value string) error {
	requestString := fmt.Sprintf("{\"name\":\"%s\",\"value\": \"%s\"}", name, value)
	log.Printf("requestString: %s\n", requestString)
	request := []byte(requestString)
	var response map[string]any
	path := fmt.Sprintf("vms/%s/params", vm.Id)
	headers := map[string]string{"Accept": "text/plain"}
	text, err := r.api.Put(path, &request, &response, &headers)
	if err != nil {
		return fmt.Errorf("PUT %s request failed: %v\n", path, err)
	}
	log.Printf("SetParam response text: %s\n", text)
	log.Printf("SetParam response: %+v\n", response)
	return nil
}
