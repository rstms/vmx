package ws

import (
	"log"
)

type WinExecClient struct {
	api   *APIClient
	debug bool
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

	url := ViperGetString("winexec.client.url")
	cert := ViperGetString("winexec.client.cert")
	key := ViperGetString("winexec.client.key")
	ca := ViperGetString("winexec.client.ca")
	clientDebug := ViperGetString("winexec.client.debug")

	if ViperGetBool("verbose") {
		log.Printf("NewWinExecClient: %s\n", FormatJSON(&map[string]any{
			"url":   url,
			"cert":  cert,
			"key":   key,
			"ca":    ca,
			"debug": clientDebug,
		}))
	}

	api, err := NewAPIClient(url, cert, key, ca, nil)
	if err != nil {
		return nil, Fatal(err)
	}
	client := WinExecClient{
		api:   api,
		debug: ViperGetBool("winexec.client.debug"),
	}

	return &client, nil

}

func (w *WinExecClient) Spawn(command string, exitCode *int) error {
	if w.debug {
		log.Printf("winexec Spawn(%s)\n", command)
	}
	request := WinExecRequest{Command: command}
	var response WinExecResponse
	if w.debug {
		log.Printf("winexec spawn request: %+v\n", request)
	}
	_, err := w.api.Post("/spawn/", &request, &response, nil)
	if err != nil {
		return Fatal(err)
	}
	if w.debug {
		log.Printf("winexec spawn response: %+v\n", response)
	}
	if !response.Success {
		return Fatalf("WinExec: spawn failed: %v", response)
	}
	if exitCode != nil {
		*exitCode = response.ExitCode
	} else if response.ExitCode != 0 {
		return Fatalf("Spawned Process '%s' exited %d", command, response.ExitCode)
	}
	return nil
}

func (w *WinExecClient) Exec(command string, args []string, exitCode *int) (string, string, error) {
	if w.debug {
		log.Printf("winexec Exec(%s %v)\n", command, args)
	}
	request := WinExecRequest{Command: command, Args: args}
	var response WinExecResponse
	if w.debug {
		log.Printf("winexec exec request: %+v\n", request)
	}
	_, err := w.api.Post("/exec/", &request, &response, nil)
	if err != nil {
		return "", "", Fatal(err)
	}
	if w.debug {
		log.Printf("winexec exec response: %+v\n", response)
	}
	if !response.Success {
		return "", "", Fatalf("WinExec: exec failed: %v", response)
	}
	if exitCode != nil {
		*exitCode = response.ExitCode
	} else if response.ExitCode != 0 {
		return "", "", Fatalf("Process '%s' exited %d\n%s", command, response.ExitCode, response.Stderr)
	}
	return response.Stdout, response.Stderr, nil
}
