package workstation

type VmRestGetVmCpuRamResponse struct {
	Cpu struct {
		Processors int `json:"processors,omitzero"`
	} `json:"cpu"`
	ID     string `json:"id,omitzero"`
	Memory int    `json:"memory,omitzero"`
}
