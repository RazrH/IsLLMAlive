package notify

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestNotificationTextIsData(t *testing.T) {
	title := "test \" \\ $()\n]]> 中文"
	body := "'; throw 'injection'; #\n$(Start-Process calc) ]]>"
	for _, platform := range []string{"windows", "darwin", "linux"} {
		cmd, err := command(t.Context(), platform, title, body)
		if err != nil {
			t.Fatal(err)
		}
		if platform == "windows" {
			for _, a := range cmd.Args {
				if strings.Contains(a, body) || strings.Contains(a, title) {
					t.Fatal("text reached command source")
				}
			}
			data, err := io.ReadAll(cmd.Stdin)
			if err != nil {
				t.Fatal(err)
			}
			var payload map[string]string
			if err := json.Unmarshal(data, &payload); err != nil {
				t.Fatal(err)
			}
			if payload["title"] != title || payload["message"] != body {
				t.Fatal("data changed")
			}
		} else if cmd.Args[len(cmd.Args)-2] != title || cmd.Args[len(cmd.Args)-1] != body {
			t.Fatal("arguments changed")
		}
	}
}
func TestQueueReplacesPendingAndCancels(t *testing.T) {
	entered := make(chan string, 3)
	release := make(chan struct{})
	q := NewQueue(t.Context(), func(ctx context.Context, title, body string) error {
		entered <- body
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	t.Cleanup(q.Stop)
	q.Enqueue(Message{Key: "a", Body: "first"})
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("worker did not start")
	}
	q.Enqueue(Message{Key: "a", Body: "old"})
	q.Enqueue(Message{Key: "a", Body: "latest"})
	release <- struct{}{}
	select {
	case got := <-entered:
		if got != "latest" {
			t.Fatalf("got %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("missing latest")
	}
	stopped := make(chan struct{})
	go func() { q.Stop(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("stop did not cancel")
	}
	q.Stop()
	if q.LastError() == "" {
		t.Fatal("cancellation not diagnosed")
	}
	q.Enqueue(Message{Key: "a", Body: "after stop"})
	select {
	case <-entered:
		t.Fatal("sent after stop")
	default:
	}
}
func TestQueueErrorIsRecorded(t *testing.T) {
	q := NewQueue(t.Context(), func(context.Context, string, string) error { return errors.New("failed") })
	defer q.Stop()
	q.Enqueue(Message{Key: "a"})
	deadline := time.After(time.Second)
	for q.LastError() == "" {
		select {
		case <-deadline:
			t.Fatal("no error")
		case <-time.After(time.Millisecond):
		}
	}
	if !strings.Contains(q.LastError(), "failed") {
		t.Fatal(q.LastError())
	}
}
