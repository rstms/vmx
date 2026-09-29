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
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/rstms/vmx/controller"
	"github.com/spf13/cobra"
)

var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "delete all vm instance files",
	Long: `
The destroy command is used to irrecoverably delete all vm instance files on
the host system.  Use this command with caution; it assumes you know what you
are asking for.
`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		InitController()
		vid := args[0]
		vm, err := vmx.Get(vid)
		cobra.CheckErr(err)
		if confirm(fmt.Sprintf("Confirm IRRECOVERABLE DESTRUCTION of VM instance '%s'", vm.Name)) {
			options := controller.DestroyOptions{
				Force: ViperGetBool("destroy.kill"),
			}
			err := vmx.Destroy(vm.Id, options)
			cobra.CheckErr(err)

			if OutputJSON && ViperGetBool("status") {
				// we can't call OutputInstanceState, so build the status here
				status := controller.VMState{Name: vm.Name, Id: vm.Id, Result: "vm_destroyed"}
				fmt.Println(FormatJSON(&status))
			}
		}
	},
}

func confirm(prompt string) bool {
	if ViperGetBool("destroy.force") {
		return true
	}
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("%s [y/N]: ", prompt)
		response, err := reader.ReadString('\n')
		cobra.CheckErr(err)
		response = strings.ToLower(strings.TrimSpace(response))
		if response == "y" || response == "yes" {
			return true
		} else if response == "n" || response == "no" || response == "" {
			fmt.Println("Cowardly refused.")
			return false
		}
	}
}

func init() {
	CobraAddCommand(rootCmd, rootCmd, destroyCmd)
	OptionSwitch(destroyCmd, "force", "", "suppress confirmation prompt")
	OptionSwitch(destroyCmd, "kill", "", "destroy running instance")
}
