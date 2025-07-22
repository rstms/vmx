package ws

type VmRestGetVmsResponse []struct {
	ID   string `json:"id,omitzero"`
	Path string `json:"path,omitzero"`
	Name string `json: "name,omitzero"`
}
