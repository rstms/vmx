package ws

import (
	"log"
	"path/filepath"
	"regexp"
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
	sep := string(filepath.Separator)

	pattern := VMX_PATTERN
	var paths []string
	if options.Iso {
		paths = []string{FormatIsoPath(v.IsoPath, vid)}
		pattern = ISO_PATTERN
	} else if vid == "" {
		vmids, err := v.cli.GetVIDs()
		if err != nil {
			return lines, err
		}
		for _, vmid := range vmids {
			path, _ := filepath.Split(vmid.Path)
			log.Printf("Files path: %s\n", path)
			paths = append(paths, path)
		}
	} else {
		vm, err := v.cli.GetVM(vid)
		if err != nil {
			return lines, err
		}
		path, _ := filepath.Split(vm.Path)
		paths = []string{path}
	}

	if options.Detail || options.All {
		pattern = ALL_PATTERN
	}

	for i, path := range paths {
		paths[i] = strings.TrimRight(paths[i], sep)
		plines, err := v.listFiles(path, options.Detail, pattern)
		if err != nil {
			return lines, err
		}
		lines = append(lines, plines...)
	}

	return lines, nil
}

func (v *vmctl) listFiles(path string, detail bool, pattern *regexp.Regexp) ([]string, error) {

	if v.debug {
		log.Printf("listFiles(%s, %v, %+v)\n", path, detail, *pattern)
	}

	lines := []string{}

	hostPath, err := PathFormat(v.Remote, path)
	if err != nil {
		return lines, err
	}
	path = hostPath

	var command string
	if v.Remote == "windows" {
		if detail {
			command = "dir /-C " + path

		} else {
			command = "dir /B " + path
		}
	} else {
		if detail {
			command = "ls -al " + path
		} else {
			command = "ls " + path
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
		if detail {
			lines = append(lines, line)
		} else if pattern.MatchString(line) {
			nline, err := PathNormalize(filepath.Join(strings.TrimRight(path, "/\\"), line))
			if err != nil {
				return lines, err
			}
			lines = append(lines, nline)
		}
	}

	return lines, nil
}
