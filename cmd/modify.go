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

		err = initUSBOptions(&options)
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

	if ViperGetBool("modify.eth") {
		options.ModifyNIC = true
		options.MacAddress = "auto"
		return nil
	}

	if ViperGetBool("modify.no_eth") {
		options.ModifyNIC = true
		return nil
	}

	address := ViperGetString("modify.eth_mac")
	if address != "" {
		options.ModifyNIC = true
		options.MacAddress = address
		return nil
	}
	return nil
}

func initTTYOptions(options *ws.CreateOptions) error {

	ttyPipe := ViperGetString("modify.tty_pipe")
	disable := ViperGetBool("modify.tty_disable")
	ttyClient := ViperGetBool("modify.tty_client")
	ttyV2V := ViperGetBool("modify.tty_v2v")

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
	enable := ViperGetBool("modify.vnc_enable")
	disable := ViperGetBool("modify.vnc_disable")
	if enable && disable {
		return Fatalf("conflict: vnc-enable/vnc-disable")
	}
	switch {
	case enable:
		options.ModifyVNC = true
		options.VNCEnabled = true
		options.VNCPort = ViperGetInt("modify.vnc_port")
	case disable:
		options.ModifyVNC = true
	}
	return nil
}

func initEFIOptions(options *ws.CreateOptions) error {
	bootEFI := ViperGetBool("modify.boot_efi")
	bootBIOS := ViperGetBool("modify.boot_bios")
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
	if ViperGetBool("modify.no_share") {
		options.ModifyShare = true
		return nil
	}
	share := ViperGetString("modify.share")
	if share != "" {
		options.ModifyShare = true
		options.FileShareEnabled = true
		host, guest, ok := strings.Cut(share, ",")
		if !ok || host == "" || guest == "" {
			return Fatalf("failed parsing share-enable paths: '%s'", share)
		}
		options.SharedHostPath = host
		options.SharedGuestPath = guest
	}
	return nil
}

func initClipboardOptions(options *ws.CreateOptions) error {
	if ViperGetBool("modify.clibboard") {
		options.ModifyClipboard = true
		options.ClipboardEnabled = true
	}
	if ViperGetBool("modify.no_clipboard") {
		options.ModifyClipboard = true
		options.ClipboardEnabled = false
	}
	return nil
}

func initUSBOptions(options *ws.CreateOptions) error {

	if ViperGetBool("modify.no_usb") {
		options.ModifyUSB = true
		options.USBVersion = 0
		return nil
	}

	if ViperGetBool("modify.usb") {
		options.ModifyUSB = true
		options.USBVersion = 3
	}

	if ViperGetBool("modify.usb2") {
		options.ModifyUSB = true
		options.USBVersion = 2
	}

	if ViperGetBool("modify.usb_hid") {
		options.ModifyUSB = true
		options.AllowHID = true
	}

	if ViperGetBool("modify.usb_ccid") {
		options.ModifyUSB = true
		options.AllowCCID = true
	}

	if ViperGetBool("modify.usb_restrict") {
		options.ModifyUSB = true
		options.RestrictUSB = true
	}

	device := ViperGetString("modify.usb_device0")
	if device != "" {
		options.ModifyUSB = true
		options.Device0 = device
	}

	device = ViperGetString("modify.usb_device1")
	if device != "" {
		options.ModifyUSB = true
		options.Device1 = device
	}

	device = ViperGetString("modify.usb_device2")
	if device != "" {
		options.ModifyUSB = true
		options.Device2 = device
	}

	device = ViperGetString("modify.usb_device3")
	if device != "" {
		options.ModifyUSB = true
		options.Device3 = device
	}

	if options.ModifyUSB || (options.USBVersion != 2) {
		options.USBVersion = 3
	}

	return nil
}

func init() {
	CobraAddCommand(rootCmd, rootCmd, modifyCmd)
	OptionSwitch(modifyCmd, "eth", "", "enable ethernet (auto-generated MAC)")
	OptionString(modifyCmd, "eth-mac", "", "", "enable ethernet (user-defined MAC)")
	OptionSwitch(modifyCmd, "no-eth", "", "remove ethernet device")
	modifyCmd.MarkFlagsMutuallyExclusive("eth", "no-eth")

	OptionSwitch(modifyCmd, "vnc-enable", "", "enable instance VNC server")
	OptionString(modifyCmd, "vnc-port", "", "5900", "VNC listen port")
	OptionSwitch(modifyCmd, "vnc-disable", "", "disable VNC server")
	modifyCmd.MarkFlagsMutuallyExclusive("vnc-enable", "vnc-disable")

	OptionString(modifyCmd, "tty-pipe", "", "", "enable serial port with named pipe")
	OptionSwitch(modifyCmd, "tty-disable", "", "disable and remove serial port")
	OptionSwitch(modifyCmd, "tty-client", "", "instance connects to pipe [default: instance creates pipe]")
	OptionSwitch(modifyCmd, "tty-v2v", "", "configure for VM to VM connection")
	modifyCmd.MarkFlagsMutuallyExclusive("tty-disable", "tty-pipe")
	modifyCmd.MarkFlagsMutuallyExclusive("tty-disable", "tty-client")
	modifyCmd.MarkFlagsMutuallyExclusive("tty-disable", "tty-v2v")

	OptionSwitch(modifyCmd, "boot-efi", "", "select EFI boot firmware")
	OptionSwitch(modifyCmd, "boot-bios", "", "select BIOS boot firmware")
	modifyCmd.MarkFlagsMutuallyExclusive("boot-efi", "boot-bios")

	OptionString(modifyCmd, "share", "", "", "enable filesystem share [format: 'host_path,guest_path']")
	OptionSwitch(modifyCmd, "no-share", "", "disable filesystem share")

	OptionSwitch(modifyCmd, "clipboard", "", "enable copy/paste/drag-and-drop")
	OptionSwitch(modifyCmd, "no-clipboard", "", "disable copy/paste/drag-and-drop")
	modifyCmd.MarkFlagsMutuallyExclusive("clipboard", "no-clipboard")

	OptionSwitch(modifyCmd, "usb-hid", "", "enable USB AllowHID")
	OptionSwitch(modifyCmd, "usb-ccid", "", "enable USB AllowCCID")
	OptionSwitch(modifyCmd, "usb-restrict", "", "restrict USB devices")
	OptionString(modifyCmd, "usb-device0", "", "", "USB autoconnect device0 VID:PID")
	OptionString(modifyCmd, "usb-device1", "", "", "USB autoconnect device1 VID:PID")
	OptionString(modifyCmd, "usb-device2", "", "", "USB autoconnect device2 VID:PID")
	OptionString(modifyCmd, "usb-device3", "", "", "USB autoconnect device3 VID:PID")
	OptionSwitch(modifyCmd, "usb", "", "enable USB controller version 3.2)")
	OptionSwitch(modifyCmd, "usb2", "", "enable USB Version 2.0 controller")
	OptionSwitch(modifyCmd, "no-usb", "", "disable USB controller")

	modifyCmd.MarkFlagsMutuallyExclusive("no-usb", "usb")
	modifyCmd.MarkFlagsMutuallyExclusive("no-usb", "usb2")
	modifyCmd.MarkFlagsMutuallyExclusive("usb2", "usb")
}
