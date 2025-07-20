package workstation

import (
	"fmt"
	"path"
)

func (v *vmctl) DownloadISO(url, cert, key, ca string) (string, error) {

	_, basename := path.Split(url)
	pathname := FormatIsoPathname(v.IsoPath, basename)
	hostPathname, err := PathFormat(v.Remote, pathname)
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
