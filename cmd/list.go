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

var listCmd = &cobra.Command{
	Use:   "list [VID|PATH|iso]",
	Short: "List VM files",
	Long: `
List host files.  If PATH is an instance name the instance directory is 
selected.  If the first element in path is 'iso', the configured vmware_iso
path is used as the root.

The output is the host's default directory list format.  Use --long for a
long listing.
`,
	Aliases: []string{"ls"},
	Args:    cobra.RangeArgs(0, 1),
	Run: func(cmd *cobra.Command, args []string) {
		InitController()
		var vid string
		if len(args) > 0 {
			vid = args[0]
		}
		iso, err := ws.IsIsoPath(vid)
		cobra.CheckErr(err)
		options := ws.FilesOptions{
			Detail: ViperGetBool("long"),
			All:    ViperGetBool("all"),
			Iso:    iso,
		}
		lines, err := vmx.Files(vid, options)
		cobra.CheckErr(err)
		if OutputJSON && !options.Detail {
			fmt.Println(FormatJSON(lines))
		} else {
			for _, line := range lines {
				fmt.Println(line)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
