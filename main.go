package main

import (
	"fmt"
	"log"
	"os"
	"path"

	"encoding/json"
	"io/ioutil"

	"golang.org/x/exp/inotify"
)

type Config struct {
	ReplaysFolder  string
	MaxQueueSize   int
	DebugMode      bool
	DeleteOldFiles bool
	MaxTries       int
	RequestUrl     string
}

type FilePending struct {
	Path string
	Name string
}

type FileFinished struct {
	Path  string
	Name  string
	Stats *MatchStats
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Expected a .json config file as argument")
	}

	conf := readConfig(os.Args[1])
	dirName := conf.ReplaysFolder

	InitDebugMode(conf.DebugMode)
	DebugPrint("watching directory %s\n", dirName)

	// set up queue and start the parsing in a goroutine
	// I think this is thread-safe
	parseQueue := make(chan *FilePending, conf.MaxQueueSize)
	finishedQueue := make(chan *FileFinished, conf.MaxQueueSize)

	go Parse(parseQueue, finishedQueue)
	go ReportAndClean(finishedQueue, conf)

	// check the directory for any existing .dem files
	dirEntries, err := ioutil.ReadDir(dirName)
	if err != nil {
		log.Fatal(err)
	}

	for _, fileInfo := range dirEntries {
		handleFile(path.Join(dirName, fileInfo.Name()), parseQueue)
	}

	// watch the directory for file writes, which should only happen
	// when a new file is created
	watcher := inotifySetup(dirName)

	for {
		select {
		case ev := <-watcher.Event:
			if ev.Mask&inotify.IN_CLOSE_WRITE > 0 {
				handleFile(ev.Name, parseQueue)
			}
		case err := <-watcher.Error:
			log.Println("inotify error:", err)
		}
	}
}

func readConfig(fname string) *Config {
	file, _ := os.Open(fname)
	decoder := json.NewDecoder(file)
	configuration := Config{}
	err := decoder.Decode(&configuration)
	if err != nil {
		fmt.Println("error:", err)
	}

	return &configuration
}

func inotifySetup(dir string) *inotify.Watcher {
	watcher, err := inotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	err = watcher.Watch(dir)
	if err != nil {
		log.Fatal(err)
	}

	return watcher
}

func handleFile(fname string, parseQueue chan<- *FilePending) {
	basename := path.Base(fname)
	ext := path.Ext(fname)

	if ext == ".dem" {
		parseQueue <- &FilePending{Path: path.Clean(fname), Name: basename}
	}
}
