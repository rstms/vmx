package ws

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var SIZE_PATTERN = regexp.MustCompile(`^\s*([0-9.]+)([KMGTP]B*){0,1}\s*$`)

const KB = int64(1024)
const MB = KB * 1024
const GB = MB * 1024
const TB = GB * 1024
const PB = TB * 1024

func SizeParse(param string) (int64, error) {
	var multiplier int64 = 1

	match := SIZE_PATTERN.FindStringSubmatch(param)
	//fmt.Printf("match: %d %v\n", len(match), match)
	if len(match) != 3 {
		return 0, Fatalf("failed parsing size parameter: '%s'", param)
	}
	number := match[1]
	suffix := match[2]
	switch suffix {
	case "":
		multiplier = 1
	case "K", "KB":
		multiplier = KB
	case "M", "MB":
		multiplier = MB
	case "G", "GB":
		multiplier = GB
	case "T", "TB":
		multiplier = TB
	case "P", "PB":
		multiplier = PB
	default:
		return 0, Fatalf("unexpected suffix in size parameter: '%s'", param)
	}
	fsize, err := strconv.ParseFloat(number, 64)
	if err != nil {
		return 0, Fatal(err)
	}
	size := int64(fsize * float64(multiplier))
	//fmt.Printf("%s == %d\n", param, size)
	return size, nil
}

func FormatSize(size int64) string {
	if ViperGetBool("no_humanize") {
		return fmt.Sprintf("%d", size)
	}
	suffix := ""
	var mult int64
	var sizeStr string
	switch {
	case size >= PB:
		mult = PB
		suffix = "P"
	case size >= TB:
		mult = TB
		suffix = "T"
	case size >= GB:
		mult = GB
		suffix = "G"
	case size >= MB:
		mult = MB
		suffix = "M"
	case size >= KB:
		mult = KB
		suffix = "K"
	default:
		sizeStr = fmt.Sprintf("%d", size)
	}
	if sizeStr == "" {
		if (size % mult) == 0 {
			sizeStr = fmt.Sprintf("%d%s", size/mult, suffix)
		} else {
			fsize := float64(size) / float64(mult)
			sizeStr = fmt.Sprintf("%.2f", fsize)
			if strings.HasPrefix(sizeStr, ".") {
				sizeStr = "0" + sizeStr
			}
			sizeStr = strings.TrimRight(sizeStr, "0")
			sizeStr = strings.TrimRight(sizeStr, ".")
			sizeStr += suffix
		}
	}
	//log.Printf("FormatSize(%d) -> %s\n", size, sizeStr)
	return sizeStr
}
