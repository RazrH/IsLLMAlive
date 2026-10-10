package diagnostics

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Sample writes private local JSONL diagnostics without forcing GC or opening a server.
func Sample(ctx context.Context, path string, period time.Duration, snapshot func() string) error {
	if path == "" {
		return nil
	}
	if period <= 0 {
		return fmt.Errorf("sampling interval must be positive")
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	ticker := time.NewTicker(period)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return nil
		}
		record := struct {
			Time        time.Time `json:"time"`
			Diagnostics string    `json:"diagnostics"`
		}{time.Now().UTC(), snapshot()}
		if err := encoder.Encode(record); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
