package utils

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"os"
)

func GetFiles(path string) ([]string, error) {
	// Get files from path
	files, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var filesList []string
	for _, file := range files {
		filesList = append(filesList, file.Name())
	}
	return filesList, nil
}

func GetFileSize(path string) (int64, error) {
	// Get file info
	file, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return file.Size(), nil
}

func GetMD5(path string) (string, error) {
	// Open file
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = file.Close()
	}()
	// Calculate MD5 hash
	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	// Convert hash to string
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func GetDate(path string) string {
	// Get file info
	file, err := os.Stat(path)
	if err != nil {
		return ""
	}
	return file.ModTime().Format("2006-01-02 15:04:05")
}
