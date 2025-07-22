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

	"github.com/rstms/vmx/ws"
	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show [VID]",
	Short: "Display VM instances",
	Long: `
Display VM instance data
`,
	Aliases: []string{"ps"},
	Args:    cobra.RangeArgs(0, 1),
	Run: func(cmd *cobra.Command, args []string) {
		InitController()
		vid := ""
		if len(args) > 0 {
			vid = args[0]
		}
		options := ws.ShowOptions{
			Detail:  ViperGetBool("long"),
			Running: !ViperGetBool("all"),
		}
		vms, err := vmx.Show(vid, options)
		cobra.CheckErr(err)
		if options.Detail {
			fmt.Println(FormatJSON(vms))
		} else {
			names := make([]string, len(vms))
			for i, vm := range vms {
				if OutputJSON {
					names[i] = vm.Name
				} else {
					fmt.Println(vm.Name)
				}
			}
			if OutputJSON {
				fmt.Println(FormatJSON(names))
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(showCmd)
}
