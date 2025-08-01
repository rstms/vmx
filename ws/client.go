package ws

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

type APIClient struct {
	Client  *http.Client
	URL     string
	Headers map[string]string
	verbose bool
	debug   bool
}

func NewAPIClient(url, certFile, keyFile, caFile string, headers *map[string]string) (*APIClient, error) {

	api := APIClient{
		URL:     url,
		Headers: make(map[string]string),
		verbose: ViperGetBool("api_client.verbose"),
		debug:   ViperGetBool("api_client.debug"),
	}

	if headers != nil {
		for k, v := range *headers {
			api.Headers[k] = v
		}
	}

	transport := http.Transport{
		IdleConnTimeout:   time.Duration(ViperGetInt64("idle_conn_timeout")) * time.Second,
		DisableKeepAlives: ViperGetBool("disable_keepalives"),
	}

	if certFile != "" || keyFile != "" || caFile != "" {
		tlsConfig := tls.Config{}
		if certFile == "" || keyFile == "" || caFile == "" {
			return nil, fmt.Errorf("incomplete TLS config: cert=%s key=%s ca=%s\n", certFile, keyFile, caFile)
		}

		cert, err := tls.LoadX509KeyPair(os.ExpandEnv(certFile), os.ExpandEnv(keyFile))
		if err != nil {
			return nil, fmt.Errorf("error loading client certificate pair: %v", err)
		}

		caCert, err := ioutil.ReadFile(os.ExpandEnv(caFile))
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
		transport.TLSClientConfig = &tlsConfig
	}

	api.Client = &http.Client{Transport: &transport}

	return &api, nil
}

func FormatJSON(v any) (string, error) {
	formatted, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed formatting JSON response: %v", err)
	}
	return string(formatted), nil
}

func LogJSON(label string, v any) {
	text, err := FormatJSON(v)
	if err != nil {
		log.Printf("LogJSON: %v", err)
		log.Printf("%s: %+v\n", label, v)
	} else {
		log.Printf("%s: %s\n", label, text)
	}
}

func (a *APIClient) Close() {
	a.Client.CloseIdleConnections()
	a.Client = nil
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

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var detail string
		if len(body) > 0 {
			detail = "\n" + string(body)
		}
		return "", fmt.Errorf("%s%s", response.Status, detail)
	}

	return text, nil
}
