package controller

// functions to query host info

import (
	"fmt"

	"encoding/json"
	"strings"
)

const psCmdGetResolution = `Get-CimInstance -ClassName Win32_VideoController | Select-Object CurrentHorizontalResolution, CurrentVerticalResolution | . ConvertTo-Json`

func (v *vmctl) HostResolution() (int, int, error) {

	switch v.Remote {
	case "windows":
		stdout, err := v.RemoteExec("powershell", []string{"-command", psCmdGetResolution}, nil, nil)
		if err != nil {
			return 0, 0, Fatal(err)
		}
		var result struct {
			CurrentHorizontalResolution int
			CurrentVerticalResolution   int
		}
		err = json.Unmarshal([]byte(strings.Join(stdout, "\n")), &result)
		if err != nil {
			return 0, 0, Fatal(err)
		}
		fmt.Printf("resolution: %s\n", FormatJSON(result))
		return result.CurrentHorizontalResolution, result.CurrentVerticalResolution, nil
	}
	return 0, 0, Fatalf("Host OS %s does not support resolution query", v.Remote)
}
