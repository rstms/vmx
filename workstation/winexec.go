package workstation

import (
	"fmt"
	"github.com/spf13/viper"
	"log"
)

type WinExecClient struct {
	api     *APIClient
	verbose bool
	debug   bool
}

type WinExecRequest struct {
	Command string
	Args    []string
}

type WinExecResponse struct {
	Success  bool
	Command  string
	ExitCode int
	Stdout   string
	Stderr   string
}

func NewWinExecClient() (*WinExecClient, error) {
	url := viper.GetString("winexec_url")
	_, cert, err := GetViperPath("winexec_cert")
	if err != nil {
		return nil, err
	}
	_, key, err := GetViperPath("winexec_key")
	if err != nil {
		return nil, err
	}
	_, ca, err := GetViperPath("winexec_ca")
	if err != nil {
		return nil, err
	}
	api, err := NewAPIClient(url, cert, key, ca, nil)
	if err != nil {
		return nil, err
	}
	client := WinExecClient{
		api:     api,
		verbose: viper.GetBool("verbose"),
		debug:   viper.GetBool("debug"),
	}
	return &client, nil

}

func (w *WinExecClient) Spawn(command string) (int, error) {
	request := WinExecRequest{Command: command}
	var response WinExecResponse
	log.Printf("winexec request: %+v\n", request)
	_, err := w.api.Post("/spawn/", &request, &response, nil)
	if err != nil {
		return 0, err
	}
	log.Printf("winexec response: %+v\n", response)
	if !response.Success {
		return 0, fmt.Errorf("WinExec: spawn failed: %v", response)
	}
	return response.ExitCode, nil
}

func (w *WinExecClient) Exec(command string, args []string) (int, string, string, error) {
	request := WinExecRequest{Command: command, Args: args}
	var response WinExecResponse
	log.Printf("winexec request: %+v\n", request)
	_, err := w.api.Post("/exec/", &request, &response, nil)
	if err != nil {
		return 0, "", "", err
	}
	log.Printf("winexec response: %+v\n", response)
	if !response.Success {
		return 0, "", "", fmt.Errorf("WinExec: exec failed: %v", response)
	}
	return response.ExitCode, response.Stdout, response.Stderr, nil
}
