package workstation

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
