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

	"github.com/rstms/vmx/workstation"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os/user"
)

var cfgFile string
var vmxctl workstation.Controller

var rootCmd = &cobra.Command{
	Version: "0.0.5",
	Use:     "vmx",
	Short:   "control VMWare Workstation instances",
	Long: `
Control VMWare Workstation instances
`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		vmxctl = GetController()
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if vmxctl != nil {
			err := vmxctl.Close()
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
	hostname, err := os.Hostname()
	cobra.CheckErr(err)
	currentUser, err := user.Current()
	cobra.CheckErr(err)
	username := currentUser.Username
	OptionString("logfile", "", "", "log filename")
	OptionString("config", "c", "", "config file")
	OptionSwitch("debug", "d", "produce debug output")
	OptionSwitch("verbose", "v", "produce diagnostic output")
	OptionString("hostname", "H", hostname, "controller hostname")
	OptionString("username", "U", username, "controller username")
	OptionString("private-key", "i", "", "ssh private key file")
	OptionString("api-username", "", username, "VMREST API Username")
	OptionString("api-password", "", "", "VMREST API Password")
	OptionString("port", "", fmt.Sprintf("%d", workstation.VMREST_PORT), "VMREST API Port")
	OptionString("relay", "L", "", "ssh port forward VMREST port")
	OptionString("url", "", "", "VMREST API URL")
}

func GetController() workstation.Controller {
	c, err := workstation.NewController()
	cobra.CheckErr(err)
	if viper.GetBool("verbose") {
		log.Printf("Controller: %s\n", FormatJSON(c))
	}
	return c
}
