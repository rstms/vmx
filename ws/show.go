package ws

import (
	"log"
	"strings"
)

type ShowOptions struct {
	Running bool
	Detail  bool
}

func (v *vmctl) Show(name string, options ShowOptions) ([]VM, error) {
	if v.debug {
		log.Printf("Show(%s, %+v)\n", name, options)
	}

	vids := []*VID{}

	if name == "" && options.Running {
		// we only need the running vms, so spoof vids with only the Name using vmrun output
		olines, err := v.RemoteExec("vmrun list", nil)
		if err != nil {
			return []VM{}, err
		}
		for _, line := range olines {
			if !strings.HasPrefix(line, "Total running VMs:") {
				runningName, err := PathToName(line)
				if err != nil {
					return []VM{}, err
				}
				vids = append(vids, &VID{Name: runningName})
			}
		}
	} else {
		// set vids from API
		v, err := v.cli.GetVIDs()
		if err != nil {
			return []VM{}, err
		}
		vids = v
	}

	selected := []*VID{}
	for _, vid := range vids {
		if name == "" || (strings.ToLower(name) == strings.ToLower(vid.Name)) {
			selected = append(selected, vid)
		}
	}

	vms := make([]VM, len(selected))
	for i, vid := range selected {
		if options.Detail {
			vm, err := v.cli.GetVM(vid.Name)
			if err != nil {
				return []VM{}, err
			}
			err = v.queryVM(&vm, QueryTypeAll)
			if err != nil {
				return []VM{}, err
			}
			vms[i] = vm
		} else {
			vms[i] = VM{Name: vid.Name}
		}
	}
	return vms, nil
}
