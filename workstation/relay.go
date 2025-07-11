package workstation

import (
	"bufio"
	"fmt"
	"github.com/spf13/viper"
	"log"
	"net"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const LISTEN_TIMEOUT = 5

type Relay struct {
	cmd     *exec.Cmd
	verbose bool
	debug   bool
	wg      sync.WaitGroup
}

func isPortOpen(host string, port int) bool {
	address := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.Dial("tcp", address)
	if err == nil {
		conn.Close()
		return true
	}
	return false
}

func waitListener(host string, port int) error {
	start := time.Now()
	timeout := LISTEN_TIMEOUT * time.Second
	for {
		if isPortOpen(host, port) {
			return nil
		}
		if time.Since(start) > timeout {
			return fmt.Errorf("timeout waiting for LISTEN port %d", port)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func NewRelay(forward string) (*Relay, error) {
	debug := viper.GetBool("debug")
	verbose := viper.GetBool("debug")
	username := viper.GetString("username")
	hostname := viper.GetString("hostname")
	_, keyPath, err := GetViperPath("private_key")
	if err != nil {
		return nil, err
	}
	args := []string{}

	if debug {
		args = append(args, "-v")
	} else {
		args = append(args, "-q")
	}

	args = append(args, []string{
		"-N",
		"-o", "ExitOnForwardFailure=yes",
		"-L", forward,
		"-i", keyPath,
		username + "@" + hostname,
	}...)

	r := Relay{
		verbose: verbose,
		debug:   debug,
	}
	r.cmd = exec.Command("ssh", args...)

	stderr, err := r.cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed opening stderr pipe: %v", err)
	}
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		if verbose {
			log.Println("ssh_relay: stderr reader started")
			defer log.Println("ssh_relay: stderr reader exited")
		}
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			if debug {
				log.Printf("ssh_relay: %s\n", line)
			}
		}
		err := scanner.Err()
		if err != nil {
			log.Printf("ssh_relay: stderr reader failed: %v", err)
		}
	}()
	if debug {
		log.Printf("ssh_relay: command: %+v\n", r.cmd)
	}

	err = r.cmd.Start()
	if err != nil {
		return nil, err
	}

	if debug {
		log.Printf("ssh_relay: started process: %+v\n", r.cmd.Process)
	}

	field, _, ok := strings.Cut(forward, ":")
	if !ok {
		return nil, fmt.Errorf("failed parsing port from: %s", forward)
	}
	port, err := strconv.Atoi(field)
	if err != nil {
		return nil, fmt.Errorf("failed int conversion: %s", field)
	}

	if debug {
		log.Println("ssh_relay: awaiting LISTEN...")
	}
	err = waitListener("localhost", port)
	if err != nil {
		return nil, err
	}
	if verbose {
		log.Printf("ssh_relay: started process %d forwarding %s", r.cmd.Process.Pid, forward)
	}
	return &r, nil
}

func (r *Relay) Close() error {
	if r.verbose {
		log.Printf("ssh_relay: stopping process %d\n", r.cmd.Process.Pid)
	}
	if runtime.GOOS == "windows" {
		err := r.cmd.Process.Kill()
		if err != nil {
			return err
		}
	} else {
		err := r.cmd.Process.Signal(syscall.SIGTERM)
		if err != nil {
			return err
		}
	}
	if r.verbose {
		log.Println("ssh_relay: awaiting stderr reader...")
	}
	r.wg.Wait()
	if r.verbose {
		log.Println("ssh_relay: awaiting process...")
	}
	_, err := r.cmd.Process.Wait()
	if err != nil {
		return err
	}
	if r.verbose {
		log.Println("ssh_relay: shutdown complete")
	}
	return nil
}
