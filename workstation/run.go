package workstation

import (
	"bytes"
	"fmt"
	"github.com/spf13/viper"
	"log"
	"os/exec"
	"strings"
)

func Run(shell, os, command string) ([]string, []string, error) {
	switch shell {
	case "ssh":
		return SSHExec(os, command)
	case "sh":
		return Exec("/bin/sh", []string{}, command)
	case "cmd":
		return Exec("cmd", []string{"/c", command}, "")
	}
	return []string{}, []string{}, fmt.Errorf("unexpected shell: %s", shell)
}

func SSHExec(os, command string) ([]string, []string, error) {
	username := viper.GetString("username")
	hostname := viper.GetString("hostname")
	_, keyPath, err := GetViperPath("private_key")
	if err != nil {
		return []string{}, []string{}, err
	}
	args := []string{"-q", "-i", keyPath, username + "@" + hostname}
	if os == "windows" {
		args = append(args, command)
		command = ""
	}
	return Exec("ssh", args, command)
}

func Exec(command string, args []string, stdin string) ([]string, []string, error) {
	debug := viper.GetBool("debug")
	cmd := exec.Command(command, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if len(stdin) > 0 {
		cmd.Stdin = bytes.NewBuffer([]byte(stdin + "\n"))
	}
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if debug {
		log.Printf("cmd: %+v", cmd)
		if len(stdin) > 0 {
			log.Printf("stdin: '%s'\n", stdin)
		}
	}
	err := cmd.Run()
	if err != nil {
		return []string{}, []string{}, err
	}
	olines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if viper.GetBool("debug") {
		for i, line := range olines {
			log.Printf("stdout[%d] %s\n", i, line)
		}
	}
	elines := strings.Split(strings.TrimSpace(stderr.String()), "\n")
	if viper.GetBool("debug") {
		for i, line := range elines {
			log.Printf("stderr[%d] %s\n", i, line)
		}
	}
	return olines, elines, nil
}
