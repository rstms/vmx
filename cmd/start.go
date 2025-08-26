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
	"github.com/rstms/vmx/ws"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:     "start VID",
	Aliases: []string{"restart"},
	Short:   "start a VM instance",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		InitController()
		vid := args[0]

		if cmd.CalledAs() == "restart" {
			options := ws.StopOptions{
				Wait: true,
			}
			_, err := vmx.Stop(vid, options)
			cobra.CheckErr(err)
		}

		options := ws.StartOptions{
			Background: ViperGetBool("background"),
			FullScreen: ViperGetBool("fullscreen"),
			Wait:       ViperGetBool("wait"),
		}

		if ViperGetBool("stretch") {
			options.ModifyStretch = true
			options.StretchEnabled = true
		}

		if ViperGetBool("no_stretch") {
			options.ModifyStretch = true
			options.StretchEnabled = false
		}

		isoOptions, err := InitIsoOptions()
		cobra.CheckErr(err)

		result, err := vmx.Start(vid, options, *isoOptions)
		cobra.CheckErr(err)
		if OutputJSON && ViperGetBool("status") {
			OutputInstanceState(vid, result)
		}
	},
}

func init() {
	CobraAddCommand(rootCmd, rootCmd, startCmd)
	OptionSwitch(startCmd, "stretch", "", "enable stretched display")
	OptionSwitch(startCmd, "no-stretch", "", "disable stretched display")
	OptionSwitch(startCmd, "background", "", "start in background mode")
	OptionSwitch(startCmd, "fullscreen", "", "start in full-screen mode")
}
