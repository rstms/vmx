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

	"github.com/spf13/cobra"
	"strings"
)

// vmrunCmd represents the vmrun command
var vmrunCmd = &cobra.Command{
	Use:   "vmrun",
	Short: "execute the vmrun command on the host",
	Long: `
Execute the vmrun utility on the configured VMWare Workstation host
Prefix command line arguments with '-T ws'
Use vxm vmrun /? for help
`,
	Args: cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		vmx = GetController()
		fields := []string{"vmrun"}
		if len(args) > 0 {
			if len(args) == 1 && (args[0] == "help" || args[0] == "/?") {
				fields = []string{"vmrun", "/?"}
			} else {
				fields = append([]string{"vmrun", "-T", "ws"}, args...)
			}
		}
		exitCode, olines, elines, err := vmx.Exec(strings.Join(fields, " "))
		cobra.CheckErr(err)
		fmt.Println(strings.Join(olines, "\n"))
		if len(elines) > 0 {
			log.Println(strings.Join(elines, "\n"))
		}
		ExitCode = &exitCode
	},
}

func init() {
	rootCmd.AddCommand(vmrunCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// vmrunCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// vmrunCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
