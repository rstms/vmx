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

	"github.com/rstms/vmx/workstation"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var modifyCmd = &cobra.Command{
	Use:   "modify nic VID",
	Short: "modify instance configuration properties",
	Long: `

vnc modify [FLAGS] VID

Change NIC, ISO, TTY, VNC, and EFI instance configuration parameters.
The instance must be in in the 'poweredOff' state.

See the flags and options help for descriptions of the available settings.
Changes can be specified for multiple categories in a single command.
`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		InitController()
		vm, err := vmx.Get(args[0])
		cobra.CheckErr(err)

		options := workstation.CreateOptions{}

		ethEnable := viper.GetBool("eth_enable")
		ethAuto := viper.GetBool("eth_auto")
		if ethAuto {
			ethEnable = true
		}
		ethDisable := viper.GetBool("eth_disable")
		ethAddress := viper.GetString("eth_mac")
		if ethAddress != "" {
			ethEnable = true
		}
		if ethAuto && ethAddress != "" {
			err := fmt.Errorf("conflict: eth_auto/eth_mac")
			cobra.CheckErr(err)
		}
		if ethEnable && ethDisable {
			err := fmt.Errorf("conflict: eth_enable/eth_disable")
			cobra.CheckErr(err)
		}
		switch {
		case ethEnable:
			options.ModifyNIC = true
			options.MacAddress = ethAddress
			if options.MacAddress == "" {
				options.MacAddress = "auto"
			}
		case ethDisable:
			options.ModifyNIC = true
		}

		isoAttached := viper.GetString("iso_attach")
		isoDetached := viper.GetString("iso_detach")
		isoDisable := viper.GetBool("iso_disable")
		if isoAttached != "" && isoDetached != "" {
			err := fmt.Errorf("conflict: iso/iso-detached")
			cobra.CheckErr(err)
		}
		if (isoAttached != "" || isoDetached != "") && isoDisable {
			err := fmt.Errorf("conflict: iso enable/disable")
			cobra.CheckErr(err)
		}

		switch {
		case isoAttached != "":
			options.ModifyISO = true
			options.IsoPresent = true
			options.IsoFile = isoAttached
			options.IsoBootConnected = true
		case isoDetached != "":
			options.ModifyISO = true
			options.IsoPresent = true
			options.IsoBootConnected = false
			options.IsoFile = isoDetached
			options.IsoBootConnected = false
		case isoDisable:
			options.ModifyISO = true
		}

		ttyPipe := viper.GetString("tty_pipe")
		ttyDisable := viper.GetBool("tty_disable")
		if (ttyPipe != "") && ttyDisable {
			err := fmt.Errorf("conflict: tty pipe/disable")
			cobra.CheckErr(err)
		}
		ttyClient := viper.GetBool("tty_client")
		ttyAppMode := viper.GetBool("tty_app_mode")

		if ttyDisable && (ttyClient || ttyAppMode) {
			err := fmt.Errorf("conflict: tty disable/client|app_mode")
			cobra.CheckErr(err)
		}
		switch {
		case ttyPipe != "":
			options.ModifyTTY = true
			options.SerialPipe = ttyPipe
			options.SerialClient = ttyClient
			options.SerialAppMode = ttyAppMode
		case viper.GetBool("tty_disable"):
			options.ModifyTTY = true
		}

		vncEnable := viper.GetBool("vnc_enable")
		vncDisable := viper.GetBool("vnc_disable")
		if vncEnable && vncDisable {
			err := fmt.Errorf("conflict: vnc enable/disable")
			cobra.CheckErr(err)
		}
		switch {
		case vncEnable:
			options.ModifyVNC = true
			options.VNCEnabled = true
			options.VNCPort = viper.GetInt("vnc_port")
		case vncDisable:
			options.ModifyVNC = true
		}

		bootEFI := viper.GetBool("boot_efi")
		bootBIOS := viper.GetBool("boot_bios")
		if bootEFI && bootBIOS {
			err := fmt.Errorf("conflict: EFI/BIOS")
			cobra.CheckErr(err)
		}
		switch {
		case bootEFI:
			options.ModifyEFI = true
			options.EFIBoot = true
		case bootBIOS:
			options.ModifyEFI = true
		}

		actions, err := vmx.Modify(vm.Name, options)
		cobra.CheckErr(err)
		if OutputJSON {
			output := make(map[string]any)
			output[vm.Name] = actions
			fmt.Println(FormatJSON(output))
		} else {
			if viper.GetBool("verbose") {
				for _, action := range *actions {
					fmt.Printf("[%s] %s\n", vm.Name, action)
				}
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(modifyCmd)
	OptionSwitch(modifyCmd, "eth-enable", "", "configure ethernet NIC")
	OptionSwitch(modifyCmd, "eth-disable", "", "remove the ethernet NIC")
	OptionString(modifyCmd, "eth-mac", "", "", "enable ethernet with user-defined MAC address")
	OptionString(modifyCmd, "eth-auto", "", "", "enable ethernet with auto-generated MAC address")

	OptionString(modifyCmd, "iso-attach", "", "", "CD/DVD ISO boot file")
	OptionString(modifyCmd, "iso-detach", "", "", "CD/DVD ISO file (detached at boot)")
	OptionSwitch(modifyCmd, "iso-disable", "", "disable CD/DVD ISO")

	OptionSwitch(modifyCmd, "vnc-enable", "", "enable the integrated VNC server")
	OptionSwitch(modifyCmd, "vnc-disable", "", "disable and remove VNC")
	OptionString(modifyCmd, "vnc-port", "", "5900", "set the VNC server listen port")

	OptionString(modifyCmd, "tty-pipe", "", "", "enable serial port on named pipe")
	OptionSwitch(modifyCmd, "tty-disable", "", "disable serial port")
	OptionString(modifyCmd, "tty-client", "", "", "instance is the client end")
	OptionString(modifyCmd, "tty-app-mode", "", "", "configure for app interaction")

	OptionSwitch(modifyCmd, "boot-efi", "", "set EFI boot firmware")
	OptionSwitch(modifyCmd, "boot-bios", "", "set BIOS boot firmware")

}
