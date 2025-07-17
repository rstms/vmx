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
	"log"
	"os"
	"os/user"

	"github.com/rstms/vmx/workstation"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string
var ExitCode *int

var vmx workstation.Controller

var rootCmd = &cobra.Command{
	Version: "0.0.10",
	Use:     "vmx",
	Short:   "control VMWare Workstation instances",
	Long: `
Control VMWare Workstation instances
`,
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if vmx != nil {
			err := vmx.Close()
			cobra.CheckErr(err)
		}
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
func init() {
	cobra.OnInitialize(InitConfig)
	OptionString(rootCmd, "logfile", "", "", "log filename")
	OptionString(rootCmd, "config", "c", "", "config file")
	OptionSwitch(rootCmd, "debug", "d", "produce debug output")
	OptionSwitch(rootCmd, "verbose", "v", "produce diagnostic output")
	OptionSwitch(rootCmd, "no-humanize", "n", "display sizes in bytes")
	OptionSwitch(rootCmd, "text", "", "format output as text if command is capable")
	OptionSwitch(rootCmd, "json", "", "format output as JSON (default on most commands)")
	hostname, err := os.Hostname()
	cobra.CheckErr(err)
	OptionString(rootCmd, "host", "", hostname, "workstation hostname")
	user, err := user.Current()
	cobra.CheckErr(err)
	OptionString(rootCmd, "user", "", user.Username, "workstation user")
	OptionString(rootCmd, "shell", "", "ssh", "remote shell")
	OptionSwitch(rootCmd, "all", "a", "select all items")
	OptionSwitch(rootCmd, "long", "l", "add output detail")
}
func InitController() {
	c, err := workstation.NewController()
	cobra.CheckErr(err)
	if viper.GetBool("verbose") {
		log.Printf("Controller: %s\n", FormatJSON(c))
	}
	vmx = c
}
