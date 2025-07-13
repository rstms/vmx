package workstation

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const VMREST_CONTENT_TYPE = "application/vnd.vmware.vmw.rest-v1+json"
const VMREST_PORT = 8697

type APIClient struct {
	Client  *http.Client
	URL     string
	Headers map[string]string
	verbose bool
	debug   bool

	ByPath map[string]VID
	ByName map[string]VID
	ById   map[string]VID
}

func GetViperPath(key string) (bool, string, error) {
	path := viper.GetString(key)
	if path == "" {
		return false, "", nil
	}
	if len(path) < 2 {
		return false, "", fmt.Errorf("path %s too short: %s", key, path)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return false, "", err
	}
	if strings.HasPrefix(path, "~") {
		path = filepath.Join(home, path[1:])
	}
	return true, path, nil

}

func newVMRestClient() (*APIClient, error) {

	viper.SetDefault("disable_keepalives", true)
	viper.SetDefault("idle_conn_timeout", 5)
	viper.SetDefault("port", VMREST_PORT)

	url := viper.GetString("url")
	if url == "" {
		url = fmt.Sprintf("http://%s:%d/api/", viper.GetString("api_hostname"), viper.GetInt("port"))
	}

	api := APIClient{
		URL:     url,
		Headers: make(map[string]string),
		verbose: viper.GetBool("verbose"),
		debug:   viper.GetBool("debug"),
	}
	api.resetIndex()

	api.Headers["Content-Type"] = VMREST_CONTENT_TYPE
	api.Headers["Accept"] = VMREST_CONTENT_TYPE

	username := viper.GetString("api_username")
	if username == "" {
		username = viper.GetString("username")
	}
	password := viper.GetString("api_password")
	api.Headers["Authorization"] = "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))

	hasCert, certFile, err := GetViperPath("cert")
	if err != nil {
		return nil, err
	}
	hasKey, keyFile, err := GetViperPath("key")
	if err != nil {
		return nil, err
	}
	hasCA, caFile, err := GetViperPath("ca")
	if err != nil {
		return nil, err
	}

	if hasCert || hasKey || hasCA {
		if !(hasCert && hasKey && hasCA) {
			return nil, fmt.Errorf("incomplete TLS config: cert=%s key=%s ca=%s\n", certFile, keyFile, caFile)
		}
	}

	tlsConfig := tls.Config{}
	if hasCert {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("error loading client certificate pair: %v", err)
		}

		caCert, err := ioutil.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("error loading certificate authority file: %v", err)
		}

		caCertPool, err := x509.SystemCertPool()
		if err != nil {
			return nil, fmt.Errorf("error opening system certificate pool: %v", err)
		}
		caCertPool.AppendCertsFromPEM(caCert)
		tlsConfig.Certificates = []tls.Certificate{cert}
		tlsConfig.RootCAs = caCertPool
	}

	api.Client = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig:   &tlsConfig,
			IdleConnTimeout:   time.Duration(viper.GetInt64("idle_conn_timeout")) * time.Second,
			DisableKeepAlives: viper.GetBool("disable_keepalives"),
		},
	}

	return &api, nil
}

func FormatJSON(v any) (string, error) {
	formatted, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed formatting JSON response: %v", err)
	}
	return string(formatted), nil
}

func logJSON(label string, v any) {
	text, err := FormatJSON(v)
	if err != nil {
		log.Printf("logJSON: %v", err)
		log.Printf("%s: %+v\n", label, v)
	} else {
		log.Printf("%s: %s\n", label, text)
	}
}

func (a *APIClient) resetIndex() {
	a.ByPath = make(map[string]VID)
	a.ById = make(map[string]VID)
	a.ByName = make(map[string]VID)
}

func (a *APIClient) Get(path string, response interface{}) (string, error) {
	return a.request("GET", path, nil, response, nil)
}

func (a *APIClient) Post(path string, request, response interface{}, headers *map[string]string) (string, error) {
	return a.request("POST", path, request, response, headers)
}

func (a *APIClient) Put(path string, request, response interface{}, headers *map[string]string) (string, error) {
	return a.request("PUT", path, request, response, headers)
}

func (a *APIClient) Delete(path string, response interface{}) (string, error) {
	return a.request("DELETE", path, nil, response, nil)
}

func (a *APIClient) request(method, path string, requestData, responseData interface{}, headers *map[string]string) (string, error) {
	var requestBytes []byte
	var err error
	switch requestData.(type) {
	case nil:
	case *[]byte:
		requestBytes = *(requestData.(*[]byte))
	default:
		requestBytes, err = json.Marshal(requestData)
		if err != nil {
			return "", fmt.Errorf("failed marshalling JSON body for %s request: %v", method, err)
		}
	}

	request, err := http.NewRequest(method, a.URL+path, bytes.NewBuffer(requestBytes))
	if err != nil {
		return "", fmt.Errorf("failed creating %s request: %v", method, err)
	}

	// add the headers set up at instance init
	for key, value := range a.Headers {
		request.Header.Add(key, value)
	}

	if headers != nil {
		// add the headers passed in to this request
		for key, value := range *headers {
			request.Header.Add(key, value)
		}
	}

	if a.verbose {
		log.Printf("<-- %s %s (%d bytes)", method, a.URL+path, len(requestBytes))
		if a.debug {
			log.Println("BEGIN-REQUEST-HEADER")
			for key, value := range request.Header {
				log.Printf("%s: %s\n", key, value)
			}
			log.Println("END-REQUEST-HEADER")
			log.Println("BEGIN-REQUEST-BODY")
			log.Println(string(requestBytes))
			log.Println("END-REQUEST-BODY")
		}
	}

	response, err := a.Client.Do(request)
	if err != nil {
		return "", fmt.Errorf("request failed: %v", err)
	}
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("failure reading response body: %v", err)
	}
	if a.verbose {
		log.Printf("--> '%s' (%d bytes)\n", response.Status, len(body))
		if a.debug {
			log.Println("BEGIN-RESPONSE-BODY")
			log.Println(string(body))
			log.Println("END-RESPONSE-BODY")
		}
	}
	var text string
	if len(body) > 0 {
		err = json.Unmarshal(body, responseData)
		if err != nil {
			return "", fmt.Errorf("failed decoding JSON response: %v", err)
		}
		t, err := FormatJSON(responseData)
		if err != nil {
			return "", err
		}
		text = t
	}
	return text, nil
}

// GET /vms, generate index maps, return slice of VIDS
func (a *APIClient) GetVIDs() ([]VID, error) {
	path := "vms"
	var response []VID

	if a.verbose {
		log.Println("GetVIDs")
	}

	_, err := a.Get(path, &response)
	if err != nil {
		return []VID{}, fmt.Errorf("GET %s request failed: %v\n", path, err)
	}

	a.resetIndex()
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
		a.ById[vid.Id] = vid
		a.ByName[vid.Name] = vid
		a.ByPath[vid.Path] = vid
		vids[i] = vid
	}
	if a.verbose {
		logJSON("GetVIDs returning: ", &vids)
	}
	return vids, nil
}

// search for a VM by Name or Id
func (a *APIClient) IsVM(vid string) (bool, error) {
	if len(a.ById) == 0 {
		// refresh ID index
		_, err := a.GetVIDs()
		if err != nil {
			return false, err
		}
	}

	_, ok := a.ById[vid]
	if ok {
		// vid is a valid VM ID
		return true, nil
	}

	_, ok = a.ByName[vid]
	if ok {
		// vid is a valid VM Name
		return true, nil
	}
	return false, nil
}

// return VM ID by Name or ID; error if neither is found
func (a *APIClient) GetId(vid string) (string, error) {
	ok, err := a.IsVM(vid)
	if err != nil {
		return "", err
	}
	if ok {
		_, ok = a.ById[vid]
		if ok {
			// vid is a valid ID
			return vid, nil
		}

		v, ok := a.ByName[vid]
		if ok {
			// vid is a valid name, return ID
			return v.Id, nil
		}
		return "", fmt.Errorf("IsVM(%s) is true, but vid not in ById or ByName", vid)
	}
	return "", fmt.Errorf("VM not found: %s", vid)
}

func (a *APIClient) GetVM(vid string) (VM, error) {
	id, err := a.GetId(vid)
	if err != nil {
		return VM{}, err
	}
	v, ok := a.ById[id]
	if !ok {
		return VM{}, fmt.Errorf("ByID index failed: vid=%s, id=%s", vid, id)
	}
	vm := VM{Name: v.Name, Id: v.Id, Path: v.Path}
	return vm, nil
}

// GET /vms/ID, add CpuCount, RamSizeMb to VM
func (a *APIClient) GetVMCpuRam(vm *VM) error {

	path := fmt.Sprintf("vms/%s", vm.Id)

	var response VmRestGetVmCpuRamResponse

	if a.verbose {
		log.Printf("GetVMCpuRam: %s\n", path)
	}

	_, err := a.Get(path, &response)
	if err != nil {
		return fmt.Errorf("GET %s request failed: %v\n", path, err)
	}

	if a.verbose {
		logJSON("VmRestGetVmCpuRam", &response)
	}

	vm.CpuCount = response.Cpu.Processors
	vm.RamSizeMb = response.Memory

	if a.verbose {
		logJSON("GetVMCpuRam returning", vm)
	}

	return nil
}

// populate VM with configuration
func (a *APIClient) GetConfig(vm *VM) error {

	if a.verbose {
		log.Printf("GetConfig(%s)\n", vm.Id)
	}

	path := fmt.Sprintf("vms/%s/restrictions", vm.Id)

	var response VmRestGetVmRestrictionsResponse
	_, err := a.Get(path, &response)
	if err != nil {
		return fmt.Errorf("GET %s request failed: %v\n", path, err)
	}

	if a.verbose {
		logJSON("VmRestGetVmRestrictionsResponse", &response)
	}

	if vm.Id != response.ID {
		return fmt.Errorf("response ID mismatch: expected=%s response=%s", vm.Id, response.ID)
	}
	vm.CpuCount = response.Cpu.Processors
	vm.RamSizeMb = response.Memory
	vm.IsoPath = ""
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
					vm.IsoPath = path
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
	vm.SerialPath = ""
	if len(response.SerialPortList.Devices) > 0 {
		captured := false
		for _, port := range response.SerialPortList.Devices {
			if !captured {
				vm.SerialAttached = true
				vm.SerialPath = ""
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

	if a.verbose {
		logJSON("GetConfig returning", &vm)
	}
	return nil
}

func (a *APIClient) GetState(vm *VM) error {
	err := a.GetPowerState(vm)
	if err != nil {
		return err
	}
	return nil
}

func (a *APIClient) GetPowerState(vm *VM) error {
	var response VmRestGetPowerStateResponse
	path := fmt.Sprintf("vms/%s/power", vm.Id)
	_, err := a.Get(path, &response)
	if err != nil {
		return fmt.Errorf("GET %s request failed: %v\n", path, err)
	}
	if a.verbose {
		logJSON("VmRestGetPowerStateResponse", &response)
	}
	vm.PowerState = response.PowerState
	vm.Running = vm.PowerState == "poweredOn"
	return nil
}

func (a *APIClient) GetParam(vm *VM, name string) (string, error) {
	var response map[string]any
	path := fmt.Sprintf("vms/%s/params/%s", vm.Id, url.PathEscape(name))
	text, err := a.Get(path, &response)
	if err != nil {
		return "", fmt.Errorf("GET %s request failed: %v\n", path, err)
	}
	log.Printf("text=%s\n", text)
	log.Printf("response=%+v\n", response)
	return text, nil
}

func (a *APIClient) SetParam(vm *VM, name, value string) error {
	requestString := fmt.Sprintf("{\"name\":\"%s\",\"value\": \"%s\"}", name, value)
	log.Printf("requestString: %s\n", requestString)
	request := []byte(requestString)
	var response map[string]any
	path := fmt.Sprintf("vms/%s/params", vm.Id)
	headers := map[string]string{"Accept": "text/plain"}
	text, err := a.Put(path, &request, &response, &headers)
	if err != nil {
		return fmt.Errorf("PUT %s request failed: %v\n", path, err)
	}
	log.Printf("text=%s\n", text)
	log.Printf("response=%+v\n", response)
	return nil
}
