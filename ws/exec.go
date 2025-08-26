package ws

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

func (v *vmctl) LocalExec(command string, exitCode *int) ([]string, error) {
	if v.debug {
		log.Printf("LocalExec('%s', %v)\n", command, exitCode)
	}
	var shell string
	var args []string
	if v.Local == "windows" {
		shell = "cmd"
		args = []string{"/c", command}
	} else {
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/sh"
		}
		args = []string{"-c", command}
	}
	return v.exec(shell, args, "", exitCode)
}

func (v *vmctl) sshArgs() []string {
	return []string{"-q", "-i", v.KeyFile, v.Username + "@" + v.Hostname}
}

func (v *vmctl) RemoteExec(command string, exitCode *int) ([]string, error) {
	if v.debug {
		log.Printf("RemoteExec('%s', %v)\n", command, exitCode)
	}
	switch v.Shell {
	case "winexec":
		stdout, _, err := v.winexec.Exec("cmd", []string{"/c", command}, exitCode)
		if err != nil {
			return []string{}, Fatal(err)
		}
		return strings.Split(strings.TrimSpace(stdout), "\n"), nil
	case "ssh":
		args := v.sshArgs()
		if v.Remote == "windows" {
			args = append(args, command)
			command = ""
		}
		return v.exec(v.Shell, args, command, exitCode)
	case "sh":
		return v.exec(v.Shell, []string{}, command, exitCode)
	case "cmd":
		return v.exec(v.Shell, []string{"/c", command}, "", exitCode)
	}
	return []string{}, Fatalf("unexpected shell: %s", v.Shell)
}

func (v *vmctl) RemoteSpawn(command string, exitCode *int) error {
	if v.debug {
		log.Printf("RemoteSpawn('%s', %v)\n", command, exitCode)
	}
	switch v.Shell {
	case "winexec":
		return v.winexec.Spawn(command, exitCode)
	case "ssh":
		args := v.sshArgs()
		if v.Remote == "windows" {
			args = append(args, command)
			command = ""
		}
		_, err := v.exec(v.Shell, args, command, exitCode)
		return Fatal(err)
	case "sh":
		return v.spawn("/bin/sh", command, exitCode)
	case "cmd":
		return v.spawn("cmd", command, exitCode)
	}
	return Fatalf("unexpected shell: %s", v.Shell)
}

func (v *vmctl) spawn(shell, command string, exitCode *int) error {
	if v.debug {
		log.Printf("spawn('%s', %v)\n", command, exitCode)
	}
	stdin := ""
	args := []string{}
	if shell == "cmd" {
		args = []string{"/c", fmt.Sprintf("start /MIN %s", command)}
	} else {
		stdin = command + "&"
	}
	cmd := exec.Command(shell, args...)
	if len(stdin) > 0 {
		cmd.Stdin = bytes.NewBuffer([]byte(stdin + "\n"))
	} else {
		cmd.Stdin = nil
	}
	cmd.Stdout = nil
	cmd.Stderr = nil
	err := cmd.Run()
	switch e := err.(type) {
	case nil:
		if exitCode != nil {
			*exitCode = 0
		}
	case *exec.ExitError:
		if exitCode == nil {
			err = Fatalf("Process '%s' exited %d", cmd, e.ProcessState.ExitCode())
		} else {
			*exitCode = e.ProcessState.ExitCode()
			log.Printf("WARNING: process '%s' exited %d\n", cmd, *exitCode)
			err = nil
		}
	}
	return nil
}

// note: if exitCode is nil, exit != 0 is an error, otherwise the exit code will be set
func (v *vmctl) exec(command string, args []string, stdin string, exitCode *int) ([]string, error) {
	if v.debug {
		log.Printf("exec('%s', %v, '%s', %v)\n", command, args, stdin, exitCode)
	}
	olines := []string{}
	elines := []string{}
	cmd := exec.Command(command, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if len(stdin) > 0 {
		cmd.Stdin = bytes.NewBuffer([]byte(stdin + "\n"))
	}
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	estr := strings.TrimSpace(stderr.String())
	if estr != "" {
		elines = strings.Split(estr, "\n")
		for i, line := range elines {
			log.Printf("stderr[%d] %s\n", i, line)
		}
	}
	ostr := strings.TrimSpace(stdout.String())
	if ostr != "" {
		olines = strings.Split(ostr, "\n")
		if v.debug {
			for i, line := range olines {
				log.Printf("stdout[%d] %s\n", i, line)
			}
		}
	}

	switch e := err.(type) {
	case nil:
		if exitCode != nil {
			*exitCode = 0
		}
	case *exec.ExitError:
		if exitCode == nil {
			err = Fatalf("Process '%s' exited %d\n%s", cmd, e.ProcessState.ExitCode(), stderr.String())
		} else {
			*exitCode = e.ProcessState.ExitCode()
			log.Printf("WARNING: process '%s' exited %d\n%s", cmd, *exitCode, stderr.String())
			err = nil
		}
	}

	return olines, err
}
