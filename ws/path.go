package ws

import (
	"fmt"
	"log"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const debug = false

const ALLOW_ABSOLUTE_ISO_PATH = false

var DRIVE_LETTER = regexp.MustCompile(`^([a-zA-Z]:)(.*)`)
var DRIVE_LETTER_NORMALIZED = regexp.MustCompile(`^(/[a-zA-Z]/)(.*)`)

var WINDOWS_FILE_LIST = regexp.MustCompile(`^\d{2}\/\d{2}\/\d{4}\s+\d{2}:\d{2}\s+\S+\s+(\d+)\s+(\S+)\s*$`)
var UNIX_FILE_LIST = regexp.MustCompile(`^\S+\s+\S+\s+\S+\s+\S+\s+(\d+)\s+\S+\s+\S+\s+\S+\s+(\S+)\s*$`)

func PathCompare(first, second string) (bool, error) {
	first, err := PathNormalize(first)
	if err != nil {
		return false, err
	}
	second, err = PathNormalize(second)
	if err != nil {
		return false, err
	}
	return first == second, nil
}

func PathNormalize(inPath string) (string, error) {
	if debug {
		log.Printf("PathNormalize(%s)\n", inPath)
	}
	match := DRIVE_LETTER.FindStringSubmatch(inPath)
	//log.Printf("match: %d %v\n", len(match), match)
	if len(match) == 3 {
		if len(match[2]) == 0 || !strings.HasPrefix(match[2], "\\") {
			return "", fmt.Errorf("PathNormalize parse failed: %s", inPath)
		}
		inPath = "\\" + string(match[1][0]) + match[2]
	}
	if strings.Contains(inPath, "\\") {
		inPath = strings.ReplaceAll(inPath, "\\", "/")
	}

	for strings.Contains(inPath, "//") {
		inPath = strings.ReplaceAll(inPath, "//", "/")
	}

	if debug {
		log.Printf("PathNormalize returning: %s\n", inPath)
	}
	return inPath, nil
}

func PathFormat(os, inPath string) (string, error) {
	if debug {
		log.Printf("PathFormat(%s, %s)\n", os, inPath)
	}
	inPath, err := PathnameFormat(os, inPath)
	if err != nil {
		return "", err
	}
	inPath = strings.TrimRight(inPath, "/\\")
	if debug {
		log.Printf("PathFormat returning: %s\n", inPath)
	}
	return inPath, nil
}

func PathnameFormat(os, inPath string) (string, error) {
	if debug {
		log.Printf("PathnameFormat(%s, %s)\n", os, inPath)
	}
	inPath, err := PathNormalize(inPath)
	if err != nil {
		return "", err
	}
	switch os {
	case "windows", "scp":
		match := DRIVE_LETTER_NORMALIZED.FindStringSubmatch(inPath)
		//log.Printf("match: %d %v\n", len(match), match)
		if len(match) == 3 {
			driveLetter := match[1][1]
			subPath := match[2]
			inPath = fmt.Sprintf("%c:/%s", driveLetter, subPath)
		}
		if os == "windows" {
			inPath = strings.ReplaceAll(inPath, "/", "\\")
		}
	case "default":
		inPath = filepath.ToSlash(inPath)
	}
	if debug {
		log.Printf("PathnameFormat returning: %s\n", inPath)
	}
	return inPath, nil
}

func PathToName(inPath string) (string, error) {
	if debug {
		log.Printf("PathToName(%s)\n", inPath)
	}
	inPath, err := PathNormalize(inPath)
	if err != nil {
		return "", err
	}
	_, filename := path.Split(inPath)
	name, _, _ := strings.Cut(filename, ".")
	if name == "" {
		return "", fmt.Errorf("cannot parse name from: '%s'", inPath)
	}
	if debug {
		log.Printf("PathToName returning: %s\n", name)
	}
	return name, nil
}

func ParseFileList(os string, lines []string) ([]VMFile, error) {
	if debug {
		log.Printf("ParseFileList(%s, %v)\n", os, lines)
	}
	files := []VMFile{}
	for _, line := range lines {
		var match []string
		if os == "windows" {
			match = WINDOWS_FILE_LIST.FindStringSubmatch(line)
		} else {
			match = UNIX_FILE_LIST.FindStringSubmatch(line)
		}
		//fmt.Printf("match: %d %v\n", len(match), match)
		if len(match) == 3 {
			length, err := strconv.ParseUint(match[1], 10, 64)
			if err != nil {
				return []VMFile{}, err
			}
			files = append(files, VMFile{Name: match[2], Length: length})
		}

	}
	if debug {
		log.Printf("ParseFileList returning: %+v\n", files)
	}
	return files, nil
}

func PathChdirCommand(os string, inPath string) (string, error) {
	if debug {
		log.Printf("PathChdirCommand(%s, %s)\n", os, inPath)
	}
	var command string
	inPath, err := PathNormalize(inPath)
	if err != nil {
		return "", nil
	}
	if os == "windows" {
		match := DRIVE_LETTER_NORMALIZED.FindStringSubmatch(inPath)
		if len(match) == 3 {
			command = string(match[1][1]) + ": & cd \\" + match[2] + " & "
		} else {
			command = "cd " + inPath + " & "
		}
		command = strings.ReplaceAll(command, "/", "\\")
	} else {
		command = "cd " + inPath + " && "
	}
	if debug {
		log.Printf("PathChdirCommand returning: %s\n", command)
	}
	return command, nil
}

func IsIsoPath(inPath string) (bool, error) {
	nPath, err := PathNormalize(inPath)
	if err != nil {
		return false, err
	}
	ret := false
	switch {
	case nPath == "iso":
		ret = true
	case strings.HasPrefix(nPath, "iso/"):
		ret = true
	case strings.HasSuffix(nPath, "/iso"):
		ret = true
	}
	if debug {
		log.Printf("IsIsoPath(%s) returning: %v\n", inPath, ret)
	}
	return ret, nil
}

func FormatIsoPath(isoPath, subPath string) (string, error) {
	if debug {
		log.Printf("FormatIsoPath(%s, %s)\n", isoPath, subPath)
	}
	nPath, err := PathNormalize(subPath)
	if err != nil {
		return "", err
	}
	nIsoPath, err := PathNormalize(isoPath)
	if err != nil {
		return "", err
	}

	// reject `^/.*`
	if strings.HasPrefix(nPath, "/") {
		if ALLOW_ABSOLUTE_ISO_PATH {
			if debug {
				log.Printf("FormatIsoPath returning: %s\n", nPath)
			}
			return nPath, nil
		}
		return "", fmt.Errorf("ISO subpath is absolute: '%s'", subPath)
	}
	// remove `/$`
	nPath = strings.TrimRight(nPath, "/")
	if nPath == "iso" {
		nPath = ""
	}
	// remove `^iso/`
	if strings.HasPrefix(nPath, "iso/") {
		nPath = nPath[4:]
	}
	fPath := nIsoPath
	if nPath != "" {
		// join to normalized isoPath
		fPath = path.Join(nIsoPath, nPath)
	}
	if debug {
		log.Printf("FormatIsoPath returning: %s\n", fPath)
	}
	return fPath, nil
}

func FormatIsoPathname(isoPath, subPath string) (string, error) {
	if debug {
		log.Printf("FormatIsoPathname(%s, %s)\n", isoPath, subPath)
	}
	n, err := PathNormalize(subPath)
	if err != nil {
		return "", err
	}
	subPath = n
	n, err = PathNormalize(isoPath)
	if err != nil {
		return "", err
	}
	isoPath = n

	dir, file := path.Split(subPath)
	if file == "" {
		return "", fmt.Errorf("missing filename in: '%s'", subPath)
	}
	//log.Printf("isoPath=%s subPath=%s dir=%s file=%s\n", isoPath, subPath, dir, file)
	if strings.HasPrefix(dir, "iso/") {
		dir = path.Join(isoPath, dir[4:])
	} else if !strings.HasPrefix(dir, "/") {
		dir = path.Join(isoPath, dir)
	}
	if !strings.HasSuffix(file, ".iso") {
		file += ".iso"
	}
	formatted := path.Join(dir, file)
	if debug {
		log.Printf("FormatIsoPathname returning: %s\n", formatted)
	}
	return formatted, nil
}
