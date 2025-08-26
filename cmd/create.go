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

		options := ws.NewCreateOptions()

		options.Wait = ViperGetBool("wait")
		options.CpuCount = ViperGetInt("cpu")
		options.MemorySize = ViperGetString("ram")
		options.DiskSize = ViperGetString("disk")
		options.DiskPreallocated = ViperGetBool("preallocate")
		options.DiskSingleFile = ViperGetBool("single_file")
		options.EFIBoot = ViperGetBool("efi")
		options.HostTimeSync = ViperGetBool("time_sync")
		options.GuestTimeZone = ViperGetString("timezone")
		options.ClipboardEnabled = ViperGetBool("clipboard")
		options.MacAddress = ViperGetString("mac")

		switch {
		case ViperGetBool("openbsd"):
			options.GuestOS = "openbsd-64"
		case ViperGetBool("debian"):
			options.GuestOS = "debian12-64"
		case ViperGetBool("windows"):
			options.GuestOS = "windows11-64"
		case ViperGetBool("ubuntu"):
			options.GuestOS = "ubuntu-64"
		default:
			options.GuestOS = "other-64"
		}

		isoOptions, err := InitIsoOptions()
		cobra.CheckErr(err)

		result, err := vmx.Create(name, *options, *isoOptions)
		cobra.CheckErr(err)
		if OutputJSON && options.Wait && ViperGetBool("status") {
			OutputInstanceState(name, result)
		}
	},
}

func init() {
	CobraAddCommand(rootCmd, rootCmd, createCmd)
	OptionString(createCmd, "cpu", "", "1", "cpu count")
	OptionString(createCmd, "ram", "", "2G", "memory size")
	OptionString(createCmd, "disk", "", "16G", "disk size")
	OptionString(createCmd, "timezone", "", "UTC", "guest time zone")
	OptionString(createCmd, "mac", "", "auto", "MAC address")
	OptionSwitch(createCmd, "efi", "", "EFI boot")
	OptionSwitch(createCmd, "time-sync", "", "enable time sync with host")
	OptionSwitch(createCmd, "clipboard", "", "enable clipboard sharing with host")
	OptionSwitch(createCmd, "single-file", "", "create single-file VMDK disk")
	OptionSwitch(createCmd, "preallocated", "", "pre-allocate VMDK disk")
	OptionSwitch(createCmd, "openbsd", "", "OpenBSD guest")
	OptionSwitch(createCmd, "debian", "", "Debian guest")
	OptionSwitch(createCmd, "ubuntu", "", "Ubuntu guest")
	OptionSwitch(createCmd, "windows", "", "Windows guest")
}
