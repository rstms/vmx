package ws

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func (v *vmctl) ReadHostFile(vm *VM, filename string) ([]byte, error) {
	if v.debug {
		log.Printf("ReadHostFile(%s, %s)\n", vm.Name, filename)
	}
	tempFile, err := os.CreateTemp("", "vmx_read.*")
	if err != nil {
		return []byte{}, err
	}
	localPath := tempFile.Name()
	err = tempFile.Close()
	if err != nil {
		return []byte{}, err
	}
	defer os.Remove(localPath)

	err = v.DownloadFile(vm, localPath, filename)
	if err != nil {
		return []byte{}, err
	}
	return os.ReadFile(localPath)
}

func (v *vmctl) WriteHostFile(vm *VM, filename string, data []byte) error {
	if v.debug {
		log.Printf("WriteHostFile(%s, %s, (%d bytes))\n", vm.Name, filename, len(data))
	}
	tempFile, err := os.CreateTemp("", "vmx_write.*")
	if err != nil {
		return err
	}
	localPath := tempFile.Name()
	err = tempFile.Close()
	if err != nil {
		return err
	}
	defer os.Remove(localPath)
	err = os.WriteFile(localPath, data, 0600)
	if err != nil {
		return err
	}
	return v.UploadFile(vm, localPath, filename)
}

func (v *vmctl) copyFile(dstPath, srcPath string) error {
	if v.debug {
		log.Printf("copyFile(%s, %s)\n", dstPath, srcPath)
	}

	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dst.Close()
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()
	_, err = io.Copy(dst, src)
	return err

}

func (v *vmctl) Download(vid, localPath, filename string) error {
	if v.debug {
		log.Printf("Download(%s, %s, %s)\n", vid, localPath, filename)
	}
	vm, err := v.cli.GetVM(vid)
	if err != nil {
		return err
	}
	return v.DownloadFile(&vm, localPath, filename)
}

func (v *vmctl) DownloadFile(vm *VM, localPath, filename string) error {
	if v.debug {
		log.Printf("DownloadFile(%s, %s, %s)\n", vm.Name, localPath, filename)
	}

	if strings.ContainsAny(filename, ":/\\") {
		return fmt.Errorf("invalid characters in '%s'", filename)
	}

	hostDir, _ := filepath.Split(vm.Path)
	hostPath := filepath.Join(hostDir, filename)

	local, err := isLocal()
	if err != nil {

		return err
	}
	if local {
		hostPath, err := PathnameFormat(v.Local, hostPath)
		if err != nil {
			return err
		}
		return v.copyFile(localPath, hostPath)
	}

	path, err := PathnameFormat("scp", hostPath)
	if err != nil {
		return err
	}
	remoteSource := fmt.Sprintf("%s@%s:%s", v.Username, v.Hostname, path)
	args := []string{"-i", v.KeyFile, remoteSource, localPath}
	_, err = v.exec("scp", args, "", nil)
	return err
}

func (v *vmctl) Upload(vid, localPath, filename string) error {
	if v.debug {
		log.Printf("Upload(%s, %s, %s)\n", vid, localPath, filename)
	}
	vm, err := v.cli.GetVM(vid)
	if err != nil {
		return err
	}
	return v.UploadFile(&vm, localPath, filename)
}

func (v *vmctl) UploadFile(vm *VM, localPath, filename string) error {
	if v.debug {
		log.Printf("UploadFile(%s, %s, %s)\n", vm.Name, localPath, filename)
	}
	if strings.ContainsAny(filename, ":/\\") {
		return fmt.Errorf("invalid characters in '%s'", filename)
	}
	hostDir, _ := filepath.Split(vm.Path)
	hostPath := filepath.Join(hostDir, filename)
	local, err := isLocal()
	if err != nil {
		return err
	}
	if local {
		hostPath, err := PathnameFormat(v.Local, hostPath)
		if err != nil {
			return err
		}
		return v.copyFile(hostPath, localPath)
	}

	path, err := PathnameFormat("scp", hostPath)
	if err != nil {
		return err
	}
	remoteTarget := fmt.Sprintf("%s@%s:%s", v.Username, v.Hostname, path)
	args := []string{"-i", v.KeyFile, localPath, remoteTarget}
	_, err = v.exec("scp", args, "", nil)
	return err
}
