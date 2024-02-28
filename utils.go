package main

import (
	"bufio"
	"os"
)

func loadPaths(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var paths []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		paths = append(paths, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return paths, nil
}
