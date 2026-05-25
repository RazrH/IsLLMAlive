package config

import (
	"log"
	"os"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watch starts monitoring the specific config file for changes.
func Watch(configPath string, onEvent func()) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	err = watcher.Add(configPath)
	if err != nil {
		return err
	}

	go func() {
		defer watcher.Close()
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				// If the editor saves by deleting and replacing the file (atomic save),
				// the watch on the original file is lost.
				if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
					go func() {
						// Poll until the file is recreated
						for i := 0; i < 50; i++ {
							time.Sleep(100 * time.Millisecond)
							if _, err := os.Stat(configPath); err == nil {
								// File exists again, re-add to watcher
								_ = watcher.Add(configPath)
								// Small delay to ensure file write finishes
								time.Sleep(100 * time.Millisecond)
								onEvent()
								return
							}
						}
						log.Println("Watcher lost config file permanently")
					}()
					continue
				}

				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
					// Small debounce delay
					time.Sleep(100 * time.Millisecond)
					onEvent()
				}

			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("watcher error:", err)
			}
		}
	}()

	return nil
}
