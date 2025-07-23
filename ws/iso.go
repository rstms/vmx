package ws

import (
	"fmt"
	"path"
	"strings"
)

type IsoOptions struct {
	ModifyISO        bool
	IsoPresent       bool
	IsoFile          string
	IsoCA            string
	IsoClientCert    string
	IsoClientKey     string
	IsoBootConnected bool
}

func (v *vmctl) CheckISODownload(options *IsoOptions) error {

	if !options.ModifyISO {
		return nil
	}
	// if IsoFile is a URL, download the ISO
	if strings.HasPrefix(options.IsoFile, "http:") || strings.HasPrefix(options.IsoFile, "https:") {
		filename, err := v.downloadISO(options.IsoFile, options.IsoClientCert, options.IsoClientKey, options.IsoCA)
		if err != nil {
			return err
		}
		options.IsoFile = filename
	}
	return nil
}

func (v *vmctl) downloadISO(url, cert, key, ca string) (string, error) {

	_, basename := path.Split(url)
	pathname, err := FormatIsoPathname(v.IsoPath, basename)
	if err != nil {
		return "", err
	}
	hostPathname, err := PathnameFormat(v.Remote, pathname)
	if err != nil {
		return "", err
	}

	clientCertArg := ""
	if cert != "" || key != "" {
		if key == "" {
			return "", fmt.Errorf("missing key for client certificate file: '%s'", cert)
		}
		if cert == "" {
			return "", fmt.Errorf("missing client certificate for key file: '%s'", key)
		}
		clientCertArg = fmt.Sprintf(" --cert %s --key %s", cert, key)
	}

	caArg := ""
	if ca != "" {
		caArg = fmt.Sprintf(" --cacert %s", ca)
	}

	command := fmt.Sprintf("curl -L -s%s%s %s", clientCertArg, caArg, hostPathname)

	_, err = v.RemoteExec(command, nil)
	if err != nil {
		return "", nil
	}
	return basename, err
}
