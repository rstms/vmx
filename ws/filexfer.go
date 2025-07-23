package ws

import (
	"fmt"
	"io"
	"log"
	"os"
	"path"
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

	err = v.Download(vm.Name, localPath, filename)
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
	return v.Upload(vm.Name, localPath, filename)
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

func (v *vmctl) Download(vid string, localDestPathname, vmDirFilename string) error {
	if v.debug {
		log.Printf("Download(%s, %s, %s)\n", vid, localDestPathname, vmDirFilename)
	}
	vm, err := v.cli.GetVM(vid)
	if err != nil {
		return err
	}
	dir, _ := path.Split(vm.Path)
	remoteSourcePathname := path.Join(dir, vmDirFilename)
	return v.DownloadFile(&vm, localDestPathname, remoteSourcePathname)
}

func (v *vmctl) DownloadFile(vm *VM, localDestPathname, remoteSourcePathname string) error {
	if v.debug {
		log.Printf("DownloadFile(%s, %s, %s)\n", vm.Name, localDestPathname, remoteSourcePathname)
	}

	localDest, err := PathnameFormat(v.Local, localDestPathname)
	if err != nil {
		return err
	}

	local, err := isLocal()
	if err != nil {
		return err
	}
	if local {
		localSource, err := PathnameFormat(v.Local, remoteSourcePathname)
		if err != nil {
			return err
		}
		return v.copyFile(localDest, localSource)
	}

	sourcePath, err := PathnameFormat("scp", remoteSourcePathname)
	if err != nil {
		return err
	}
	remoteSource := fmt.Sprintf("%s@%s:%s", v.Username, v.Hostname, sourcePath)
	args := []string{"-i", v.KeyFile, remoteSource, localDest}
	_, err = v.exec("scp", args, "", nil)
	return err
}

func (v *vmctl) Upload(vid, localSourcePathname, vmDirFilename string) error {
	if v.debug {
		log.Printf("Upload(%s, %s, %s)\n", vid, localSourcePathname, vmDirFilename)
	}
	vm, err := v.cli.GetVM(vid)
	if err != nil {
		return err
	}
	dir, _ := path.Split(vm.Path)
	remoteDestPathname := path.Join(dir, vmDirFilename)
	return v.UploadFile(&vm, localSourcePathname, remoteDestPathname)
}

func (v *vmctl) UploadFile(vm *VM, localSourcePathname, remoteDestPathname string) error {
	if v.debug {
		log.Printf("UploadFile(%s, %s, %s)\n", vm.Name, localSourcePathname, remoteDestPathname)
	}

	localSource, err := PathnameFormat(v.Local, localSourcePathname)
	if err != nil {
		return err
	}
	local, err := isLocal()
	if err != nil {
		return err
	}
	if local {
		localDest, err := PathnameFormat(v.Local, remoteDestPathname)
		if err != nil {
			return err
		}
		return v.copyFile(localDest, localSource)
	}

	remoteDest, err := PathnameFormat("scp", remoteDestPathname)
	if err != nil {
		return err
	}
	remoteTarget := fmt.Sprintf("%s@%s:%s", v.Username, v.Hostname, remoteDest)
	args := []string{"-i", v.KeyFile, localSource, remoteTarget}
	_, err = v.exec("scp", args, "", nil)
	return err
}
