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

	"github.com/rstms/vmx/ws"
	"github.com/spf13/cobra"
)

var modifyCmd = &cobra.Command{
	Use:   "modify VID",
	Short: "modify instance configuration properties",
	Long: `

vnc modify [FLAGS] VID

Change instance NIC, ISO, TTY, VNC, EFI configuration parameters.  
The instance must be powered off.

See the flags and options help for descriptions of the available settings.
Changes can be specified for multiple categories in a single command.
`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		InitController()
		vm, err := vmx.Get(args[0])
		cobra.CheckErr(err)

		options := ws.CreateOptions{}

		// initX functions depend on zero-values in CreateOptions

		err = initETHOptions(&options)
		cobra.CheckErr(err)

		err = initTTYOptions(&options)
		cobra.CheckErr(err)

		err = initVNCOptions(&options)
		cobra.CheckErr(err)

		err = initEFIOptions(&options)
		cobra.CheckErr(err)

		err = initShareOptions(&options)
		cobra.CheckErr(err)

		err = initClipboardOptions(&options)
		cobra.CheckErr(err)

		isoOptions, err := InitIsoOptions()
		cobra.CheckErr(err)

		actions, err := vmx.Modify(vm.Name, options, *isoOptions)
		cobra.CheckErr(err)
		if OutputJSON {
			output := make(map[string]any)
			output[vm.Name] = actions
			fmt.Println(FormatJSON(output))
		} else {
			if ViperGetBool("verbose") {
				for _, action := range *actions {
					fmt.Printf("[%s] %s\n", vm.Name, action)
				}
			}
		}
	},
}

func initETHOptions(options *ws.CreateOptions) error {
	enable := ViperGetBool("eth_enable")
	disable := ViperGetBool("eth_disable")
	if enable && disable {
		return Fatalf("conflict: eth_enable/eth_disable")
	}
	address := ViperGetString("eth_mac")
	if address != "" {
		enable = true
		if disable {
			return Fatalf("conflict: eth_mac/eth_disable")
		}
	}
	switch {
	case enable:
		options.ModifyNIC = true
		if address == "" {
			address = "auto"
		}
		options.MacAddress = address
	case disable:
		options.ModifyNIC = true
	}
	return nil
}

func initTTYOptions(options *ws.CreateOptions) error {

	ttyPipe := ViperGetString("tty_pipe")
	disable := ViperGetBool("tty_disable")
	ttyClient := ViperGetBool("tty_client")
	ttyV2V := ViperGetBool("tty_v2v")

	switch {
	case ttyPipe != "":
		if disable {
			return Fatalf("conflict: tty-pipe/tty-disable")
		}
		options.ModifyTTY = true
		options.SerialPipe = ttyPipe
		options.SerialClient = ttyClient
		options.SerialV2V = ttyV2V
	case disable:
		if ttyClient {
			return Fatalf("conflict: tty-disable/tty-client")
		}
		if ttyV2V {
			return Fatalf("conflict: tty-disable/tty-v2v")
		}
		options.ModifyTTY = true
	}
	return nil
}

func initVNCOptions(options *ws.CreateOptions) error {
	enable := ViperGetBool("vnc_enable")
	disable := ViperGetBool("vnc_disable")
	if enable && disable {
		return Fatalf("conflict: vnc-enable/vnc-disable")
	}
	switch {
	case enable:
		options.ModifyVNC = true
		options.VNCEnabled = true
		options.VNCPort = ViperGetInt("vnc_port")
	case disable:
		options.ModifyVNC = true
	}
	return nil
}

func initEFIOptions(options *ws.CreateOptions) error {
	bootEFI := ViperGetBool("boot_efi")
	bootBIOS := ViperGetBool("boot_bios")
	if bootEFI && bootBIOS {
		return Fatalf("conflict: boot-efi/boot-bios")
	}
	switch {
	case bootEFI:
		options.ModifyEFI = true
		options.EFIBoot = true
	case bootBIOS:
		options.ModifyEFI = true
	}
	return nil
}

func initShareOptions(options *ws.CreateOptions) error {
	enable := ViperGetString("share_enable")
	disable := ViperGetBool("share_disable")
	switch {
	case enable != "":
		if disable {
			return Fatalf("conflict: share-enable/share-disable")
		}
		options.ModifyShare = true
		options.FileShareEnabled = true
		host, guest, ok := strings.Cut(enable, ",")
		if !ok || host == "" || guest == "" {
			return Fatalf("failed parsing share-enable paths: '%s'", enable)
		}
		options.SharedHostPath = host
		options.SharedGuestPath = guest
	case disable:
		options.ModifyShare = true
	}
	return nil
}

func initClipboardOptions(options *ws.CreateOptions) error {
	enable := ViperGetBool("clibboard_enable")
	disable := ViperGetBool("clipboard_disable")
	switch {
	case enable:
		if disable {
			return Fatalf("conflict: clipboard-enable/clipboard-disable")
		}
		options.ModifyClipboard = true
		options.ClipboardEnabled = true
	case disable:
		options.ModifyClipboard = true
	}
	return nil
}

func init() {
	CobraAddCommand(rootCmd, rootCmd, modifyCmd)
	OptionSwitch(modifyCmd, "eth-enable", "", "enable ethernet [auto-generated MAC]")
	OptionString(modifyCmd, "eth-mac", "", "", "enable ethernet [user-defined MAC]")
	OptionSwitch(modifyCmd, "eth-disable", "", "remove ethernet device")

	OptionSwitch(modifyCmd, "vnc-enable", "", "enable instance VNC server")
	OptionString(modifyCmd, "vnc-port", "", "5900", "VNC listen port")
	OptionSwitch(modifyCmd, "vnc-disable", "", "disable VNC server")

	OptionString(modifyCmd, "tty-pipe", "", "", "enable serial port with named pipe")
	OptionSwitch(modifyCmd, "tty-disable", "", "disable and remove serial port")
	OptionSwitch(modifyCmd, "tty-client", "", "instance connects to pipe [default: instance creates pipe]")
	OptionSwitch(modifyCmd, "tty-v2v", "", "configure for VM to VM connection")

	OptionSwitch(modifyCmd, "boot-efi", "", "select EFI boot firmware")
	OptionSwitch(modifyCmd, "boot-bios", "", "select BIOS boot firmware")

	OptionString(modifyCmd, "share-enable", "", "", "enable filesystem share [format: 'host_path,guest_path']")
	OptionSwitch(modifyCmd, "share-disable", "", "disable filesystem share")

	OptionSwitch(modifyCmd, "clipboard-enable", "", "enable copy/paste/drag-and-drop")
	OptionSwitch(modifyCmd, "clipboard-disable", "", "disable copy/paste/drag-and-drop")

}
