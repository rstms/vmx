package ws

import (
	"bufio"
	"bytes"
	"encoding/json"
	"github.com/vmware/govmomi/vmdk"
	"log"
	"regexp"
)

var VMDK_VMX_LINE = regexp.MustCompile(`^([^:]+:\d+).fileName = "([^.]+.[vV][mM][dD][kK])"\s*$`)

type VDiskType int

const (
	DiskTypeSingleFileGrowable = iota
	DiskTypeMultiFileGrowable
	DiskTypeSingleFilePreallocated
	DiskTypeMultiFilePreallocated
	DiskTypeESXPreallocated
	DiskTypeStreaming
	DiskTypeThin
)

var diskTypeName = map[VDiskType]string{
	DiskTypeSingleFileGrowable:     "single_file_growable",
	DiskTypeMultiFileGrowable:      "multiple_file_growable",
	DiskTypeSingleFilePreallocated: "sigle_file_preallocated",
	DiskTypeMultiFilePreallocated:  "multiple_file_preallocated",
	DiskTypeESXPreallocated:        "preallocated_ESX",
	DiskTypeStreaming:              "compressed_streaming_optimized",
	DiskTypeThin:                   "thin_provisioned",
}

func (dt VDiskType) String() string {
	return diskTypeName[dt]
}

type VMDisk struct {
	Device     string
	File       string
	Capacity   int64
	Size       string
	Descriptor map[string]any
}

func ParseDiskType(singleFile, preallocated bool) VDiskType {
	switch {
	case singleFile && preallocated:
		return DiskTypeSingleFilePreallocated
	case singleFile:
		return DiskTypeSingleFileGrowable
	case preallocated:
		return DiskTypeMultiFilePreallocated
	}
	return DiskTypeMultiFileGrowable
}

// search lines of a VMX file returning map of device[vmxfile], true_if_found
func ScanVMX(data []byte) (map[string]string, error) {
	disks := make(map[string]string)
	scanner := bufio.NewScanner(bytes.NewBuffer(data))
	for scanner.Scan() {
		line := scanner.Text()
		log.Printf("line: %s\n", line)
		match := VMDK_VMX_LINE.FindStringSubmatch(line)
		log.Printf("    match: %v\n", match)
		if len(match) == 3 {
			disks[match[1]] = match[2]
		}
	}
	err := scanner.Err()
	if err != nil {
		return disks, Fatal(err)
	}
	return disks, nil
}

func NewVMDisk(device, filename string, data []byte) (*VMDisk, error) {

	disk := VMDisk{
		Device: device,
		File:   filename,
	}
	err := disk.parseVMDK(data)
	if err != nil {
		return nil, Fatal(err)
	}
	return &disk, nil
}

func (d *VMDisk) parseVMDK(data []byte) error {

	buf := bytes.NewBuffer(data)

	descriptor, err := vmdk.ParseDescriptor(buf)
	if err != nil {
		return Fatal(err)
	}
	descriptorData, err := json.MarshalIndent(descriptor, "", "  ")
	if err != nil {
		return Fatal(err)
	}
	err = json.Unmarshal(descriptorData, &d.Descriptor)
	if err != nil {
		return Fatal(err)
	}
	log.Printf("descriptorData: %s\n", string(descriptorData))

	d.Capacity = descriptor.Capacity()
	d.Size = FormatSize(d.Capacity)

	return nil
}
