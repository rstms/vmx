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
	"os/user"

	"github.com/rstms/vmx/workstation"
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

var vmx workstation.Controller

var rootCmd = &cobra.Command{
	Version: "0.0.22",
	Use:     "vmx",
	Short:   "control VMWare Workstation instances",
	Long: `
Control VMWare Workstation instances
`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		OutputJSON = true
		OutputText = false
		if viper.GetBool("text") {
			OutputText = true
			OutputJSON = false
		}
		if viper.GetBool("no_wait") {
			viper.Set("wait", false)
		} else {
			viper.Set("wait", true)
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
	OptionString(rootCmd, "logfile", "", "", "log filename")
	OptionString(rootCmd, "config", "c", "", "config file")
	OptionSwitch(rootCmd, "debug", "d", "produce debug output")
	OptionSwitch(rootCmd, "verbose", "v", "produce diagnostic output")
	OptionString(rootCmd, "timeout", "t", "60", "wait timeout in seconds")
	OptionString(rootCmd, "interval", "i", "1", "wait query interval in seconds")
	OptionSwitch(rootCmd, "no-humanize", "n", "display sizes in bytes")
	OptionSwitch(rootCmd, "json", "", "format output as JSON")
	OptionSwitch(rootCmd, "text", "", "format output as text")
	hostname, err := os.Hostname()
	cobra.CheckErr(err)
	OptionString(rootCmd, "host", "", hostname, "workstation hostname")
	user, err := user.Current()
	cobra.CheckErr(err)
	OptionString(rootCmd, "user", "", user.Username, "workstation user")
	OptionString(rootCmd, "shell", "", "ssh", "remote shell")
	OptionSwitch(rootCmd, "all", "a", "select all items")
	OptionSwitch(rootCmd, "long", "l", "add output detail")
	OptionSwitch(rootCmd, "no-wait", "W", "do not wait for expected powerState after start/stop/kill")
	OptionSwitch(rootCmd, "wait", "w", "wait for expected powerState after start/stop/kill")

	OptionString(rootCmd, "iso", "", "", "CD/DVD ISO boot file or URL")
	OptionString(rootCmd, "iso-ca", "", "", "CA for ISO URL download")
	OptionString(rootCmd, "iso-cert", "", "", "client certificate for ISO URL download")
	OptionString(rootCmd, "iso-key", "", "", "client cert key for ISO URL download")
	OptionSwitch(rootCmd, "iso-detach", "", "set CD/DVD detached at boot")
	OptionSwitch(rootCmd, "iso-disable", "", "remove the CD/DVD ISO device")
}

func InitController() {
	c, err := workstation.NewController()
	cobra.CheckErr(err)
	if viper.GetBool("verbose") {
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

func InitIsoOptions() (*workstation.IsoOptions, error) {

	options := workstation.IsoOptions{}
	iso := viper.GetString("iso")
	disable := viper.GetBool("iso_disable")
	if (iso != "") && disable {
		return nil, fmt.Errorf("conflict: iso/iso-disable")
	}
	switch {
	case iso != "":
		options.ModifyISO = true
		options.IsoPresent = true
		options.IsoFile = iso
		options.IsoBootConnected = true
		options.IsoCA = viper.GetString("iso_ca")
		options.IsoClientCert = viper.GetString("iso_cert")
		options.IsoClientKey = viper.GetString("iso_key")
	case disable:
		options.ModifyISO = true
		options.IsoPresent = false
	}
	if viper.GetBool("iso_detach") {
		options.ModifyISO = true
		options.IsoBootConnected = false
	}
	return &options, nil
}
