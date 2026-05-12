package main

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/laguilar-io/devbrowser/internal/browser"
	"github.com/laguilar-io/devbrowser/internal/config"
	"github.com/laguilar-io/devbrowser/internal/deps"
	"github.com/laguilar-io/devbrowser/internal/envfiles"
	"github.com/laguilar-io/devbrowser/internal/port"
	"github.com/laguilar-io/devbrowser/internal/server"
	"github.com/laguilar-io/devbrowser/internal/state"
	"github.com/laguilar-io/devbrowser/internal/worktree"
)

var (
	flagPort    int
	flagCommand string
)

var stdinScanner = bufio.NewScanner(os.Stdin)

func readLine() string {
	lineCh := make(chan string, 1)
	go func() {
		if stdinScanner.Scan() {
			lineCh <- strings.TrimSpace(strings.ToLower(stdinScanner.Text()))
		} else {
			lineCh <- ""
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	select {
	case line := <-lineCh:
		return line
	case <-sigCh:
		fmt.Println()
		os.Exit(0)
		return ""
	}
}

var runCmd = &cobra.Command{
	Use:     "run [worktree]",
	Short:   "Start a dev server and open an isolated Chrome session",
	Args:    cobra.MaximumNArgs(1),
	Aliases: []string{"r"},
	RunE:    runRun,
}

func init() {
	runCmd.Flags().IntVarP(&flagPort, "port", "p", 0, "Port override (default: auto-detect)")
	runCmd.Flags().StringVarP(&flagCommand, "command", "c", "", "Dev server command override")

	rootCmd.RunE = runRun
	rootCmd.Args = cobra.MaximumNArgs(1)
	rootCmd.Flags().IntVarP(&flagPort, "port", "p", 0, "Port override")
	rootCmd.Flags().StringVarP(&flagCommand, "command", "c", "", "Dev server command override")
}

// shortenPath replaces the home directory prefix with ~
func shortenPath(p string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	if strings.HasPrefix(p, home) {
		return "~" + p[len(home):]
	}
	return p
}

func clearScreen() {
	fmt.Print("\033[H\033[2J")
}

type sessionInfo struct {
	worktreeName string
	worktreeDir  string
	devCmd       string
	p            int
	profileDir   string
	url          string
	envCopied    []string
}

func printSessionInfo(info sessionInfo) {
	bold := color.New(color.Bold).SprintFunc()
	cyan := color.CyanString

	fmt.Println()
	fmt.Printf("  %s\n", bold("devbrowser"))
	fmt.Println()

	rows := [][]string{
		{"worktree", shortenPath(info.worktreeDir)},
		{"command ", info.devCmd},
		{"port    ", fmt.Sprintf("%d", info.p)},
		{"url     ", info.url},
		{"profile ", shortenPath(info.profileDir)},
	}
	for _, f := range info.envCopied {
		rows = append(rows, []string{"env     ", filepath.Base(f)})
	}

	for _, row := range rows {
		fmt.Printf("  %s  %s\n", cyan(row[0]), row[1])
	}
	fmt.Println()
}

func runRun(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Attach-only mode: explicit -p with no worktree arg and no command
	// Just open Chrome pointed at the running server, no server management.
	if flagPort != 0 && len(args) == 0 && flagCommand == "" {
		return attachOnly(cfg, flagPort)
	}

	worktreeDir := "."
	worktreeName := ""
	repoRoot := ""

	if len(args) == 1 {
		wt, err := worktree.FindByName(args[0])
		if err != nil {
			return err
		}
		worktreeDir = wt.Path
		projectName := filepath.Base(filepath.Dir(filepath.Dir(wt.Path)))
		worktreeName = projectName + "__" + wt.Name
		wts, _ := worktree.List()
		if len(wts) > 0 {
			repoRoot = wts[0].Path
		}
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		worktreeDir = cwd
		worktreeName = filepath.Base(filepath.Dir(cwd)) + "__" + filepath.Base(cwd)
		repoRoot = cwd
	}

	p := flagPort
	if p == 0 {
		p, err = port.FindAvailable(cfg.StartPort)
		if err != nil {
			return err
		}
	}

	devCmd := flagCommand
	if devCmd == "" {
		devCmd = cfg.DefaultCommand
	}

	existing, _ := state.Get(worktreeName)
	if existing != nil {
		if isServerAlive(existing.ServerPID) {
			// Server is still running — just reattach Chrome with the correct profile
			fmt.Printf("  %s  existing session on port %d — attaching...\n\n",
				color.YellowString("found    "), existing.Port)
			existingURL := fmt.Sprintf("http://localhost:%d", existing.Port)
			existingProfileDir := filepath.Join(func() string {
				if cfg.ProfilesDir != "" {
					return cfg.ProfilesDir
				}
				d, _ := config.ProfilesDir()
				return d
			}(), worktreeName)
			return attachToSession(cfg, worktreeName, existing, existingProfileDir, existingURL)
		}
		// Stale entry — clean up and start fresh
		fmt.Printf("  %s  stale session (PID %d dead) — starting fresh\n\n",
			color.YellowString("cleanup  "), existing.ServerPID)
		_ = state.Remove(worktreeName)
	}

	chromeBin, err := browser.Find(cfg.BrowserPath)
	if err != nil {
		return err
	}

	profilesDir := cfg.ProfilesDir
	if profilesDir == "" {
		profilesDir, err = config.ProfilesDir()
		if err != nil {
			return err
		}
	}
	profileDir := filepath.Join(profilesDir, worktreeName)
	url := fmt.Sprintf("http://localhost:%d", p)

	// Symlink node_modules from repo root if missing in worktree
	if depsMsg, depsErr := deps.EnsureNodeModules(repoRoot, worktreeDir); depsErr != nil {
		fmt.Printf("⚠️  Could not symlink node_modules: %v\n", depsErr)
	} else if depsMsg != "" {
		fmt.Printf("  %s  %s\n", color.CyanString("deps    "), depsMsg)
	}

	// Copy .env*.local from repo root to worktree
	var envResult *envfiles.CopyResult
	if repoRoot != "" && repoRoot != worktreeDir {
		envResult, err = envfiles.CopyToWorktree(repoRoot, worktreeDir)
		if err != nil {
			fmt.Printf("⚠️  Could not copy env files: %v\n", err)
		}
	}

	info := sessionInfo{
		worktreeName: worktreeName,
		worktreeDir:  worktreeDir,
		devCmd:       devCmd,
		p:            p,
		profileDir:   profileDir,
		url:          url,
	}
	if envResult != nil {
		info.envCopied = envResult.Copied
	}

	clearScreen()
	printSessionInfo(info)

	// Start dev server
	srv, err := server.Start(worktreeDir, devCmd, p)
	if err != nil {
		return err
	}

	entry := &state.Entry{
		WorktreePath: worktreeDir,
		Port:         p,
		ServerPID:    srv.PID,
		ServerPGID:   srv.PGID,
		Command:      devCmd,
		StartedAt:    time.Now(),
	}
	_ = state.Add(worktreeName, entry)

	cleanup := func() {
		srv.Stop()
		_ = state.Remove(worktreeName)
		if envResult != nil {
			envResult.Cleanup()
		}
	}

	fmt.Printf("⏳  Waiting for localhost:%d...\n", p)

	readyCh := make(chan error, 1)
	go func() { readyCh <- server.WaitReady(p, 90*time.Second) }()

	serverExitCh := make(chan error, 1)
	go func() { serverExitCh <- srv.Wait() }()

	select {
	case err := <-readyCh:
		if err != nil {
			cleanup()
			return err
		}
	case err := <-serverExitCh:
		cleanup()
		if err != nil {
			return fmt.Errorf("dev server exited unexpectedly: %w", err)
		}
		return fmt.Errorf("dev server exited unexpectedly")
	}

	fmt.Printf("✅  Server ready — opening Chrome...\n\n")

	for {
		browserCmd, err := browser.Launch(chromeBin, profileDir, url)
		if err != nil {
			cleanup()
			return err
		}
		entry.BrowserPID = browserCmd.Process.Pid
		_ = state.Add(worktreeName, entry)

		browserDone := make(chan struct{}, 1)
		go func() {
			browser.WaitForBrowserClose(browserCmd.Process.Pid, chromeBin, profileDir)
			browserDone <- struct{}{}
		}()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

		select {
		case <-sigCh:
			browser.KillBrowser(browserCmd, chromeBin, profileDir)
			action := promptAfterChromeClosed()
			switch action {
			case "r":
				clearScreen()
				printSessionInfo(info)
				fmt.Printf("🔄  Relaunching Chrome at %s...\n\n", url)
				continue
			case "k":
				fmt.Println("🔴  Stopping dev server...")
				cleanup()
				return nil
			default: // "q"
				fmt.Printf("💤  Dev server kept running on port %d\n", p)
				if len(args) == 1 {
					fmt.Printf("    Re-attach: devbrowser %s\n", args[0])
				}
				return nil
			}

		case <-serverExitCh:
			fmt.Println("\n🔴  Dev server exited — closing browser...")
			browser.KillBrowser(browserCmd, chromeBin, profileDir)
			cleanup()
			return nil

		case <-browserDone:
			browser.KillBrowser(browserCmd, chromeBin, profileDir)
			action := promptAfterChromeClosed()
			switch action {
			case "r":
				clearScreen()
				printSessionInfo(info)
				fmt.Printf("🔄  Relaunching Chrome at %s...\n\n", url)
				continue
			case "k":
				fmt.Println("🔴  Stopping dev server...")
				cleanup()
				return nil
			default: // "q" — keep server, keep state so reattach works
				fmt.Printf("💤  Dev server kept running on port %d\n", p)
				if len(args) == 1 {
					fmt.Printf("    Re-attach: devbrowser %s\n", args[0])
				}
				// Keep state entry so `devbrowser <worktree>` can find the session
				return nil
			}
		}
	}
}

func isServerAlive(pid int) bool { return isProcessAlive(pid) }

// attachToSession opens Chrome for an existing session using the correct profile.
// existing is passed in so [k]ill works even if state.json is cleared externally.
func attachToSession(cfg config.Config, worktreeName string, existing *state.Entry, profileDir, url string) error {
	p := existing.Port
	chromeBin, err := browser.Find(cfg.BrowserPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		return err
	}

	killServer := func() {
		fmt.Println("🔴  Stopping dev server...")
		killGroupByPGID(existing.ServerPGID)
		_ = state.Remove(worktreeName)
	}

	for {
		browserCmd, err := browser.Launch(chromeBin, profileDir, url)
		if err != nil {
			return err
		}

		browserDone := make(chan struct{}, 1)
		go func() {
			browser.WaitForBrowserClose(browserCmd.Process.Pid, chromeBin, profileDir)
			browserDone <- struct{}{}
		}()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

		handleAction := func(action string) (done bool) {
			switch action {
			case "k":
				killServer()
				return true
			case "q":
				fmt.Printf("💤  Detached. Server still on port %d\n", p)
				return true
			}
			return false // "r" — relaunch
		}

		select {
		case <-sigCh:
			browser.KillBrowser(browserCmd, chromeBin, profileDir)
			if handleAction(promptAttachedChromeClosed()) {
				return nil
			}
		case <-browserDone:
			browser.KillBrowser(browserCmd, chromeBin, profileDir)
			if handleAction(promptAttachedChromeClosed()) {
				return nil
			}
		}
	}
}

func promptAttachedChromeClosed() string {
	fmt.Println()
	fmt.Println("Chrome closed. What would you like to do?")
	fmt.Println("  [r] Relaunch Chrome  (keeps session, cookies, localStorage)")
	fmt.Println("  [k] Kill dev server and exit")
	fmt.Println("  [q] Quit devbrowser  (keep dev server running)")
	fmt.Print("> ")
	return readLine()
}

// attachOnly opens Chrome pointed at an already-running server.
// No server is started or stopped — Chrome lifecycle only.
func attachOnly(cfg config.Config, p int) error {
	chromeBin, err := browser.Find(cfg.BrowserPath)
	if err != nil {
		return err
	}

	profilesDir := cfg.ProfilesDir
	if profilesDir == "" {
		profilesDir, err = config.ProfilesDir()
		if err != nil {
			return err
		}
	}

	cwd, _ := os.Getwd()
	worktreeName := filepath.Base(filepath.Dir(cwd)) + "__" + filepath.Base(cwd)
	profileDir := filepath.Join(profilesDir, worktreeName)
	url := fmt.Sprintf("http://localhost:%d", p)

	info := sessionInfo{
		worktreeName: worktreeName,
		worktreeDir:  cwd,
		devCmd:       "(attached — server already running)",
		p:            p,
		profileDir:   profileDir,
		url:          url,
	}
	clearScreen()
	printSessionInfo(info)
	fmt.Printf("🔗  Attaching Chrome to existing server...\n\n")

	for {
		browserCmd, err := browser.Launch(chromeBin, profileDir, url)
		if err != nil {
			return err
		}

		browserDone := make(chan struct{}, 1)
		go func() {
			browser.WaitForBrowserClose(browserCmd.Process.Pid, chromeBin, profileDir)
			browserDone <- struct{}{}
		}()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

		select {
		case <-sigCh:
			browser.KillBrowser(browserCmd, chromeBin, profileDir)
			action := promptAfterChromeClosed()
			switch action {
			case "r":
				clearScreen()
				printSessionInfo(info)
				fmt.Printf("🔄  Relaunching Chrome at %s...\n\n", url)
				continue
			default:
				fmt.Printf("💤  Detached from port %d\n", p)
				return nil
			}
		case <-browserDone:
			browser.KillBrowser(browserCmd, chromeBin, profileDir)
			action := promptAfterChromeClosed()
			switch action {
			case "r":
				clearScreen()
				printSessionInfo(info)
				fmt.Printf("🔄  Relaunching Chrome at %s...\n\n", url)
				continue
			default: // "k" or "q" — we don't own the server either way
				fmt.Printf("💤  Detached from port %d\n", p)
				return nil
			}
		}
	}
}

func promptAfterChromeClosed() string {
	fmt.Println("Chrome closed. What would you like to do?")
	fmt.Println("  [r] Relaunch Chrome  (keeps session, cookies, localStorage)")
	fmt.Println("  [k] Kill dev server and exit")
	fmt.Println("  [q] Quit devbrowser  (keep dev server running in background)")
	fmt.Print("> ")
	return readLine()
}
