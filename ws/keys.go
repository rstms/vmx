package ws

import (
	"fmt"
	"log"
	"strconv"
	"unicode"
)

func (v *vmctl) SendKeys(vid, keys string) error {
	var buf string
	vm, err := v.Get(vid)
	if err != nil {
		return Fatal(err)
	}
	if v.debug {
		log.Printf("keys:\n%s\n\n", HexDump([]byte(keys)))
	}
	var unquoted string
	for len(keys) > 0 {
		char, multi, tail, err := strconv.UnquoteChar(keys, byte('"'))
		if err != nil {
			return Fatal(err)
		}
		if multi {
			return Fatalf("multibyte encoding not supported: '%s'", keys)
		}
		unquoted += string(char)
		keys = tail
	}
	if v.debug {
		log.Printf("Unquoted:\n%s\n\n", HexDump([]byte(unquoted)))
	}

	if err != nil {
		return Fatal(err)
	}
	for _, key := range unquoted {

		//log.Printf("key: %02x %s\n", int(key), strconv.Quote(string(key)))

		if unicode.IsLetter(key) || unicode.IsDigit(key) {
			buf += string(key)
		} else {
			if len(buf) > 0 {
				err := v.sendBuf(&vm, buf)
				if err != nil {
					return Fatal(err)
				}
				buf = ""
			}
			hid, ok := HIDMap[key]
			if !ok {
				return Fatalf("cannot encode: %02x %s\n", key, strconv.Quote(string(key)))
			}
			err = v.sendCode(&vm, hid.Code, hid.Modifier)
			if err != nil {
				return Fatal(err)
			}
		}
	}
	if len(buf) > 0 {
		err := v.sendBuf(&vm, buf)
		if err != nil {
			return Fatal(err)
		}
	}
	return nil
}

func (v *vmctl) sendBuf(vm *VM, buf string) error {
	if v.debug {
		fmt.Printf("sendBuf(%s, '%s')\n", vm.Name, buf)
	}
	path, err := PathnameFormat(v.Remote, vm.Path)
	if err != nil {
		return Fatal(err)
	}
	_, err = v.RemoteExec(fmt.Sprintf("vmcli %s mks sendKeySequence %s", path, buf), nil)
	if err != nil {
		return Fatal(err)
	}
	return nil
}

func (v *vmctl) sendCode(vm *VM, code, mod uint32) error {
	if v.debug {
		fmt.Printf("sendCode(%s, %04x, %04x)\n", vm.Name, code, mod)
	}
	path, err := PathnameFormat(v.Remote, vm.Path)
	if err != nil {
		return Fatal(err)
	}

	code = code<<16 | 0x0007
	_, err = v.RemoteExec(fmt.Sprintf("vmcli %s mks sendKeyEvent %d %d", path, code, mod), nil)
	if err != nil {
		return Fatal(err)
	}
	return nil
}
