package config

import (
	"context"
	"github.com/fsnotify/fsnotify"
	"path/filepath"
	"sync"
	"time"
)

// Watch observes the parent directory so atomic replace saves do not lose the watch.
func Watch(parent context.Context, path string, onEvent func(), onError func(error)) (func(), error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err := watcher.Add(filepath.Dir(path)); err != nil {
		watcher.Close()
		return nil, err
	}
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer watcher.Close()
		timer := time.NewTimer(time.Hour)
		timer.Stop()
		defer timer.Stop()
		var ready <-chan time.Time
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if filepath.Clean(event.Name) != path {
					continue
				}
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
					timer.Reset(100 * time.Millisecond)
					ready = timer.C
				}
			case <-ready:
				ready = nil
				onEvent()
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				if onError != nil {
					onError(err)
				}
			}
		}
	}()
	var once sync.Once
	stop := func() { once.Do(cancel); <-done }
	return stop, nil
}
