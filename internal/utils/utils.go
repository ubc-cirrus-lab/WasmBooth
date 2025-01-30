package utils

import (
	"os"
	"syscall"
)

func CreateDirectory(directoryPath string) error {
	err := os.Mkdir(directoryPath, os.ModePerm)
	if err != nil {
		return err
	}
	return nil
}

func DeleteDirectory(directoryPath string) error {
	err := os.RemoveAll(directoryPath)
	if err != nil {
		return err
	}
	return nil
}

func CreatePipe(pipePath string) error {
	_ = syscall.Unlink(pipePath)
	_ = syscall.Umask(0)
	syscall.Mkfifo(pipePath, 0666)

	// Open the FIFO file to set permissions explicitly
	// file, err := os.OpenFile(pipePath, os.O_RDONLY, 0)
	// if err != nil {
	// 	return err
	// }
	// defer file.Close()

	// // Set permissions
	// err = file.Chmod(os.FileMode(0666))
	// if err != nil {
	// 	return err
	// }

	return nil
}
