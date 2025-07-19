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

var createCmd = &cobra.Command{
	Use:   "create NAME",
	Short: "generate new VM instance",
	Long: `
Create the named VM instance on the host.  Generate a VMDK virtual disk in
the instance subdirectory.  Default instance creation parameters are used 
unless specified with option flags.
`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		InitController()
		name := args[0]

		options := workstation.NewCreateOptions()

		options.CpuCount = viper.GetInt("cpu")
		options.MemorySize = viper.GetString("ram")
		options.DiskSize = viper.GetString("disk")
		options.DiskPreallocated = viper.GetBool("preallocate")
		options.DiskSingleFile = viper.GetBool("single_file")
		options.EFIBoot = viper.GetBool("efi")
		options.HostTimeSync = viper.GetBool("time_sync")
		options.GuestTimeZone = viper.GetString("timezone")
		options.DisableDragAndDrop = !viper.GetBool("drag_and_drop")
		options.DisableClipboard = !viper.GetBool("clipboard")
		options.DisableFilesystemShare = !viper.GetBool("filesystem_share")
		options.MacAddress = viper.GetString("mac")
		options.IsoFile = viper.GetString("iso")
		options.IsoPresent = options.IsoFile != ""
		options.IsoBootConnected = !viper.GetBool("detach_iso")
		switch {
		case viper.GetBool("openbsd"):
			options.GuestOS = "openbsd"
		case viper.GetBool("debian"):
			options.GuestOS = "debian"
		case viper.GetBool("windows"):
			options.GuestOS = "windows"
		case viper.GetBool("ubuntu"):
			options.GuestOS = "ubuntu"
		default:
			options.GuestOS = "other"
		}
		fmt.Printf("cmd/Create: viper.GetString(iso)=%s\n", viper.GetString("iso"))
		fmt.Printf("cmd/Create: options.IsoFile=%s\n", options.IsoFile)
		vm, err := vmx.Create(name, *options)
		cobra.CheckErr(err)
		if viper.GetBool("verbose") {
			fmt.Println(vm)
		}
	},
}

func init() {
	rootCmd.AddCommand(createCmd)
	OptionString(createCmd, "cpu", "", "1", "cpu count")
	OptionString(createCmd, "ram", "", "2G", "memory size")
	OptionString(createCmd, "disk", "", "16G", "disk size")
	OptionString(createCmd, "timezone", "", "UTC", "guest time zone")
	OptionString(createCmd, "mac", "", "auto", "MAC address")
	OptionString(createCmd, "iso", "", "", "boot ISO pathname or URL")
	OptionSwitch(createCmd, "detach-iso", "", "detach ISO at boot")
	OptionSwitch(createCmd, "efi", "", "EFI boot")
	OptionSwitch(createCmd, "time-sync", "", "enable time sync with host")
	OptionSwitch(createCmd, "drag-and-drop", "", "enable drag-and-drop")
	OptionSwitch(createCmd, "clipboard", "", "enable clipboard sharing with host")
	OptionSwitch(createCmd, "filesystem-share", "", "enable host/guest filesystem sharing")
	OptionSwitch(createCmd, "single-file", "", "create single-file VMDK disk")
	OptionSwitch(createCmd, "preallocated", "", "pre-allocate VMDK disk")
	OptionSwitch(createCmd, "openbsd", "", "OpenBSD guest")
	OptionSwitch(createCmd, "debian", "", "Debian guest")
	OptionSwitch(createCmd, "ubuntu", "", "Ubuntu guest")
	OptionSwitch(createCmd, "windows", "", "Windows guest")
}
