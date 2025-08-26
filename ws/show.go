package ws

import (
	"log"
	"strings"
)

type ShowOptions struct {
	Running bool
	Detail  bool
}

func (v *vmctl) Show(name string, options ShowOptions) (*[]VMState, error) {
	if v.debug {
		log.Printf("Show(%s, %+v)\n", name, options)
	}

	vids := []*VID{}

	if name == "" && options.Running {
		// we only need the running vms, so spoof vids with only the Name using vmrun output
		olines, err := v.RemoteExec("vmrun list", nil)
		if err != nil {
			return nil, Fatal(err)
		}
		for _, line := range olines {
			if !strings.HasPrefix(line, "Total running VMs:") {
				runningName, err := PathToName(line)
				if err != nil {
					return nil, Fatal(err)
				}
				vids = append(vids, &VID{Name: runningName})
			}
		}
	} else {
		// set vids from API
		v, err := v.cli.GetVIDs()
		if err != nil {
			return nil, Fatal(err)
		}
		vids = v
	}

	selected := []*VID{}
	for _, vid := range vids {
		if name == "" || (strings.ToLower(name) == strings.ToLower(vid.Name)) {
			selected = append(selected, vid)
		}
	}

	vms := make([]VMState, len(selected))
	for i, vid := range selected {
		if options.Detail {
			state, err := v.GetState(vid.Name)
			if err != nil {
				return nil, Fatal(err)
			}
			vms[i] = *state
		} else {
			vms[i] = VMState{Name: vid.Name}
		}
	}
	return &vms, nil
}
