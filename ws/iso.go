package ws

import (
	"path"
	"strings"
)

type IsoOptions struct {
	ModifyISO           bool
	IsoPresent          bool
	IsoFile             string
	IsoCA               string
	IsoClientCert       string
	IsoClientKey        string
	IsoBootConnected    bool
	ModifyBootConnected bool
}

func (v *vmctl) CheckISODownload(vm *VM, options *IsoOptions) error {

	if !options.ModifyISO {
		return nil
	}
	// if IsoFile is a URL, download the ISO
	if strings.HasPrefix(options.IsoFile, "http:") || strings.HasPrefix(options.IsoFile, "https:") {
		url := options.IsoFile
		_, filename := path.Split(url)
		// generate a normalized full pathname for the ISO file
		vmxFilename, err := FormatIsoPathname(v.IsoPath, filename)
		if err != nil {
			return Fatal(err)
		}
		err = v.winexec.GetISO(vmxFilename, url, options.IsoCA, options.IsoClientCert, options.IsoClientKey, nil)
		if err != nil {
			return Fatal(err)
		}
		options.IsoFile = vmxFilename
	}
	return nil
}
