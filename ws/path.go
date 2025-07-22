package ws

import (
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

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

func PathNormalize(path string) (string, error) {
	//log.Printf("PathNormalize(%s)\n", path)
	match := DRIVE_LETTER.FindStringSubmatch(path)
	//log.Printf("match: %d %v\n", len(match), match)
	if len(match) == 3 {
		if len(match[2]) == 0 || !strings.HasPrefix(match[2], "\\") {
			return "", fmt.Errorf("cannot normalize path: %s", path)
		}
		path = "\\" + string(match[1][0]) + match[2]
	}
	if strings.Contains(path, "\\") {
		path = strings.ReplaceAll(path, "\\", "/")
	}
	//log.Printf("PathNormalize returning: %s\n", path)
	return path, nil
}

func PathnameFormat(os, path string) (string, error) {
	log.Printf("PathnameFormat(%s, %s)\n", os, path)
	path, err := PathNormalize(path)
	if err != nil {
		return "", err
	}
	switch os {
	case "windows":
		match := DRIVE_LETTER_NORMALIZED.FindStringSubmatch(path)
		log.Printf("match: %d %v\n", len(match), match)
		if len(match) == 3 {
			path = string(match[1][1]) + ":\\" + match[2]
		}
		path = strings.ReplaceAll(path, "/", "\\")
	case "scp":
		match := DRIVE_LETTER_NORMALIZED.FindStringSubmatch(path)
		if len(match) == 3 {
			//log.Printf("match: %v\n", match)
			path = fmt.Sprintf("%s:/%s", string(match[1][1]), match[2])
			//log.Printf("path: %s\n", path)
		}
	case "default":
		path = filepath.ToSlash(path)
	}
	log.Printf("PathnameFormat returning: %s\n", path)
	return path, nil
}

func PathToName(path string) (string, error) {
	//log.Printf("PathToName(%s)\n", path)
	path, err := PathNormalize(path)
	if err != nil {
		return "", err
	}
	_, filename := filepath.Split(path)
	name, _, _ := strings.Cut(filename, ".")
	//log.Printf("PathToName returning: %s\n", name)
	return name, nil
}

func ParseFileList(os string, lines []string) ([]VMFile, error) {
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
	return files, nil
}

func PathChdirCommand(os string, path string) (string, error) {
	var command string
	path, err := PathNormalize(path)
	if err != nil {
		return "", nil
	}
	if os == "windows" {
		match := DRIVE_LETTER_NORMALIZED.FindStringSubmatch(path)
		if len(match) == 3 {
			command = string(match[1][1]) + ": & cd \\" + match[2] + " & "
		} else {
			command = "cd " + path + " & "
		}
		command = strings.ReplaceAll(command, "/", "\\")
	} else {
		command = "cd " + path + " ; "
	}
	//fmt.Printf("PathChdirCommand: %s\n", command)
	return command, nil
}

func IsIsoPath(path string) bool {
	sep := string(filepath.Separator)
	switch {
	case path == "iso":
		return true
	case strings.HasPrefix(path, "iso"+sep):
		return true
	case strings.HasSuffix(path, sep+"iso"):
		return true
	}
	return false
}

func FormatIsoPath(isoPath, path string) string {
	sep := string(filepath.Separator)
	// reject `^/.*`
	if strings.HasPrefix(path, sep) {
		return ""
	}
	// remove `/*$`
	path = strings.TrimRight(path, sep)
	if path == "iso" || path == "" {
		return isoPath
	}
	// remove `^iso/`
	if strings.HasPrefix(path, "iso"+sep) {
		path = path[4:]
	}
	// prepend isoPath
	return filepath.Join(isoPath, path)
}

func FormatIsoPathname(isoPath, path string) string {
	dir, file := filepath.Split(path)
	if file == "" {
		return ""
	}
	log.Printf("isoPath=%s path=%s dir=%s file=%s\n", isoPath, path, dir, file)
	if strings.HasPrefix(dir, "iso"+string(filepath.Separator)) {
		dir = filepath.Join(isoPath, dir[4:])
	} else if !strings.HasPrefix(dir, string(filepath.Separator)) {
		dir = filepath.Join(isoPath, dir)
	}
	if !strings.HasSuffix(file, ".iso") {
		file += ".iso"
	}
	return filepath.Join(dir, file)
}
