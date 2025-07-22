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

	"github.com/rstms/vmx/ws"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string
var ExitCode *int

var OutputJSON bool
var OutputText bool

const (
	OutputFormatText = iota
	OutputFormatJSON
)

var vmx ws.Controller

var rootCmd = &cobra.Command{
	Version: "0.0.24",
	Use:     "vmx",
	Short:   "control VMWare Workstation instances",
	Long: `
Control VMWare Workstation instances
`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		OutputJSON = true
		OutputText = false
		if ViperGetBool("text") {
			OutputText = true
			OutputJSON = false
		}
		if ViperGetBool("no_wait") {
			ViperSet("wait", false)
		} else {
			ViperSet("wait", true)
		}
	},
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
	OptionString(rootCmd, "config", "c", "", "config file")
	OptionString(rootCmd, "logfile", "", "", "log filename")
	OptionSwitch(rootCmd, "debug", "d", "produce debug output")
	OptionSwitch(rootCmd, "verbose", "v", "produce diagnostic output")
	OptionString(rootCmd, "timeout", "t", "60", "wait timeout in seconds")
	OptionString(rootCmd, "interval", "i", "1", "wait query interval in seconds")
	OptionSwitch(rootCmd, "json", "", "format output as JSON (default)")
	OptionSwitch(rootCmd, "text", "", "format output as text")

	OptionString(rootCmd, "shell", "", "ssh", "remote shell")
	OptionSwitch(rootCmd, "all", "a", "select all items")
	OptionSwitch(rootCmd, "long", "l", "add output detail")

	OptionSwitch(rootCmd, "no-humanize", "n", "display sizes in bytes")
	OptionSwitch(rootCmd, "no-wait", "W", "do not wait for expected powerState after start/stop/kill")

	OptionString(rootCmd, "iso", "", "", "CD/DVD ISO boot file or URL")
	OptionString(rootCmd, "iso-ca", "", "", "CA for ISO URL download")
	OptionString(rootCmd, "iso-cert", "", "", "client certificate for ISO URL download")
	OptionString(rootCmd, "iso-key", "", "", "client cert key for ISO URL download")
	OptionSwitch(rootCmd, "iso-detach", "", "set CD/DVD detached at boot")
	OptionSwitch(rootCmd, "iso-disable", "", "remove the CD/DVD ISO device")
}

func InitController() {
	// copy selected cli config to vmx section
	viper.Set("vmx.timeout", ViperGetInt("timeout"))
	viper.Set("vmx.interval", ViperGetInt("interval"))
	viper.Set("vmx.verbose", ViperGetBool("verbose"))
	viper.Set("vmx.debug", ViperGetBool("debug"))
	c, err := ws.NewController()
	cobra.CheckErr(err)
	if ViperGetBool("verbose") {
		log.Printf("Controller: %s\n", FormatJSON(c))
	}
	vmx = c
}

func OutputInstanceState(vid, result string) {
	state, err := vmx.GetStatus(vid)
	cobra.CheckErr(err)
	state.Result = result
	fmt.Println(FormatJSON(&state))
}

func InitIsoOptions() (*ws.IsoOptions, error) {

	options := ws.IsoOptions{}
	iso := ViperGetString("iso")
	disable := ViperGetBool("iso_disable")
	if (iso != "") && disable {
		return nil, fmt.Errorf("conflict: iso/iso-disable")
	}
	switch {
	case iso != "":
		options.ModifyISO = true
		options.IsoPresent = true
		options.IsoFile = iso
		options.IsoBootConnected = true
		options.IsoCA = ViperGetString("iso_ca")
		options.IsoClientCert = ViperGetString("iso_cert")
		options.IsoClientKey = ViperGetString("iso_key")
	case disable:
		options.ModifyISO = true
		options.IsoPresent = false
	}
	if ViperGetBool("iso_detach") {
		options.ModifyISO = true
		options.IsoBootConnected = false
	}
	return &options, nil
}
