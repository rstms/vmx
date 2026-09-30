package controller

import (
	"github.com/rstms/winexec/geturl"
	"log"
	"os"
	"path"
	"slices"
	"strings"
)

type IsoOptions struct {
	ModifyISO           bool
	IsoPresent          bool
	IsoFiles            []string
	IsoCA               string
	IsoClientCert       string
	IsoClientKey        string
	IsoBootConnected    bool
	ModifyBootConnected bool
}

func NewIsoOptions() IsoOptions {
	return IsoOptions{IsoFiles: []string{}}
}

func (o *IsoOptions) DiffersFrom(other IsoOptions) bool {
	switch {
	case o.ModifyISO != other.ModifyISO:
	case o.IsoPresent != other.IsoPresent:
	case slices.Compare(o.IsoFiles, other.IsoFiles) != 0:
	case o.IsoCA != other.IsoCA:
	case o.IsoClientCert != other.IsoClientCert:
	case o.IsoClientKey != other.IsoClientKey:
	case o.IsoBootConnected != other.IsoBootConnected:
	default:
		return false
	}
	return true
}

func (v *vmctl) CheckISODownload(vm *VM, options *IsoOptions) ([]string, error) {

	log.Printf("CheckISODownload: options=%s\n", FormatJSON(*options))

	switch {
	case options.ModifyISO:
	case options.IsoPresent:
	case options.IsoBootConnected:
	case len(options.IsoFiles) > 0:
	default:
		log.Println("CheckISODownload: no download required")
		return nil, nil
	}

	pathnames := []string{}
	for _, url := range options.IsoFiles {

		// generate normalized ISO pathname
		_, filename := path.Split(url)
		isoPathname, err := FormatIsoPathname(v.IsoPath, filename)
		if err != nil {
			return nil, Fatal(err)
		}

		localController, err := v.isLocal()
		if err != nil {
			return nil, Fatal(err)
		}

		// if IsoFile is a URL, download the ISO
		switch {
		case strings.HasPrefix(url, "http:") || strings.HasPrefix(url, "https:"):
			switch {
			case localController:
				// vmx controller is local, download the file directly to the vmware ISO dir
				localizedIsoPathname, err := PathnameFormat(v.Local, isoPathname)
				if err != nil {
					return nil, Fatal(err)
				}
				var ca, cert, key []byte
				if options.IsoCA != "" {
					ca, err = os.ReadFile(options.IsoCA)
					if err != nil {
						return nil, Fatal(err)
					}
				}
				if options.IsoClientCert != "" {
					cert, err = os.ReadFile(options.IsoClientCert)
					if err != nil {
						return nil, Fatal(err)
					}
				}
				if options.IsoClientKey != "" {
					key, err = os.ReadFile(options.IsoClientKey)
					if err != nil {
						return nil, Fatal(err)
					}
				}
				_, err = geturl.GetURL(localizedIsoPathname, url, ca, cert, key)
				if err != nil {
					return nil, Fatal(err)
				}
				isoPathname = localizedIsoPathname
			case v.Shell == "winexec":
				err := v.checkWinexec()
				if err != nil {
					return nil, Fatal(err)
				}
				err = v.winexec.GetISO(isoPathname, url, options.IsoCA, options.IsoClientCert, options.IsoClientKey, nil)
				if err != nil {
					return nil, Fatal(err)
				}
			default:
				return nil, Fatalf("ISO Download Failure: either local or winexec is required; URL: %s", url)
			}
		case strings.HasPrefix(url, "file:"):
			switch {
			case localController:
				localizedIsoPathname, err := PathnameFormat(v.Local, isoPathname)
				if err != nil {
					return nil, Fatal(err)
				}
				if !IsFile(localizedIsoPathname) {
					return nil, Fatalf("local ISO file not found: %s\n", localizedIsoPathname)
				}
				isoPathname = localizedIsoPathname
			case v.Shell == "winexec":
				err := v.checkWinexec()
				if err != nil {
					return nil, Fatal(err)
				}
				exists, err := v.winexec.IsFile(isoPathname)
				if err != nil {
					return nil, Fatal(err)
				}
				if !exists {
					return nil, Fatalf("host ISO file not found: %s\n", isoPathname)
				}
			default:
				return nil, Fatalf("ISO Download Failure: either local or winexec is required; URL: %s", url)
			}

		default:
			return nil, Fatalf("invalid ISO download URL: %s", url)

		}
		pathnames = append(pathnames, isoPathname)
	}

	return pathnames, nil
}
