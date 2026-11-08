package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"
	"taskwarrior/internal/app"
	"taskwarrior/internal/config"
	"taskwarrior/internal/storage"
)

const version = "0.4.0-milestone4"

func defaultPaths(system, home string) (string, string) {
	var data, config string
	switch system {
	case "darwin":
		data = filepath.Join(home, "Library", "Application Support")
		config = data
	case "windows":
		data = os.Getenv("LOCALAPPDATA")
		config = os.Getenv("APPDATA")
		if data == "" {
			data = filepath.Join(home, "AppData", "Local")
		}
		if config == "" {
			config = filepath.Join(home, "AppData", "Roaming")
		}
	default:
		data = os.Getenv("XDG_DATA_HOME")
		config = os.Getenv("XDG_CONFIG_HOME")
		if data == "" || !filepath.IsAbs(data) {
			data = filepath.Join(home, ".local", "share")
		}
		if config == "" || !filepath.IsAbs(config) {
			config = filepath.Join(home, ".config")
		}
	}
	return filepath.Join(data, "taskwarrior-tui", "tasks.json"), filepath.Join(config, "taskwarrior-tui", "config.toml")
}
func run(args []string, in, out *os.File) error {
	flags := flag.NewFlagSet("taskwarrior", flag.ContinueOnError)
	flags.SetOutput(out)
	dataFlag := flags.String("data", "", "task JSON file (platform default if omitted)")
	configFlag := flags.String("config", "", "TOML configuration file (platform default if omitted)")
	showVersion := flags.Bool("version", false, "show version")
	flags.Usage = func() {
		fmt.Fprintln(out, "Usage: taskwarrior [--data FILE] [--config FILE]\n\nMilestone 4: local tasks, groups, colors, and editable presentation.\nFlags:")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	if *showVersion {
		fmt.Fprintln(out, "taskwarrior", version)
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dataPath, configPath := defaultPaths(runtime.GOOS, home)
	if *dataFlag != "" {
		dataPath = *dataFlag
	}
	if *configFlag != "" {
		configPath = *configFlag
	}
	dataPath, err = storage.ResolvePath(dataPath)
	if err != nil {
		return err
	}
	configPath, err = storage.ResolvePath(configPath)
	if err != nil {
		return err
	}
	if !term.IsTerminal(in.Fd()) || !term.IsTerminal(out.Fd()) {
		return errors.New("the TUI requires interactive terminal input and output")
	}
	release, err := storage.LockPaths(dataPath, configPath)
	if err != nil {
		return err
	}
	defer release()
	data, err := storage.Load(dataPath)
	if err != nil {
		return err
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	model := app.NewConfigured(data, dataPath, configPath, cfg)
	final, err := tea.NewProgram(model, tea.WithInput(in), tea.WithOutput(out)).Run()
	if err != nil {
		return err
	}
	if finalModel, ok := final.(*app.Model); ok && finalModel.LastError != nil {
		return finalModel.LastError
	}
	return nil
}
func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "taskwarrior:", err)
		os.Exit(1)
	}
}
