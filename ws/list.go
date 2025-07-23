package ws

import (
	"log"
	"path"
	"regexp"
	"sort"
	"strings"
)

var VMX_PATTERN = regexp.MustCompile(`^.*\.[vV][mM][xX]$`)
var ISO_PATTERN = regexp.MustCompile(`^.*\.[iI][sS][oO]$`)
var ALL_PATTERN = regexp.MustCompile(`.*`)

type FilesOptions struct {
	Detail bool
	All    bool
	Iso    bool
}

func (v *vmctl) Files(vid string, options FilesOptions) ([]string, error) {
	if v.debug {
		log.Printf("Files(%s, %+v)\n", vid, options)
	}

	lines := []string{}

	pattern := VMX_PATTERN
	var paths []string
	if options.Iso {
		p, err := FormatIsoPath(v.IsoPath, vid)
		if err != nil {
			return lines, err
		}
		paths = []string{p}
		pattern = ISO_PATTERN
	} else if vid == "" {
		vmids, err := v.cli.GetVIDs()
		if err != nil {
			return lines, err
		}
		for _, vmid := range vmids {
			vmPath, _ := path.Split(vmid.Path)
			log.Printf("Files path: %s\n", vmPath)
			paths = append(paths, vmPath)
		}
	} else {
		vm, err := v.cli.GetVM(vid)
		if err != nil {
			return lines, err
		}
		vmPath, _ := path.Split(vm.Path)
		paths = []string{vmPath}
	}

	if options.Detail || options.All {
		pattern = ALL_PATTERN
	}

	for _, listPath := range paths {
		plines, err := v.listFiles(listPath, options.Detail, pattern)
		if err != nil {
			return lines, err
		}
		lines = append(lines, plines...)
	}

	if !options.Detail {
		sort.Strings(lines)
	}

	return lines, nil
}

func (v *vmctl) listFiles(listPath string, detail bool, pattern *regexp.Regexp) ([]string, error) {

	if v.debug {
		log.Printf("listFiles(%s, %v, %+v)\n", listPath, detail, *pattern)
	}

	lines := []string{}

	n, err := PathNormalize(listPath)
	if err != nil {
		return lines, err
	}
	if n != listPath {
		log.Printf("WARNING: listFiles received non-normalized path: '%s'\n", listPath)
		listPath = n
	}

	hostPath, err := PathnameFormat(v.Remote, listPath)
	if err != nil {
		return lines, err
	}

	var command string
	if v.Remote == "windows" {
		if detail {
			command = "dir /-C " + hostPath

		} else {
			command = "dir /B " + hostPath
		}
	} else {
		if detail {
			command = "ls -al " + hostPath
		} else {
			command = "ls " + hostPath
		}
	}

	olines, err := v.RemoteExec(command, nil)
	if err != nil {
		return lines, err
	}

	log.Printf("listFiles pattern: %+v\n", pattern)
	log.Printf("listFiles detail: %v\n", detail)
	for _, line := range olines {
		line = strings.TrimSpace(line)
		log.Printf("listFiles.line: %s\n", line)
		// FIXME: detail (--long) implies (--all) -- not documented as such
		if detail {
			lines = append(lines, line)
		} else if pattern.MatchString(line) {
			nline, err := PathNormalize(path.Join(listPath, line))
			if err != nil {
				return lines, err
			}
			lines = append(lines, nline)
		}
	}

	return lines, nil
}
