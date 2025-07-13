/*
Copyright © 2025 Matt Krueger <mkrueger@rstms.net>
All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

 1. Redistributions of source code must retain the above copyright notice,
    this list of conditions and the following disclaimer.

 2. Redistributions in binary form must reproduce the above copyright notice,
    this list of conditions and the following disclaimer in the documentation
    and/or other materials provided with the distribution.

 3. Neither the name of the copyright holder nor the names of its contributors
    may be used to endorse or promote products derived from this software
    without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
POSSIBILITY OF SUCH DAMAGE.
*/
package cmd

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"
)

var editCmd = &cobra.Command{
	Use:   "edit VID",
	Short: "edit vmx file of the selected instance",
	Long: `
Download the VMX file of the seleceted instance and open it in the system
editor.  On save, upload the file to the host.

The instance must be in the 'poweredOff' state.

The original content of the file is saved in a backup file in the current
directory.
`,
	Run: func(cmd *cobra.Command, args []string) {
		InitController()
		vid := args[0]
		vm, err := vmx.Get(vid)
		cobra.CheckErr(err)

		// verify VM is powered off
		powerState, err := vmx.GetProperty(vm.Id, "power")
		cobra.CheckErr(err)
		if powerState != "poweredOff" {
			err := fmt.Errorf("cannot edit in power state: %s", powerState)
			cobra.CheckErr(err)
		}

		// read VMX data
		vmxData, err := vmx.GetProperty(vm.Id, "vmx")
		cobra.CheckErr(err)

		// write data to edit file
		vmxFile := fmt.Sprintf("%s.vmx", vm.Name)
		err = os.WriteFile(vmxFile, []byte(vmxData), 0600)
		cobra.CheckErr(err)

		// write data to backup file
		backupFile := backupFilename(vmxFile)
		err = os.WriteFile(backupFile, []byte(vmxData), 0600)
		cobra.CheckErr(err)

		// edit file
		var editCommand string
		if runtime.GOOS == "windows" {
			editCommand = "notepad"
		} else {
			editCommand = os.Getenv("VISUAL")
			if editCommand == "" {
				editCommand = os.Getenv("EDITOR")
				if editCommand == "" {
					editCommand = "vi"
				}
			}
		}
		editor := exec.Command(editCommand, vmxFile)
		log.Printf("editor: %s\n", editor)
		editor.Stdin = os.Stdin
		editor.Stdout = os.Stdout
		editor.Stderr = os.Stderr
		err = editor.Run()
		switch err.(type) {
		case *exec.ExitError:
			fmt.Fprintf(os.Stderr, "editor exited: %d, not uploading result\n", editor.ProcessState.ExitCode())
			return
		default:
			cobra.CheckErr(err)
		}
		// upload result to host
		editedData, err := os.ReadFile(vmxFile)
		cobra.CheckErr(err)
		if string(editedData) == string(vmxData) {
			fmt.Println("no changes")
		} else {
			err = vmx.SetProperty(vm.Id, "vmx", string(editedData))
			cobra.CheckErr(err)
		}
	},
}

func backupFilename(path string) string {
	filename := path + ".bak"
	count := 0
	for {
		if IsFile(filename) {
			count += 1
			filename = fmt.Sprintf("%s.bak.%d", path, count)
		} else {
			return filename
		}
	}
}

func init() {
	rootCmd.AddCommand(editCmd)
}
