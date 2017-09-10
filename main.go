// Package f searches a string in a directory
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var recursive = flag.Bool("r", false, "search directories recursively")

func main() {
	// Determine the initial directories.
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		fmt.Println("no arguments")
		return
	}
	query := []byte(args[0])
	var roots []string
	if len(args) == 1 {
		roots = []string{"."}
	} else {
		roots = args[1:]
	}

	started := time.Now()
	// Traverse each root of the file tree in parallel.
	results := make(chan string)
	count := make(chan struct{})
	var n sync.WaitGroup
	for _, root := range roots {
		n.Add(1)
		go walkDir(query, root, &n, results, count)
	}
	go func() {
		n.Wait()
		close(results)
	}()
	var nmatches, nfiles int64
loop:
	for {
		select {
		case <-count:
			nfiles++
		case result, ok := <-results:
			if !ok {
				break loop // results was closed
			}
			fmt.Println(result)
			nmatches++
		}
	}
	elapsed := time.Since(started)
	fmt.Printf("Found %d matches in %d files\n", nmatches, nfiles)
	fmt.Printf("Took %s\n", elapsed)
}

// walkDir recursively walks the file tree rooted at dir
// and sends the matches on results.
func walkDir(query []byte, dir string, n *sync.WaitGroup, results chan<- string, count chan<- struct{}) {
	defer n.Done()
	for _, entry := range dirents(dir) {
		name := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			if *recursive {
				n.Add(1)
				go walkDir(query, name, n, results, count)
			}
		} else {
			count <- struct{}{}
			if checkFile(query, name) {
				results <- name
			}
		}
	}
}

// dirsSema is a counting semaphore for limiting concurrency in dirents.
var dirsSema = make(chan struct{}, 20)

// dirents returns the entries of directory dir.
func dirents(dir string) []os.FileInfo {
	dirsSema <- struct{}{}        // adquire token
	defer func() { <-dirsSema }() // release token
	entries, err := ioutil.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "f: %\n", err)
		return nil
	}
	return entries
}

// filesSema is a counting semaphore for limiting concurrency in checkFile.
var filesSema = make(chan struct{}, 180)

// checkFile reads a file and checks if a query is present in the file content.
func checkFile(query []byte, name string) bool {
	filesSema <- struct{}{}        // adquire token
	defer func() { <-filesSema }() // release token
	b, err := ioutil.ReadFile(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "f: %\n", err)
		return false
	}
	return bytes.Contains(b, query)
}
