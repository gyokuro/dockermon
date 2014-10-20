package main

import (
	"flag"
	"fmt"
	"github.com/golang/glog"
	"github.com/gyokuro/dockermon/pkg/filewatch"
	"os"
	"path/filepath"
)

var (
	// get ulimit information
	rlimit               = new(filewatch.Rlimit)
	currentWorkingDir, _ = os.Getwd()

	dirToWatch        = flag.String("d", currentWorkingDir, "Name of directory to watch")
	maxDirsPerWatcher = flag.Uint64("m", 0, "Max directories per watcher, use ulimit -n to find out")
)

func main() {
	err := filewatch.Getrlimit(rlimit)
	if err == nil {
		fmt.Println("Current system's limits files open per process to ", rlimit.Max, rlimit.Cur)
	} else {
		panic(err)
	}

	flag.Parse()

	id := *dirToWatch

	// Signal for shutdown
	done := make(chan bool)

	// check against rlimit
	if *maxDirsPerWatcher > uint64(rlimit.Cur) || *maxDirsPerWatcher == 0 {
		glog.Warningf("%s - Trying to watch more directories than allowed: %d vs %d. max=%d",
			id, *maxDirsPerWatcher, rlimit.Cur, rlimit.Max)
		// reset it
		*maxDirsPerWatcher = uint64(float32(rlimit.Cur) * 0.8)
		glog.Infof("%s - Setting max directories to watch to %d", id, *maxDirsPerWatcher)
	}

	dir := filepath.FromSlash(*dirToWatch)
	targets, _, _ := filewatch.RecurseDir(dir)
	targets = append(targets, *dirToWatch)

	watcher := filewatch.NewWatcher(id, targets, *maxDirsPerWatcher)
	glog.Infof("%s -- Starting file system watcher: watching %d, not watching %d",
		id, len(watcher.Watching), len(watcher.NotWatching))

	channel := filewatch.NewChannel(id)
	defer filewatch.CloseChannel(id)

	go func() {
		for {
			select {
			case event := <-channel:
				glog.Infoln("Got event", event)
			}
		}
	}()

	watcher.Start()
	<-done // blocks
}
