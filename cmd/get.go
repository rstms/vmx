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
	"strings"

	"github.com/spf13/cobra"
)

var getCmd = &cobra.Command{
	Use:   "get VM [PROPERTY] [PREFIX]",
	Short: "get instance detail",
	Long: `
By default, write the VM instance config and state as JSON.  A PROPERTY may
be specified to select other output values.

A PROPERTY can be a VM Object property key, a VMX config value, or one of the
following labels:

config ---- instance configuration 
state ----- instate state
power ----- power state
ip -------- IP address
vmx ------- vmx file content

PREFIX may be provided with the 'vmx' property to filter the vmx file output
`,
	Args: cobra.RangeArgs(1, 3),
	Run: func(cmd *cobra.Command, args []string) {
		InitController()
		vid := args[0]
		var property string
		var prefix string
		if len(args) > 1 {
			property = args[1]
		}
		if strings.ToLower(property) == "vmx" {
			property = "vmx"
			if len(args) > 2 {
				prefix = strings.ToLower(args[2])
			}
		}
		value, err := vmx.GetProperty(vid, property)
		cobra.CheckErr(err)
		if prefix != "" {
			for _, line := range strings.Split(value, "\n") {
				if strings.HasPrefix(strings.ToLower(line), prefix) {
					fmt.Println(line)
				}
			}
		} else {
			fmt.Println(value)
		}
	},
}

func init() {
	rootCmd.AddCommand(getCmd)
}
