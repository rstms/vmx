package workstation

type VmRestGetVmRestrictionsResponse struct {
	ApplianceView struct {
		Author        string `json:"author"`
		Port          string `json:"port"`
		ShowAtPowerOn string `json:"showAtPowerOn"`
		Version       string `json:"version"`
	} `json:"applianceView"`
	CddvdList struct {
		Devices []struct {
			ConnectionStatus int    `json:"connectionStatus,omitzero"`
			DevicePath       string `json:"devicePath,omitzero"`
			DeviceType       int    `json:"deviceType,omitzero"`
			Index            int    `json:"index"`
			StartConnected   bool   `json:"startConnected"`
		} `json:"devices"`
		Num int `json:"num"`
	} `json:"cddvdList"`
	Cpu struct {
		Processors int `json:"processors,omitzero"`
	} `json:"cpu"`
	FirewareType int `json:"firewareType"`
	FloppyList   struct {
		Devices []any `json:"devices"`
		Num     int   `json:"num"`
	} `json:"floppyList"`
	GroupID        string `json:"groupID"`
	GuestIsolation struct {
		CopyDisabled  bool `json:"copyDisabled"`
		DndDisabled   bool `json:"dndDisabled"`
		HgfsDisabled  bool `json:"hgfsDisabled"`
		PasteDisabled bool `json:"pasteDisabled"`
	} `json:"guestIsolation"`
	ID                  string `json:"id,omitzero"`
	IntegrityConstraint string `json:"integrityConstraint"`
	Memory              int    `json:"memory,omitzero"`
	NicList             struct {
		Nics []struct {
			Index      int    `json:"index,omitzero"`
			MacAddress string `json:"macAddress,omitzero"`
			Type       string `json:"type,omitzero"`
			Vmnet      string `json:"vmnet,omitzero"`
		} `json:"nics"`
		Num int `json:"num,omitzero"`
	} `json:"nicList"`
	OrgDisplayName   string `json:"orgDisplayName"`
	ParallelPortList struct {
		Devices []any `json:"devices"`
		Num     int   `json:"num"`
	} `json:"parallelPortList"`
	RemoteVnc struct {
		VncEnabled bool `json:"VNCEnabled"`
		VncPort    int  `json:"VNCPort,omitzero"`
	} `json:"remoteVNC"`
	SerialPortList struct {
		Devices []struct {
			ConnectionStatus int    `json:"connectionStatus,omitzero"`
			DevicePath       string `json:"devicePath,omitzero"`
			DeviceType       int    `json:"deviceType,omitzero"`
			Index            int    `json:"index"`
			StartConnected   bool   `json:"startConnected,omitzero"`
		} `json:"devices"`
		Num int `json:"num"`
	} `json:"serialPortList"`
	UsbList struct {
		Num        int `json:"num"`
		UsbDevices []struct {
			BackingType int    `json:"BackingType,omitzero"`
			BackingInfo string `json:"backingInfo"`
			Connected   bool   `json:"connected"`
			Index       int    `json:"index"`
		} `json:"usbDevices"`
	} `json:"usbList"`
}
