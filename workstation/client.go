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
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const VMREST_CONTENT_TYPE = "application/vnd.vmware.vmw.reset-v1+json"
const VMREST_PORT = 8697

type APIClient struct {
	Client   *http.Client
	URL      string
	Headers  map[string]string
	verbose  bool
	debug    bool
	vmxFiles map[string]string
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
		URL:      url,
		Headers:  make(map[string]string),
		verbose:  viper.GetBool("verbose"),
		debug:    viper.GetBool("debug"),
		vmxFiles: make(map[string]string),
	}

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

func (a *APIClient) Get(path string, response interface{}) (string, error) {
	return a.request("GET", path, nil, response, nil)
}

func (a *APIClient) Post(path string, request, response interface{}, headers *map[string]string) (string, error) {
	return a.request("POST", path, request, response, headers)
}

func (a *APIClient) Put(path string, response interface{}) (string, error) {
	return a.request("PUT", path, nil, response, nil)
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
	err = json.Unmarshal(body, responseData)
	if err != nil {
		return "", fmt.Errorf("failed decoding JSON response: %v", err)
	}
	if err != nil {
		return "", err
	}
	var text []byte
	text, err = json.MarshalIndent(responseData, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed formatting JSON response: %v", err)
	}
	return string(text), nil
}

// GET /vms, generate vmxFiles map, return slice of VIDS
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

	a.vmxFiles = make(map[string]string)
	for _, vid := range response {
		a.vmxFiles[vid.Id] = vid.Path
	}
	if a.verbose {
		log.Printf("GetVIDs returning: %+v\n", response)
	}
	return response, nil
}

// GET /vms/ID, add CpuCount, RamSizeMb to VM
func (a *APIClient) GetVM(vm *VM) error {

	vid := vm.Id
	path := fmt.Sprintf("vms/%s", vid)

	var response struct {
		cpu struct {
			processors int
		}
		id     string
		memory int
	}

	if a.verbose {
		log.Printf("GetVM(%s)\n", vid)
	}

	_, err := a.Get(path, &response)
	if err != nil {
		return fmt.Errorf("GET %s request failed: %v\n", path, err)
	}

	vm.CpuCount = response.cpu.processors
	vm.RamSizeMb = response.memory

	if a.verbose {
		log.Printf("GetVM(%s) returning: %+v\n", vid, *vm)
	}

	return nil
}

// call get/vms/ID/restrictions endpoint and add response data to VM
func (a *APIClient) GetVMRestrictions(vm *VM) error {

	vid := vm.Id
	path := fmt.Sprintf("vms/%s/restrictions", vid)
	/*
		var response struct {
			cpu struct {
				processors int
			}
			id     string
			memory int
		}
	*/

	var response map[string]any

	if a.verbose {
		log.Printf("GetVMRestrictions(%s)\n", vid)
	}

	text, err := a.Get(path, &response)
	if err != nil {
		return fmt.Errorf("GET %s request failed: %v\n", path, err)
	}

	log.Printf("text: %s\n", text)
	log.Printf("response: %+v\n", response)

	/*
		vm.CpuCount = response.cpu.processors
		vm.RamSizeMb = response.memory
	*/

	if a.verbose {
		log.Printf("GetVMRestrictions(%s) returning: %+v\n", vid, *vm)
	}

	return nil
}
