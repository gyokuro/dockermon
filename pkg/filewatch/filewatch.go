package filewatch

import (
	"flag"
	"fmt"
	"github.com/golang/glog"
	"github.com/howeyc/fsnotify"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

// If object fires event more than one in this period, fire only one.
var eventThreshold = flag.Int64("filewatch.threshold", 3, "Event threshold in seconds.")

// Basic filewatcher structure.  This serves as a handle
// with a private field that retains the reference to the
// fsnotify watcher object.
type FileWatcher struct {
	Id           string
	watcher      *fsnotify.Watcher
	Watching     []string // actual directories watched
	NotWatching  []string
	TotalObjects uint64
}

// Event types
// This object can be marshaled into JSON for websocket clients
type FileChanged struct {
	Event     string `json:"event"`
	Timestamp int64  `json:"timestamp"`
	Path      string `json:"path,omitempty"`
	IsCreate  bool   `json:"create,omitempty"`
	IsModify  bool   `json:"modify,omitempty"`
	IsRename  bool   `json:"rename,omitempty"`
	IsDelete  bool   `json:"deleted,omitempty"`
}

// Synchronization and management of filesystem notification events
// channels and websocket connections
var allChannelsMutex = sync.Mutex{}
var channelsToClose = []chan *FileChanged{}

// Map of filesystem channels keyed by some object
var allChannelsMap = make(map[interface{}]chan *FileChanged)

// singletons to store the previous event information
var lastEventTs = time.Now().Unix()
var lastEventString = ""

// Returns a list of subdirectories and true if this path is a directory
// If the path is a regular file, then return empty list and false.
func RecurseDir(path string) ([]string, bool, uint64) {
	var result = make([]string, 0)
	var total uint64 = 1
	stat, err := os.Stat(path)
	if err == nil {
		if !stat.IsDir() {
			glog.Infof("%s not a directory.", path)
			return result, false, total
		} else {
			f, err := os.Open(path)
			if err == nil {
				files, err := f.Readdir(-1)
				total += uint64(len(files))
				defer f.Close()
				if err == nil {
					for _, file := range files {
						if file.IsDir() {
							child := path + "/" + file.Name()
							result = append(result, child)
							subdirs, isDir, subCount := RecurseDir(child)
							if isDir {
								for _, c := range subdirs {
									result = append(result, c)
								}
							}
							total += subCount
						}
					}
					return result, true, total
				}
			}
		}
	}
	return result, false, total
}

func watchDir(id string, watcher *fsnotify.Watcher, path string, total *uint64, max uint64) error {
	if *total >= max {
		return fmt.Errorf("limit exceeded: %d", max)
	}
	files, err := ioutil.ReadDir(path)
	if err != nil {
		glog.Warningf("%s - Error reading dir %s: %s: total=%d, max=%d", id, path, err, *total, max)
		// here we don't know how many files are in the folder, but we know there are
		// enough to cause the limit to hit.  So update the total to be max
		//
		// TODO: caller should do a check on the error type and determine if we hit the limit
		*total = max
		return err
	}

	openFiles := uint64(len(files)) + *total
	if openFiles < max {
		err = watcher.Watch(path)
		if err == nil {
			*total = openFiles // update the total count
			glog.Infof("%s - Watching dir %s with %d objects (total=%d)", id, path, len(files), openFiles)
			return err
		} else {
			glog.Warningf("%s - Error on dir %s with %d objects (total=%d): %s", id, path, len(files), total, err)
			return err
		}
	}
	glog.V(20).Infof("%s - Limit exceeded for dir %s", id, path)
	return fmt.Errorf("limit exceeded: %d vs %d", openFiles, max)
}

func concat(a []string, b []string) []string {
	c := make([]string, len(a)+len(b))
	copy(c, a)
	copy(c[len(a):], b)
	return c
}

// Creates a new watcher given the list of directories to watch
func NewWatcher(id string, shouldWatch []string, maxLimit uint64) *FileWatcher {
	var total uint64 = 0
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		glog.Warning("Error while creating watcher")
		return nil
	}
	handle := &FileWatcher{
		Id:          id,
		watcher:     watcher,
		Watching:    []string{},
		NotWatching: []string{},
	}

	// Now add the directories
	for _, dir := range shouldWatch {
		err = watchDir(id, watcher, dir, &total, maxLimit)
		if err == nil {
			handle.Watching = append(handle.Watching, dir)
		} else {
			handle.NotWatching = append(handle.NotWatching, dir)
		}
	}
	return handle
}

// Allocate a new channel for listening for filesystem events.
func NewChannel(key interface{}) chan *FileChanged {
	newChannel := make(chan *FileChanged)
	allChannelsMutex.Lock()
	allChannelsMap[key] = newChannel
	allChannelsMutex.Unlock()
	return newChannel
}

// Close the existing channel
func CloseChannel(key interface{}) {
	channelToClose, exists := allChannelsMap[key]
	if exists {
		allChannelsMutex.Lock()
		delete(allChannelsMap, key)
		channelsToClose = append(channelsToClose, channelToClose)
		allChannelsMutex.Unlock()
	}
}

// Blocks for any events coming from the channel and returns an object that
// can be marshaled into JSON
func BlockForEvent(channel chan *FileChanged, namespace string, match string) (json *FileChanged, log string) {
	select {
	case event := <-channel:
		glog.Infoln(">>", event)
		// see if the event matches
		matched, _ := regexp.MatchString(namespace, "fsnotify")
		if matched {
			// event type matches now match filepath
			matched, _ = regexp.MatchString(match, event.Event)
			if matched {
				log = fmt.Sprintf("%s:%d %s", event.Event, event.Timestamp, event.Path)
				json = event
				return
			}
		} else {
			glog.Infof("Event %s did not match (%s). Skipped.", event.Event, match)
		}
	}
	return
}

// Function invoked when a filesystem event occurs
func onFileWatchEvent(watcher *fsnotify.Watcher, event *fsnotify.FileEvent) {
	ts := time.Now().Unix() // timestamp
	if event.IsCreate() {
		// check the filetype
		s, err := os.Stat(event.Name)
		if err == nil {
			if s.IsDir() {
				glog.Infof("Watching new directory %s", event.Name)
				watcher.Watch(event.Name)
				// also in case a mkdir -p new/dir/children was used:
				subfolders, isDir, _ := RecurseDir(event.Name)
				if isDir {
					for _, dir := range subfolders {
						glog.Infof("Adding new directory %s", dir)
						watcher.Watch(dir)
					}
				}
			}
		}
	}

	glog.V(30).Infoln(lastEventTs, ts, lastEventString, event.String())

	// Remove duplication - sometimes multiple events are fired
	// within a second, for a same path and operation.
	// do a simple filter to avoid sending out duplicates -- this basically
	// makes the event resolution at the per-second level.
	if (ts-lastEventTs) > *eventThreshold || lastEventString != event.String() {
		// broadcast
		glog.V(10).Infoln(ts, event.String())
		for _, channel := range allChannelsMap {
			absName, _ := filepath.Abs(event.Name)
			p := filepath.FromSlash(absName)
			json := &FileChanged{
				Event:     "fsnotify",
				Timestamp: time.Now().UTC().Unix(),
				Path:      p,
				IsCreate:  event.IsCreate(),
				IsModify:  event.IsModify(),
				IsRename:  event.IsRename(),
				IsDelete:  event.IsDelete(),
			}
			channel <- json
		}
	}
	lastEventTs = ts
	lastEventString = event.String()

	// do clean up.  generally we should close the channel from the sender
	for _, channel := range channelsToClose {
		glog.Infof("Closing channel %s", channel)
		close(channel)
	}
}

// Run the watcher asynchronously.  This call returns immediately after
// a closure is scheduled and run.
func (handle *FileWatcher) Start() {
	go func() {
		for {
			select {
			case ev := <-handle.watcher.Event:
				onFileWatchEvent(handle.watcher, ev)
			case err := <-handle.watcher.Error:
				glog.Infoln("error:", err)
			}
		}
	}()
}

// Stops the filewatcher
func (handle *FileWatcher) Stop() {
	handle.watcher.Close()
}
