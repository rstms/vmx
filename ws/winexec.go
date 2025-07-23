package ws

import (
	"fmt"
	"log"
)

type WinExecClient struct {
	api     *APIClient
	debug   bool
	verbose bool
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

	url := ViperGetString("winexec.url")
	cert := ViperGetString("winexec.cert")
	key := ViperGetString("winexec.key")
	ca := ViperGetString("winexec.ca")

	api, err := NewAPIClient(url, cert, key, ca, nil)
	if err != nil {
		return nil, err
	}
	client := WinExecClient{
		api:     api,
		debug:   ViperGetBool("winexec.debug"),
		verbose: ViperGetBool("winexec.verbose"),
	}
	return &client, nil

}

func (w *WinExecClient) Spawn(command string, exitCode *int) error {
	if w.debug {
		log.Printf("winexec Spawn(%s)\n", command)
	}
	request := WinExecRequest{Command: command}
	var response WinExecResponse
	if w.verbose {
		log.Printf("winexec spawn request: %+v\n", request)
	}
	_, err := w.api.Post("/spawn/", &request, &response, nil)
	if err != nil {
		return err
	}
	if w.verbose {
		log.Printf("winexec spawn response: %+v\n", response)
	}
	if !response.Success {
		return fmt.Errorf("WinExec: spawn failed: %v", response)
	}
	if exitCode != nil {
		*exitCode = response.ExitCode
	} else if response.ExitCode != 0 {
		return fmt.Errorf("Spawned Process '%s' exited %d", command, response.ExitCode)
	}
	return nil
}

func (w *WinExecClient) Exec(command string, args []string, exitCode *int) (string, string, error) {
	if w.debug {
		log.Printf("winexec Exec(%s %v)\n", command, args)
	}
	request := WinExecRequest{Command: command, Args: args}
	var response WinExecResponse
	if w.verbose {
		log.Printf("winexec exec request: %+v\n", request)
	}
	_, err := w.api.Post("/exec/", &request, &response, nil)
	if err != nil {
		return "", "", err
	}
	if w.verbose {
		log.Printf("winexec exec response: %+v\n", response)
	}
	if !response.Success {
		return "", "", fmt.Errorf("WinExec: exec failed: %v", response)
	}
	if exitCode != nil {
		*exitCode = response.ExitCode
	} else if response.ExitCode != 0 {
		return "", "", fmt.Errorf("Process '%s' exited %d\n%s", command, response.ExitCode, response.Stderr)
	}
	return response.Stdout, response.Stderr, nil
}
