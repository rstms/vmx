package ws

import (
	"fmt"
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
		filename, err := v.downloadISO(vm, options.IsoFile, options.IsoClientCert, options.IsoClientKey, options.IsoCA)
		if err != nil {
			return Fatal(err)
		}
		options.IsoFile = filename
	}
	return nil
}

func (v *vmctl) uploadTLSCredential(vm *VM, pathname string) (string, error) {

	certsPath := ViperGetString("certs_path")
	hostCertsDir, err := PathFormat(v.Remote, certsPath)
	if err != nil {
		return "", Fatal(err)
	}
	var exit int
	_, err = v.RemoteExec("mkdir "+hostCertsDir, &exit)
	if err != nil {
		return "", Fatal(err)
	}

	normalized, err := PathNormalize(pathname)
	if err != nil {
		return "", Fatal(err)
	}
	_, name := path.Split(normalized)

	vmCredential := path.Join(certsPath, name)
	err = v.UploadFile(vm, normalized, vmCredential)
	if err != nil {
		return "", Fatal(err)
	}
	hostCredential, err := PathnameFormat(v.Remote, vmCredential)
	if err != nil {
		return "", Fatal(err)
	}
	return hostCredential, nil
}

func (v *vmctl) downloadISO(vm *VM, url, cert, key, ca string) (string, error) {

	_, filename := path.Split(url)

	// generate a normalized full pathname for the ISO file
	vmxIsoFile, err := FormatIsoPathname(v.IsoPath, filename)
	if err != nil {
		return "", Fatal(err)
	}

	command := ViperGetString("iso_download.command")

	if cert != "" || key != "" {
		if key == "" {
			return "", Fatalf("missing key for client certificate file: '%s'", cert)
		}
		if cert == "" {
			return "", Fatalf("missing client certificate for key file: '%s'", key)
		}

		hostCert, err := v.uploadTLSCredential(vm, cert)
		if err != nil {
			return "", Fatal(err)
		}
		hostKey, err := v.uploadTLSCredential(vm, key)
		if err != nil {
			return "", Fatal(err)
		}
		cert_flag := ViperGetString("iso_download.client_cert_flag")
		key_flag := ViperGetString("iso_download.client_key_flag")
		command += fmt.Sprintf(" %s %s %s %s", cert_flag, hostCert, key_flag, hostKey)
	}

	if ca != "" {
		hostCA, err := v.uploadTLSCredential(vm, ca)
		if err != nil {
			return "", Fatal(err)
		}
		key_flag := ViperGetString("iso_download.ca_flag")
		command += fmt.Sprintf(" %s %s", key_flag, hostCA)
	}

	filename_flag := ViperGetString("iso_download.filename_flag")
	hostFilename, err := PathFormat(v.Remote, vmxIsoFile)
	if err != nil {
		return "", Fatal(err)
	}

	command += fmt.Sprintf(" %s %s %s", filename_flag, hostFilename, url)

	if v.verbose {
		fmt.Printf("[%s] downloading %s to %s...\n", vm.Name, url, vmxIsoFile)
	}

	_, err = v.RemoteExec(command, nil)
	if err != nil {
		return "", Fatal(err)
	}

	if v.verbose {
		fmt.Printf("[%s] iso download complete\n", vm.Name)
	}

	return vmxIsoFile, Fatal(err)
}
