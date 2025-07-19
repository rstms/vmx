package workstation

import (
	"fmt"
	"strconv"
	"unicode"
)

const NEWLINE_CODE = 0x28
const NEWLINE_MOD = 0x00

const LF_CODE = 0x0d
const LF_MOD = 0x01

const ESCAPE_CODE = 0x29
const ESCAPE_MOD = 0x00

const DELETE_CODE = 0x2a
const DELETE_MOD = 0x00

const TAB_CODE = 0x2b
const TAB_MOD = 0x00

/*
func dumpKeys() {
	for i := '\x00'; i < '\x80'; i++ {
		if unicode.IsLetter(i) {
			continue
		}
		if unicode.IsDigit(i) {
			continue
		}
		fmt.Printf("%03d %02x '%s'\n", int(i), int(i), strconv.Quote(string(i)))
	}
}
*/

func (v *vmctl) SendKeys(vid, keys string) error {
	var buf string
	vm, err := v.Get(vid)
	if err != nil {
		return err
	}
	fmt.Printf("keys:\n%s\n\n", HexDump([]byte(keys)))
	//fmt.Printf("Quote:\n%s\n\n", HexDump([]byte(strconv.Quote(keys))))
	//unquoted, err := strconv.Unquote("`" + keys + "`")
	//fmt.Printf("Unquote:\n%s\n\n", HexDump([]byte(unquoted)))
	var unquoted string
	for len(keys) > 0 {
		char, multi, tail, err := strconv.UnquoteChar(keys, byte('"'))
		if err != nil {
			return err
		}
		if multi {
			return fmt.Errorf("multibyte encoding not supported: '%s'", keys)
		}
		unquoted += string(char)
		keys = tail
	}
	fmt.Printf("Unquoted:\n%s\n\n", HexDump([]byte(unquoted)))

	if err != nil {
		return err
	}
	for _, key := range unquoted {
		fmt.Printf("key: %02x %s\n", int(key), strconv.Quote(string(key)))

		if unicode.IsLetter(key) || unicode.IsDigit(key) {
			buf += string(key)
		} else {
			if len(buf) > 0 {
				err := v.sendBuf(&vm, buf)
				if err != nil {
					return err
				}
				buf = ""
			}
			hid, ok := HIDMap[key]
			if !ok {
				return fmt.Errorf("cannot encode: %02x %s\n", key, strconv.Quote(string(key)))
			}
			err = v.sendCode(&vm, hid.Code, hid.Modifier)
			if err != nil {
				return err
			}
		}
	}
	if len(buf) > 0 {
		err := v.sendBuf(&vm, buf)
		if err != nil {
			return err
		}
	}
	return nil
}

func (v *vmctl) sendBuf(vm *VM, buf string) error {
	fmt.Printf("sendBuf(%s, '%s')\n", vm.Name, buf)
	path, err := PathFormat(v.Remote, vm.Path)
	if err != nil {
		return err
	}
	_, err = v.RemoteExec(fmt.Sprintf("vmcli %s mks sendKeySequence %s", path, buf), nil)
	if err != nil {
		return err
	}
	return nil
}

func (v *vmctl) sendCode(vm *VM, code, mod uint32) error {
	fmt.Printf("sendCode(%s, %04x, %04x)\n", vm.Name, code, mod)
	path, err := PathFormat(v.Remote, vm.Path)
	if err != nil {
		return err
	}

	code = code<<16 | 0x0007
	_, err = v.RemoteExec(fmt.Sprintf("vmcli %s mks sendKeyEvent %d %d", path, code, mod), nil)
	if err != nil {
		return err
	}
	return nil
}
