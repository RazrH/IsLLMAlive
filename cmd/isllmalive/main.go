package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"isllmalive/internal/app"
	"isllmalive/internal/config"
	"isllmalive/internal/tray"
)

var (
	appConfig  *config.Config
	configPath string
	mainApp    *app.App
)

func main() {
	fmt.Println("Starting IsLLMAlive Phase 4...")

	var err error
	appConfig, err = config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	exePath, _ := os.Executable()
	configPath = filepath.Join(filepath.Dir(exePath), "config.json")
	fmt.Printf("Loaded config from: %s\n", configPath)

	mainApp = app.New(appConfig)

	tray.Init(onReady, onExit)
}

func onReady() {
	tray.SetupMenu(mainApp.PollAll, mainApp.ToggleNotify, onConfig, onDiagnostics, onExit)

	fmt.Println("Tray UI initialized, starting polling...")
	mainApp.Start()

	// Start watching config.json for hot reloads
	err := config.Watch(configPath, func() {
		fmt.Println("Config file change detected. Reloading...")
		mainApp.ReloadConfig()
	})
	if err != nil {
		fmt.Printf("Warning: Failed to start config watcher: %v\n", err)
	}
}

func onConfig() {
	fmt.Println("Opening config:", configPath)
	openFile(configPath)
}

func onDiagnostics() {
	exePath, err := os.Executable()
	if err != nil {
		fmt.Printf("Failed to resolve executable path for diagnostics: %v\n", err)
		return
	}

	diagnosticsPath := filepath.Join(filepath.Dir(exePath), "diagnostics.txt")
	if err := os.WriteFile(diagnosticsPath, []byte(mainApp.Diagnostics()), 0644); err != nil {
		fmt.Printf("Failed to write diagnostics: %v\n", err)
		return
	}

	fmt.Println("Opening diagnostics:", diagnosticsPath)
	openFile(diagnosticsPath)
}

func openFile(path string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", path)
	case "windows":
		// Windows 'start' command interprets the first quoted string as a window title.
		// By passing an empty string "", we prevent paths with spaces from opening a blank cmd.
		cmd = exec.Command("cmd", "/c", "start", "", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("cmd", "/c", "start", "", path)
	}
	_ = cmd.Start()
}

func onExit() {
	fmt.Println("Exiting...")
}
