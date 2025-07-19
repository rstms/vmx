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
	"github.com/spf13/cobra"
)

var sendkeysCmd = &cobra.Command{
	Use: "sendkeys VID KEYS",
	Short: "send keystrokes to the instance",
	Long: `
Translate the KEYS argument into HID scan codes and send the result to the
selected instance using the vmcli utility on the host.

Quoting:
In bash, use single quotes around KEYS, backslash-escape double quotes, and
escape any single quotes using backslash-escaped hex (\x27).  Standard 
backslash escapes such as \n are decoded.

Examples:
vmx sendkeys testvm 'This has a \x27quoted\x27 elements\n'
vmx sendkeys testvm 'This has a \"double-quoted\" element\n'
vmx sendkeys testvm 'This has a `backquoted` element.\n'
vmx sendkeys testvm 'echo $PATH\n'
`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		vid := args[0]
		keys := args[1]
		InitController()
		err := vmx.SendKeys(vid, keys)
		cobra.CheckErr(err)
	},
}

func init() {
	rootCmd.AddCommand(sendkeysCmd)
}
