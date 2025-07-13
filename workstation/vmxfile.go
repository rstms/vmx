package workstation

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var DRIVE_LETTER = regexp.MustCompile(`^([a-zA-Z]:)(.*)`)
var DRIVE_LETTER_NORMALIZED = regexp.MustCompile(`^(/[a-zA-Z]/)(.*)`)

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

func PathFormat(os, path string) (string, error) {
	//log.Printf("PathFormat(%s, %s)\n", os, path)
	path, err := PathNormalize(path)
	if err != nil {
		return "", err
	}
	if os == "windows" {
		match := DRIVE_LETTER_NORMALIZED.FindStringSubmatch(path)
		//log.Printf("match: %d %v\n", len(match), match)
		if len(match) == 3 {
			path = string(match[1][1]) + ":\\" + match[2]
		}
		path = strings.ReplaceAll(path, "/", "\\")
	} else {
		path = filepath.ToSlash(path)
	}
	//log.Printf("PathFormat returning: %s\n", path)
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
