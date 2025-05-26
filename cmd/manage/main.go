/*
 * wpid - WordPress management tool
 * Copyright (C) 2025 Regi E
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <https://www.gnu.org/licenses/>.
 */

package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/joho/godotenv"
	"github.com/pkg/browser"
)

var (
	// Theme Colors
	colorPrimary      = lipgloss.Color("#7C3AED")
	colorSecondary    = lipgloss.Color("#38BDF8")
	colorSuccess      = lipgloss.Color("#22D3EE")
	colorError        = lipgloss.Color("#F43F5E")
	colorWarning      = lipgloss.Color("#FBBF24")
	colorInfo         = lipgloss.Color("245")
	colorMuted        = lipgloss.Color("241")
	colorTextInput    = lipgloss.Color("#F1F5F9")
	colorTextEmphasis = lipgloss.Color("#E0E7FF")

	// Base Styles
	baseTextStyle = lipgloss.NewStyle().Foreground(colorInfo)

	// Feedback Messages
	successMsgStyle = lipgloss.NewStyle().MarginBottom(1).Foreground(colorSuccess)
	errorMsgStyle   = lipgloss.NewStyle().MarginBottom(1).Foreground(colorError)
	warningMsgStyle = lipgloss.NewStyle().MarginBottom(1).Foreground(colorWarning)
	infoMsgStyle    = lipgloss.NewStyle().MarginBottom(1).Foreground(colorSecondary)

	inputPromptStyle = lipgloss.NewStyle().Foreground(colorSecondary).SetString("▸ ")
	inputTextStyle   = lipgloss.NewStyle().Foreground(colorTextInput)
	descTextStyle    = lipgloss.NewStyle().Foreground(colorMuted).Italic(true)

	// General
	subtleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("242"))
	boldStyle   = lipgloss.NewStyle().Bold(true)

	// Application & Section Headers
	appTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAA6FF")).
			Border(lipgloss.ThickBorder(), false, false, true, false).
			BorderForeground(lipgloss.Color("#AD58B4")).
			PaddingLeft(2).PaddingRight(2).
			MarginBottom(1)

	sectionHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("12")).
				Border(lipgloss.RoundedBorder(), false, false, false, true).
				BorderForeground(lipgloss.Color("12")).
				PaddingLeft(1).MarginBottom(1).MarginTop(1)

	// Feedback Messages
	msgBaseStyle = lipgloss.NewStyle().Padding(0, 1).MarginBottom(1).MaxWidth(80)

	successTitle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("78"))
	successBox   = msgBaseStyle.Border(lipgloss.NormalBorder(), true).
			BorderForeground(lipgloss.Color("78")).
			Foreground(lipgloss.Color("78"))

	errorTitle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("197"))
	errorBox   = msgBaseStyle.Border(lipgloss.NormalBorder(), true).
			BorderForeground(lipgloss.Color("197")).
			Foreground(lipgloss.Color("197"))

	warningTitle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	warningBox   = msgBaseStyle.Border(lipgloss.NormalBorder(), true).
			BorderForeground(lipgloss.Color("214")).
			Foreground(lipgloss.Color("214"))

	infoTitle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	infoBox   = msgBaseStyle.Border(lipgloss.NormalBorder(), true).
			BorderForeground(lipgloss.Color("39")).
			Foreground(lipgloss.Color("39"))

	// Command/Code style
	commandStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true)
	inputStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("202"))

	// WP-CLI output style
	wpcliOutputStyle = lipgloss.NewStyle().MarginLeft(2).Foreground(lipgloss.Color("248"))

	// List styles
	listItemStyle      = lipgloss.NewStyle().PaddingLeft(2)
	listHeaderStyle    = lipgloss.NewStyle().Bold(true).MarginBottom(1)
	highlightTextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("202")).Bold(true)

	huhTheme = huh.ThemeDracula()
	verbose  bool
)

const (
	metaFileName      = ".wordpress-meta.json"
	envFileName       = ".env"
	backupsDirDefault = "backups"
)

// --- Helper Functions ---

func printVerbose(title string, details ...string) {
	if verbose {
		// Use a slightly different style for verbose info? Maybe subtle.
		fmt.Println(subtleStyle.Render("VERBOSE: " + title))
		for _, detail := range details {
			fmt.Println(lipgloss.NewStyle().MarginLeft(2).Foreground(colorMuted).Render(detail))
		}
		fmt.Println()
	}
}

func printAppTitle(title string) {
	fmt.Println(appTitleStyle.Render("🛠️  " + title + "  🛠️"))
}

func printSectionHeader(msg string) { fmt.Println(sectionHeaderStyle.Render("╭─ " + msg)) }

func printSuccess(title string, details ...string) {
	fmt.Println(successMsgStyle.Render("✔ " + title))
	for _, detail := range details {
		fmt.Println(lipgloss.NewStyle().MarginLeft(2).Foreground(colorInfo).Render(detail))
	}
	fmt.Println()
}

func printError(title string, details ...string) {
	fmt.Println(errorMsgStyle.Render("✖ " + title))
	for _, detail := range details {
		fmt.Println(lipgloss.NewStyle().MarginLeft(2).Foreground(colorInfo).Render(detail))
	}
	fmt.Println()
}

func printWarning(title string, details ...string) {
	fmt.Println(warningMsgStyle.Render("⚠ " + title))
	for _, detail := range details {
		fmt.Println(lipgloss.NewStyle().MarginLeft(2).Foreground(colorInfo).Render(detail))
	}
	fmt.Println()
}

func printInfo(title string, details ...string) {
	fmt.Println(infoMsgStyle.Render("ℹ " + title))
	for _, detail := range details {
		fmt.Println(lipgloss.NewStyle().MarginLeft(2).Foreground(colorInfo).Render(detail))
	}
	fmt.Println()
}

// runCommand executes a command, printing info only if verbose. Streams output.
// Returns error if command fails.
func runCommand(ctx context.Context, name string, args ...string) error {
	printVerbose("Running Command:", fmt.Sprintf("%s %s", name, strings.Join(args, " ")))
	cmd := exec.CommandContext(ctx, name, args...) // Use context
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("command '%s %s' failed: %w", name, args[0], err)
	}
	return nil
}

// runCommandGetOutput executes command, captures stdout, prints stderr only on error or verbose.
// Returns output string and error.
func runCommandGetOutput(ctx context.Context, name string, args ...string) (string, error) {
	printVerbose("Running Command (for output):", fmt.Sprintf("%s %s", name, strings.Join(args, " ")))
	cmd := exec.CommandContext(ctx, name, args...) // Use context
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	err := cmd.Run()
	stderrStr := strings.TrimSpace(errb.String())
	stdoutStr := strings.TrimSpace(outb.String())

	if err != nil {
		// Print stderr only if there was an actual execution error
		if len(stderrStr) > 0 {
			printWarning("Command Stderr:", stderrStr)
		}
		return stdoutStr, fmt.Errorf("command '%s %s' failed: %w", name, args[0], err)
	} else if verbose && len(stderrStr) > 0 {
		// Print stderr even on success if verbose and stderr has content
		printVerbose("Command Stderr (Success):", stderrStr)
	}
	return stdoutStr, nil
}

// loadEnv loads .env file from the current directory. Exits on failure.
func loadEnvOrFail() {
	if _, err := os.Stat(envFileName); os.IsNotExist(err) {
		printError(fmt.Sprintf("Environment file '%s' not found.", envFileName), "This script must be run from an instance directory.")
		os.Exit(1)
	}
	// Use Overload to allow existing env vars to take precedence if needed,
	// or Load to always load from file. Load is usually fine for this context.
	err := godotenv.Load(envFileName)
	if err != nil {
		printError(fmt.Sprintf("Error loading '%s' file", envFileName), err.Error())
		os.Exit(1)
	}
	printVerbose(fmt.Sprintf("Loaded environment from %s", envFileName))
}

func runCommandWithDir(dir string, name string, args ...string) error {
	printInfo("Running Command", fmt.Sprintf("%s %s", name, strings.Join(args, " ")))
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		printError("Command Failed", fmt.Sprintf("Error executing: %s %s", name, strings.Join(args, " ")), err.Error())
	}
	return err
}

// loadEnv loads .env file from the current directory.
func loadEnv() error {
	if _, err := os.Stat(envFileName); os.IsNotExist(err) {
		printError(fmt.Sprintf("Environment file '%s' not found.", envFileName), "This script must be run from an instance directory.")
		return err
	}
	err := godotenv.Load(envFileName)
	if err != nil {
		printError(fmt.Sprintf("Error loading '%s' file", envFileName), err.Error())
		return err
	}
	printInfo(fmt.Sprintf("Loaded environment from %s", envFileName))
	return nil
}

// wpCLI runs a wp-cli command via docker compose exec AS THE www-data USER. Handles context.
func wpCLI(ctx context.Context, args ...string) error {
	wpArgs := append([]string{"compose", "exec", "-T", "--user", "www-data", "wordpress", "wp"}, args...)
	return runCommand(ctx, "docker", wpArgs...) // Use the basic runCommand now
}

// wpCLIGetOutput runs wp-cli AS THE www-data USER and captures stdout. Handles context.
func wpCLIGetOutput(ctx context.Context, args ...string) (string, error) {
	wpArgs := append([]string{"compose", "exec", "-T", "--user", "www-data", "wordpress", "wp"}, args...)
	return runCommandGetOutput(ctx, "docker", wpArgs...)
}

// --- Instance Metadata (Local) ---
type LocalInstanceMeta struct {
	WordPressVersion string `json:"wordpress_version,omitempty"`
	DBVersion        string `json:"db_version,omitempty"`
	Status           string `json:"status,omitempty"`
}

func runCommandWithOutputStreams(name string, args ...string) error {
	printInfo("Running Command:", fmt.Sprintf("%s %s", name, strings.Join(args, " ")))
	cmd := exec.Command(name, args...)
	// cmd.Dir can be set here if needed, but manage-go usually runs in instance dir

	// Pipe output directly to the Go program's output/error streams
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		// Don't double-print the error, as it should have streamed to stderr already.
		// Just return the error signal.
		// printError("Command Failed", fmt.Sprintf("Error executing: %s %s", name, strings.Join(args, " ")), err.Error())
		return fmt.Errorf("command failed: %w", err) // Wrap error for context
	}
	return nil // Success
}

func readLocalMeta() (*LocalInstanceMeta, error) {
	if _, err := os.Stat(metaFileName); os.IsNotExist(err) {
		// If meta file doesn't exist, return an empty struct, don't error out.
		return &LocalInstanceMeta{}, nil
	}
	data, err := os.ReadFile(metaFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to read local meta file %s: %w", metaFileName, err)
	}
	var meta LocalInstanceMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		// If unmarshal fails (e.g. empty file), return empty struct
		if len(data) == 0 || string(data) == "{}" {
			return &LocalInstanceMeta{}, nil
		}
		return nil, fmt.Errorf("failed to unmarshal local meta file %s: %w", metaFileName, err)
	}
	return &meta, nil
}

func writeLocalMeta(meta *LocalInstanceMeta) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal local meta data: %w", err)
	}
	return os.WriteFile(metaFileName, data, 0644)
}

func updateLocalStatus(status string) {
	meta, err := readLocalMeta()
	if err != nil {
		printWarning("Meta File Read Error", fmt.Sprintf("Could not read .wordpress-meta.json to update status: %v", err))
		return
	}
	meta.Status = status
	if err := writeLocalMeta(meta); err != nil {
		printWarning("Meta File Write Error", fmt.Sprintf("Could not write .wordpress-meta.json to update status: %v", err))
	} else {
		printInfo("Instance status updated in metadata.", fmt.Sprintf("New status: %s", status))
	}
}

func shellQuote(s string) string {
	// Replace every ' with '\'' (close quote, escaped single quote, reopen quote)
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// Helper to run a command with sudo, asking for confirmation.
// This will prompt for sudo password on the host.
func runCommandWithSudoHost(ctx context.Context, description string, command string, args ...string) error {
	printInfo("Action required:", description)
	printInfo("This will run the following command with sudo on your host machine:")
	fullCmdStr := fmt.Sprintf("sudo %s %s", command, strings.Join(args, " "))
	printInfo("  " + commandStyle.Render(fullCmdStr))

	var confirmSudo bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Confirm Sudo Execution").
				Description("Are you sure you want to run this command with sudo?").
				Affirmative("Yes, proceed").
				Negative("No, cancel").
				Value(&confirmSudo),
		),
	).WithTheme(huhTheme) // Use your manage tool's theme

	if err := form.Run(); err != nil || !confirmSudo {
		printInfo("Sudo command cancelled by user.")
		return errors.New("sudo command cancelled")
	}

	cmd := exec.CommandContext(ctx, "sudo", append([]string{command}, args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	printInfo("Executing with sudo...")
	err := cmd.Run()
	if err != nil {
		printError("Sudo command failed.", err.Error())
		return fmt.Errorf("sudo command '%s' failed: %w", command, err)
	}
	printSuccess(fmt.Sprintf("Successfully executed: %s", fullCmdStr))
	return nil
}

// File: cmd/manage/main.go

func cmdFixWPContentPermissions(ctx context.Context) error {
	printSectionHeader("Fix ./wp-content and ./wordpress Host Permissions")
	directories := []string{"./wp-content", "./wordpress"}

	for _, dir := range directories {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			printError(fmt.Sprintf("Directory '%s' not found.", dir), "Cannot fix permissions.")
			return err
		}
	}

	switch runtime.GOOS {
	case "linux", "darwin":
		currentHostUser, userErr := user.Current()
		if userErr != nil {
			printError("Could not determine current host user.", userErr.Error())
			return userErr
		}

		webGroup := "www-data"
		if runtime.GOOS == "darwin" {
			webGroup = "_www"
		}

		// Add the current user to the web server group
		addUserToGroupCmd := fmt.Sprintf("sudo usermod -aG %s %s", shellQuote(webGroup), shellQuote(currentHostUser.Username))
		printInfo(fmt.Sprintf("Adding current user '%s' to group '%s'...", currentHostUser.Username, webGroup))
		if err := runCommand(ctx, "sh", "-c", addUserToGroupCmd); err != nil {
			printError("Failed to add user to group.", err.Error())
			return err
		}
		printSuccess(fmt.Sprintf("User '%s' added to group '%s'.", currentHostUser.Username, webGroup))

		// Ensure group write access and set group ID inheritance
		for _, dir := range directories {
			safeDir := shellQuote(dir)
			cmd1 := fmt.Sprintf("sudo chmod -R g+w %s", safeDir)
			cmd2 := fmt.Sprintf("sudo find %s -type d -exec chmod g+s {} \\;", safeDir)

			printInfo(fmt.Sprintf("Ensuring group write access and inheritance for '%s'...", dir))
			if err := runCommand(ctx, "sh", "-c", fmt.Sprintf("%s && %s", cmd1, cmd2)); err != nil {
				printError(fmt.Sprintf("Failed to update permissions for '%s'.", dir), err.Error())
				return err
			}
		}

		printSuccess(fmt.Sprintf("Permissions updated for '%s' and '%s'.", directories[0], directories[1]))
		return nil

	case "windows":
		printWarning("Windows Host Detected for Permission Fix")
		printInfo("Automated Linux-style permission changes are not applicable.")
		printInfo("Ensure Docker image handles UID/GID mapping for 'www-data'.")
		return nil

	default:
		printWarning("Unsupported Host OS for Automated Permission Fix:", runtime.GOOS)
		printInfo("Please ensure './wp-content' and './wordpress' are writable by web server in container.")
		return nil
	}
}

// --- NEW: Production Readiness Commands ---

func cmdProdCheck(ctx context.Context) error {
	printSectionHeader("Production Readiness Check")
	// This command will run a series of checks and report issues.
	// It doesn't change anything, just advises.

	issuesFound := 0
	warningsFound := 0

	// Check 1: WP_DEBUG constants
	printInfo("Checking WP_DEBUG settings...")
	wpConfigContent, err := os.ReadFile(filepath.Join(".", "wordpress", "wp-config.php")) // Assuming run from instance root
	if err != nil {
		printError("Could not read wp-config.php", err.Error())
		return err
	}
	wpConfigStr := string(wpConfigContent)

	if strings.Contains(wpConfigStr, "define( 'WP_DEBUG', true )") {
		printError("WP_DEBUG is true:", "This should be false in production.")
		issuesFound++
	} else {
		printSuccess("WP_DEBUG is false or not explicitly true.")
	}
	if strings.Contains(wpConfigStr, "define( 'WP_DEBUG_DISPLAY', true )") {
		printError("WP_DEBUG_DISPLAY is true:", "This should be false or undefined in production. Errors should not be shown to site visitors.")
		issuesFound++
	} else {
		printSuccess("WP_DEBUG_DISPLAY is false or not explicitly true.")
	}
	if !strings.Contains(wpConfigStr, "define( 'WP_DEBUG_LOG', true )") {
		printWarning("WP_DEBUG_LOG is not true:", "Consider enabling WP_DEBUG_LOG in production (to a non-web-accessible file) for troubleshooting.")
		warningsFound++
	} else {
		printSuccess("WP_DEBUG_LOG is enabled.")
	}

	// Check 2: Default admin user
	printInfo("Checking for default 'admin' user...")
	adminUserExists, _ := wpCLIGetOutput(ctx, "user", "get", "admin", "--field=ID", "--format=count")
	if strings.TrimSpace(adminUserExists) == "1" {
		printError("Default 'admin' user exists:", "Rename or delete the default 'admin' user for security.")
		issuesFound++
	} else {
		printSuccess("Default 'admin' user not found.")
	}

	// Check 3: WordPress and Plugin/Theme Versions (basic check)
	printInfo("Checking for outdated software (basic)...")
	coreUpdates, _ := wpCLIGetOutput(ctx, "core", "check-update", "--format=count")
	if strings.TrimSpace(coreUpdates) != "0" && coreUpdates != "" {
		printWarning("WordPress core updates available:", strings.TrimSpace(coreUpdates)+" update(s) pending.")
		warningsFound++
	} else {
		printSuccess("WordPress core appears up to date (based on wp-cli check).")
	}
	pluginUpdates, _ := wpCLIGetOutput(ctx, "plugin", "status", "--update=available", "--format=count")
	if strings.TrimSpace(pluginUpdates) != "0" && pluginUpdates != "" {
		printWarning("Plugin updates available:", strings.TrimSpace(pluginUpdates)+" plugin(s) need updates.")
		warningsFound++
	} else {
		printSuccess("Plugins appear up to date.")
	}
	themeUpdates, _ := wpCLIGetOutput(ctx, "theme", "status", "--update=available", "--format=count")
	if strings.TrimSpace(themeUpdates) != "0" && themeUpdates != "" {
		printWarning("Theme updates available:", strings.TrimSpace(themeUpdates)+" theme(s) need updates.")
		warningsFound++
	} else {
		printSuccess("Themes appear up to date.")
	}

	// Check 4: Directory Listing (via .htaccess if Apache)
	// This is harder to check programmatically without knowing server type & config
	printInfo("Checking for directory listing prevention (manual check recommended)...")
	printWarning("Manual Check: Ensure your web server prevents directory listing.", "For Apache, add 'Options -Indexes' to .htaccess or server config.")
	warningsFound++

	// Check 5: Salts and Keys in wp-config.php
	printInfo("Checking for unique salts and keys in wp-config.php...")
	// This is tricky to definitively check without parsing PHP. A simple check for placeholders.
	if strings.Contains(wpConfigStr, "put your unique phrase here") {
		printError("Default salts/keys found in wp-config.php:", "Generate unique salts and keys for production.")
		issuesFound++
	} else {
		printSuccess("Salts and keys appear to be customized (basic check).")
	}

	// Check 6: File Permissions (Basic Idea - harder to check perfectly)
	printInfo("Checking common file/directory permissions (conceptual)...")
	printWarning("Manual Check: Ensure proper file permissions.",
		"wp-content should generally be writable by the web server for uploads/cache.",
		"Core files, wp-config.php should be less permissive.")
	warningsFound++

	// Check 7: HTTPS (informational, as this is local)
	siteURL, _ := wpCLIGetOutput(ctx, "option", "get", "siteurl")
	if !strings.HasPrefix(strings.TrimSpace(siteURL), "https://") {
		printWarning("Site URL is not HTTPS:", "Ensure production site uses HTTPS for security.")
		warningsFound++
	} else {
		printSuccess("Site URL is HTTPS (good for production).")
	}

	fmt.Println()
	printSectionHeader("Production Readiness Summary")
	if issuesFound > 0 {
		printError(fmt.Sprintf("%d CRITICAL issues found.", issuesFound), "Please address these before deploying.")
	} else {
		printSuccess("No critical issues found by automated checks!")
	}
	if warningsFound > 0 {
		printWarning(fmt.Sprintf("%d warnings/manual checks recommended.", warningsFound))
	}
	printInfo("This check is not exhaustive. Always perform thorough manual reviews and security audits before going live.")
	return nil
}

func cmdProdPrep(ctx context.Context) error {
	printSectionHeader("Prepare for Production (Guidance & Some Actions)")

	// This command can offer to make some changes or guide the user.
	var confirmChanges bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Proceed with Production Preparation?").
				Description("This will guide you through steps like setting WP_DEBUG to false, checking users, etc.\nSome changes might be applied directly to wp-config.php if confirmed.").
				Affirmative("Yes, let's prepare").
				Negative("No, cancel").
				Value(&confirmChanges),
		),
	).WithTheme(huhTheme)

	if err := form.Run(); err != nil || !confirmChanges {
		printInfo("Production preparation cancelled.")
		return nil
	}

	// Step 1: WP_DEBUG settings
	printInfo("Step 1: WP_DEBUG settings...")
	wpConfigPath := filepath.Join(".", "wordpress", "wp-config.php")
	wpConfigContent, errFile := os.ReadFile(wpConfigPath)
	if errFile != nil {
		printError("Could not read wp-config.php", errFile.Error())
		return errFile
	}
	wpConfigStr := string(wpConfigContent)

	madeChangesToConfig := false
	if strings.Contains(wpConfigStr, "define( 'WP_DEBUG', true )") {
		var confirmDebugFalse bool
		huh.NewConfirm().Title("Set WP_DEBUG to false?").Description("WP_DEBUG is currently true. For production, it should be false.").Affirmative("Yes, set to false").Negative("No, keep true").Value(&confirmDebugFalse).WithTheme(huhTheme).Run()
		if confirmDebugFalse {
			wpConfigStr = strings.Replace(wpConfigStr, "define( 'WP_DEBUG', true )", "define( 'WP_DEBUG', false )", 1)
			madeChangesToConfig = true
			printSuccess("WP_DEBUG set to false (in memory).")
		}
	}
	if strings.Contains(wpConfigStr, "define( 'WP_DEBUG_DISPLAY', true )") {
		var confirmDisplayFalse bool
		huh.NewConfirm().Title("Set WP_DEBUG_DISPLAY to false?").Description("WP_DEBUG_DISPLAY is true. For production, errors should not be shown to visitors.").Affirmative("Yes, set to false").Negative("No, keep true").Value(&confirmDisplayFalse).WithTheme(huhTheme).Run()
		if confirmDisplayFalse {
			wpConfigStr = strings.Replace(wpConfigStr, "define( 'WP_DEBUG_DISPLAY', true )", "define( 'WP_DEBUG_DISPLAY', false );\n@ini_set( 'display_errors', 0 );", 1)
			madeChangesToConfig = true
			printSuccess("WP_DEBUG_DISPLAY set to false and display_errors to 0 (in memory).")
		}
	}
	if !strings.Contains(wpConfigStr, "define( 'WP_DEBUG_LOG', true )") {
		var confirmLogTrue bool
		huh.NewConfirm().Title("Enable WP_DEBUG_LOG?").Description("WP_DEBUG_LOG is not enabled. It's good for logging errors to wp-content/debug.log in production.").Affirmative("Yes, enable WP_DEBUG_LOG").Negative("No, skip").Value(&confirmLogTrue).WithTheme(huhTheme).Run()
		if confirmLogTrue {
			// Try to add it after WP_DEBUG definition or near other debug constants
			debugLine := "define( 'WP_DEBUG', false );" // Assuming it was set or already false
			if strings.Contains(wpConfigStr, debugLine) {
				wpConfigStr = strings.Replace(wpConfigStr, debugLine, debugLine+"\n_error_log( 'WP_DEBUG_LOG enabled by wp-manager prod-prep' ); // Remove this line\ndefine( 'WP_DEBUG_LOG', true );", 1)
			} else { // Fallback: add near top (less ideal)
				wpConfigStr = "<?php\ndefine( 'WP_DEBUG_LOG', true );\n" + strings.TrimPrefix(wpConfigStr, "<?php")
				printWarning("Could not find WP_DEBUG define, added WP_DEBUG_LOG near top.")
			}
			madeChangesToConfig = true
			printSuccess("WP_DEBUG_LOG enabled (in memory).")
		}
	}

	if madeChangesToConfig {
		var confirmWriteConfig bool
		huh.NewConfirm().Title("Save wp-config.php changes?").Description("The above DEBUG changes have been prepared. Save them to wp-config.php?").Affirmative("Yes, save changes").Negative("No, discard").Value(&confirmWriteConfig).WithTheme(huhTheme).Run()
		if confirmWriteConfig {
			if err := os.WriteFile(wpConfigPath, []byte(wpConfigStr), 0644); err != nil {
				printError("Failed to write updated wp-config.php", err.Error())
			} else {
				printSuccess("wp-config.php updated with new DEBUG settings.")
			}
		} else {
			printInfo("wp-config.php DEBUG changes discarded.")
		}
	} else {
		printInfo("No DEBUG setting changes needed or confirmed for wp-config.php.")
	}
	fmt.Println()

	// Step 2: Salts and Keys (Guide only)
	printInfo("Step 2: Unique Salts and Keys...")
	if strings.Contains(wpConfigStr, "put your unique phrase here") {
		printWarning("Default salts/keys found in wp-config.php!")
		printInfo("It is CRITICAL to replace these with unique, randomly generated salts.")
		printInfo("You can generate new salts from: https://api.wordpress.org/secret-key/1.1/salt/")
		printInfo("Manually copy and paste the new salts into your wp-config.php file.")
	} else {
		printSuccess("Salts and keys in wp-config.php appear to be customized.")
	}
	fmt.Println()

	// Step 3: Default Admin User
	printInfo("Step 3: Default Admin User...")
	adminUserExists, _ := wpCLIGetOutput(ctx, "user", "get", "admin", "--field=ID", "--format=count")
	if strings.TrimSpace(adminUserExists) == "1" {
		printWarning("The default 'admin' user exists.")
		printInfo("For better security, you should rename this user or delete it after creating a new administrator with a strong password.")
		// Optionally offer to guide through creating a new admin and deleting old one via wp-cli
	} else {
		printSuccess("Default 'admin' user not found.")
	}
	fmt.Println()

	// Step 4: Update Software (Guide to use wp-cli)
	printInfo("Step 4: Update Core, Plugins, and Themes...")
	printInfo("Use WP-CLI to ensure everything is up to date before deployment:")
	printInfo("  " + commandStyle.Render(fmt.Sprintf("./%s wpcli core update", filepath.Base(os.Args[0]))))
	printInfo("  " + commandStyle.Render(fmt.Sprintf("./%s wpcli plugin update --all", filepath.Base(os.Args[0]))))
	printInfo("  " + commandStyle.Render(fmt.Sprintf("./%s wpcli theme update --all", filepath.Base(os.Args[0]))))
	fmt.Println()

	// Step 5: Permissions (Guide only)
	printInfo("Step 5: File & Directory Permissions...")
	printWarning("This step requires manual intervention on your server.")
	printInfo("Recommended permissions are typically:")
	printInfo("  - Directories: 755 or 750")
	printInfo("  - Files: 644 or 640")
	printInfo("  - wp-config.php: 600 or 440 or 400 (as restrictive as possible)")
	printInfo("  - The web server user (e.g., www-data) needs write access to `wp-content/uploads` and potentially `wp-content/cache` or other plugin-specific directories.")
	fmt.Println()

	// Step 6: .htaccess / Nginx config for security headers, permalinks (Guide)
	printInfo("Step 6: Web Server Configuration...")
	printInfo("Ensure your production web server (Apache .htaccess or Nginx config) is optimized for WordPress and includes security headers.")
	printInfo("Key items for Apache .htaccess (beyond WordPress rules):")
	printInfo("  " + commandStyle.Render("Options -Indexes") + " (to prevent directory listing)")
	printInfo("  Consider rules to protect wp-config.php, xmlrpc.php, and prevent script execution in uploads.")
	printWarning("Review security best practices for your specific web server.")
	fmt.Println()

	printSuccess("Production Preparation Guidance Complete.")
	printInfo("Remember to always back up your site before making significant changes or deploying.")
	return nil
}

// --- Service Management Commands ---

func cmdStart(ctx context.Context) error {
	printSectionHeader("Starting Services")
	err := runCommand(ctx, "docker", "compose", "up", "-d")
	if err == nil {
		updateLocalStatus("Running")
		printSuccess("Services Started", "WordPress instance is now running in the background.")
	} else {
		printError("Failed to Start Services", err.Error())
	}
	return err
}

func cmdStop(ctx context.Context) error {
	printSectionHeader("Stopping Services")
	printInfo("Stopping services and preserving data volumes (like the database)...")

	// Execute 'docker compose down' WITHOUT the -v flag to keep named volumes.
	err := runCommand(ctx, "docker", "compose", "down")

	if err == nil {
		updateLocalStatus("Stopped")
		printSuccess("Services Stopped", "WordPress instance has been shut down.")
		printInfo("Data volumes (e.g., database) have been preserved.")
		printInfo("To completely remove this instance and its data, use the main 'wp-manager delete' command.")
	} else {
		printError("Failed to Stop Services", err.Error())
	}
	return err
}

func cmdRestart(ctx context.Context) error {
	printSectionHeader("Restarting Services")
	printInfo("Stopping services...")
	if err := cmdStop(ctx); err != nil {
		// Stop failed, maybe don't try to start?
		printError("Restart failed because stop command encountered an error.")
		return err
	}
	fmt.Println() // Add space
	printInfo("Starting services...")
	if err := cmdStart(ctx); err != nil {
		printError("Restart failed because start command encountered an error.")
		return err
	}
	printSuccess("Services Restarted")
	return nil
}

func cmdUpdate(ctx context.Context) error {
	printSectionHeader("Updating Services (Docker Images)")
	printInfo("Pulling latest images...")
	if err := runCommand(ctx, "docker", "compose", "pull"); err != nil {
		return err
	}

	printInfo("Starting updated services...")
	err := runCommand(ctx, "docker", "compose", "up", "-d", "--force-recreate", "--remove-orphans")
	if err == nil {
		updateLocalStatus("Running")
		printSuccess("Services Updated and Restarted", "Docker images pulled and services recreated.")
	} else {
		printError("Failed to Start Updated Services", err.Error())
	}
	return err
}

func cmdConsole(ctx context.Context) error {
	printSectionHeader("WordPress Container Console")
	printInfo("Opening a bash shell inside the 'wordpress' container...", "Type 'exit' to return.")
	// Use docker compose exec directly for interactive session
	cmd := exec.CommandContext(ctx, "docker", "compose", "exec", "wordpress", "bash")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run() // Run directly, not via helper
	if err != nil {
		// Don't print error if it's just the user exiting normally
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 0 {
			// Normal exit
		} else {
			printError("Console Error", "Failed to open or run console session.", err.Error())
		}
	}
	return err
}

func cmdLogs(ctx context.Context) error {
	printSectionHeader("Service Logs")
	printInfo("Streaming logs from all services...", "Press Ctrl+C to stop.")
	cmd := exec.CommandContext(ctx, "docker", "compose", "logs", "-f", "--tail", "100")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	// Handle Ctrl+C (SIGINT) gracefully - it will cause an error here
	if err != nil && !errors.Is(err, context.Canceled) && !strings.Contains(err.Error(), "signal:") {
		printError("Logs Command Failed", err.Error())
		return err // Return actual error
	}
	fmt.Println("\n" + infoMsgStyle.Render("ℹ Log streaming stopped."))
	return nil // Return nil on graceful exit/Ctrl+C
}

// --- WP-CLI Commands ---

func cmdWPInstall(ctx context.Context) error {
	printSectionHeader("Finalizing WordPress Installation")

	// --- Check if services are running ---
	printInfo("Checking service status...")
	checkCmd := exec.CommandContext(ctx, "docker", "compose", "ps", "--services", "--filter", "status=running")
	var checkOut bytes.Buffer
	checkCmd.Stdout = &checkOut
	_ = checkCmd.Run() // Ignore error, just check output
	isRunning := strings.Contains(checkOut.String(), "wordpress")

	if !isRunning {
		printInfo("Services not running. Starting them now...")
		if err := runCommand(ctx, "docker", "compose", "up", "-d"); err != nil {
			printError("Failed to Start Services", "Cannot proceed with WP installation if services failed to start.", err.Error())
			return err // Return error
		}
		updateLocalStatus("Running")
		printInfo("Waiting for services to initialize after start (e.g., database)...")
		// Use context for cancellable sleep
		select {
		case <-time.After(15 * time.Second):
			// Continue
		case <-ctx.Done():
			printWarning("Wait cancelled.")
			return ctx.Err()
		}
	} else {
		printInfo("Services are already running.")
	}

	// --- Check if already installed ---
	_, errCheck := wpCLIGetOutput(ctx, "option", "get", "siteurl")
	if errCheck == nil {
		printWarning("Already Installed?", "WP-CLI reports 'siteurl' option exists. WordPress might already be installed.")
		var proceed bool
		confirm := huh.NewConfirm().
			Title("Re-run Installation?").
			Description("Are you sure you want to proceed? This might overwrite existing settings.").
			Affirmative("Yes, proceed").Negative("No, cancel").Value(&proceed)
		if errConfirm := confirm.WithTheme(huhTheme).Run(); errConfirm != nil || !proceed {
			printInfo("Installation Cancelled.")
			return nil // Not an error, user cancelled
		}
		printInfo("Proceeding with re-installation...")
	} else {
		printInfo("WordPress core options not found, proceeding with new installation.")
	}

	// --- Gather Installation Details ---
	var siteTitle, adminUser, adminEmail string
	adminPassword, _ := generateRandomString(8)

	currentDir, _ := os.Getwd()
	defaultSiteTitle := "WP Site"
	if currentDir != "" {
		defaultSiteTitle = "WP Site (" + filepath.Base(currentDir) + ")"
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Site Title").Value(&siteTitle).Placeholder(defaultSiteTitle),
			huh.NewInput().Title("Admin Username").Value(&adminUser).Placeholder("admin"),
			huh.NewInput().Title("Admin Email").Value(&adminEmail).Placeholder("admin@example.com"),
			huh.NewNote().Title("Generated Admin Password").Description(commandStyle.Render(adminPassword)),
		),
	).WithTheme(huhTheme)

	if err := form.Run(); err != nil {
		printError("Input Cancelled", "WordPress installation aborted.")
		return err
	}
	if siteTitle == "" {
		siteTitle = defaultSiteTitle
	}
	if adminUser == "" {
		adminUser = "admin"
	}
	if adminEmail == "" {
		adminEmail = "admin@example.com"
	}

	// --- Execute WP-CLI Core Install ---
	wordpressPort := os.Getenv("WORDPRESS_PORT")
	if wordpressPort == "" {
		wordpressPort = "80"
		printWarning("WORDPRESS_PORT not found in .env", "Defaulting site URL to use port 80.")
	}
	siteURL := fmt.Sprintf("http://localhost:%s", wordpressPort)

	printInfo("Installing WordPress core via WP-CLI...")
	errInstall := wpCLI(ctx, "core", "install", "--url="+siteURL, "--title="+siteTitle, "--admin_user="+adminUser, "--admin_password="+adminPassword, "--admin_email="+adminEmail)
	if errInstall != nil {
		printError("WordPress Core Install Failed", "Check WP-CLI output.")
		return errInstall
	} // Return error

	printInfo("Setting permalinks to /%postname%/...")
	_ = wpCLI(ctx, "option", "update", "permalink_structure", "/%postname%/", "--quiet")

	// --- Update Metadata ---
	printInfo("Updating metadata file...")
	wpVersion, _ := wpCLIGetOutput(ctx, "core", "version", "--quiet")
	// dbVersionRaw, _ := wpCLIGetOutput(ctx, "db", "version", "--quiet")
	// dbVersion := extractDBVersion(dbVersionRaw)

	meta, readErr := readLocalMeta()
	if readErr != nil {
		printWarning("Meta Read Failed", "Could not update metadata.", readErr.Error())
	} else {
		meta.WordPressVersion = wpVersion
		// TODO: Find a way to get the DB version from WP-CLI in the container, hard coded for now
		meta.DBVersion = "8.0" // dbVersion
		if writeErr := writeLocalMeta(meta); writeErr != nil {
			printWarning("Meta Update Failed", writeErr.Error())
		}
	}

	printSuccess("🎉 WordPress Installation Complete!",
		"Site URL: "+commandStyle.Render(siteURL),
		"Admin User: "+commandStyle.Render(adminUser),
		"Admin Password: "+commandStyle.Render(adminPassword)+" (Please save this securely!)",
		"Admin Email: "+commandStyle.Render(adminEmail))
	return nil
}

func getActivePluginsAsJSON(ctx context.Context) (string, error) {
	// Run WP-CLI to get the list of active plugins
	output, err := wpCLIGetOutput(ctx, "plugin", "list", "--status=active", "--format=json")
	if err != nil {
		printError("Failed to retrieve active plugins", err.Error())
		return "", err
	}

	// Validate JSON output
	var plugins []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &plugins); err != nil {
		printError("Failed to parse active plugins as JSON", err.Error())
		return "", err
	}

	// Return the JSON string
	return output, nil
}

// cmdManagePlugins modified to return error
func cmdManagePlugins(ctx context.Context) error {
	printSectionHeader("Plugin Management")
	// Keep looping until user selects 'back'
	for {
		var choice string
		prompt := huh.NewSelect[string]().
			Title("Plugin Actions").
			Options(
				huh.NewOption("Install New", "install"),
				huh.NewOption("Update All", "update"),
				huh.NewOption("Toggle Active/Inactive", "toggle"),
				huh.NewOption("List Installed", "list"),
				huh.NewOption("Delete", "delete"), // Added delete
				huh.NewOption("Go Back", "back"),
			).Value(&choice)

		if err := prompt.WithTheme(huhTheme).Run(); err != nil || choice == "back" {
			printInfo("Exiting plugin management.")
			return nil // Not an error to go back
		}

		var cmdErr error
		switch choice {
		case "install":
			cmdErr = pluginInstall(ctx)
		case "update":
			printInfo("Updating all plugins...")
			cmdErr = wpCLI(ctx, "plugin", "update", "--all")
		case "list":
			cmdErr = wpCLI(ctx, "plugin", "list")
		case "toggle":
			cmdErr = pluginToggle(ctx)
		case "delete":
			cmdErr = pluginDelete(ctx)
		}

		if cmdErr != nil {
			// Error already printed by wpCLI or helper, just note failure
			printError("Plugin command failed.")
			// Decide whether to loop again or return error
			// Maybe prompt user to continue? For now, just loop.
		}
		fmt.Println(subtleStyle.Render("\n--------------------\n")) // Separator
	}
}

// Helper for plugin install
func pluginInstall(ctx context.Context) error {
	var pluginSlug string
	input := huh.NewInput().Title("Enter plugin slug or ZIP URL").Value(&pluginSlug).Prompt("▸ ")
	if err := input.WithTheme(huhTheme).Run(); err != nil || pluginSlug == "" {
		printInfo("Cancelled.")
		return nil
	}
	var activate bool
	confirm := huh.NewConfirm().Title("Activate after install?").Value(&activate)
	_ = confirm.WithTheme(huhTheme).Run()

	args := []string{"plugin", "install", pluginSlug}
	if activate {
		args = append(args, "--activate")
	}
	printInfo(fmt.Sprintf("Installing plugin: %s (Activate: %v)", pluginSlug, activate))
	return wpCLI(ctx, args...)
}

// Helper for plugin toggle
func pluginToggle(ctx context.Context) error {
	pluginListOutput, err := wpCLIGetOutput(ctx, "plugin", "list", "--field=name", "--status=active,inactive")
	if err != nil {
		printError("Could not list plugins.")
		return err
	}
	plugins := strings.Split(strings.TrimSpace(pluginListOutput), "\n")
	if len(plugins) == 0 || (len(plugins) == 1 && plugins[0] == "") {
		printWarning("No Plugins Found", "No active or inactive plugins to toggle.")
		return nil
	}

	var pluginsToToggle []string
	var pluginOptions []huh.Option[string]
	for _, p := range plugins {
		if p != "" {
			pluginOptions = append(pluginOptions, huh.NewOption(p, p))
		}
	}

	multiSelect := huh.NewMultiSelect[string]().
		Title("Select plugin(s) to activate/deactivate").
		Description("Toggle the selected plugins' active/inactive state in order.").
		Options(pluginOptions...).
		Value(&pluginsToToggle)

	if err := multiSelect.WithTheme(huhTheme).Run(); err != nil || len(pluginsToToggle) == 0 {
		printInfo("Cancelled or no plugins selected for toggling.")
		return nil
	}

	printInfo("Toggling selected plugins in order:", strings.Join(pluginsToToggle, ", "))
	var lastErr error
	for _, plugin := range pluginsToToggle {
		err := wpCLI(ctx, "plugin", "toggle", plugin)
		if err != nil {
			lastErr = err
			printError(fmt.Sprintf("Failed to toggle plugin '%s'", plugin))
		}
	}
	return lastErr
}

// Helper for plugin delete
func pluginDelete(ctx context.Context) error {
	// List only inactive plugins for deletion safety? Or all? Let's list all deletable.
	pluginListOutput, err := wpCLIGetOutput(ctx, "plugin", "list", "--field=name", "--status=inactive,active") // Maybe filter more?
	if err != nil {
		printError("Could not list plugins.")
		return err
	}
	plugins := strings.Fields(pluginListOutput) // Use Fields to handle potential whitespace issues
	if len(plugins) == 0 {
		printWarning("No Plugins Found", "No plugins available to delete.")
		return nil
	}

	var pluginsToDelete []string
	multiSelect := huh.NewMultiSelect[string]().
		Title("Select plugin(s) to DELETE").
		Description("WARNING: This will delete plugin files!").
		Options(huh.NewOptions(plugins...)...).
		Value(&pluginsToDelete)

	if err := multiSelect.WithTheme(huhTheme).Run(); err != nil || len(pluginsToDelete) == 0 {
		printInfo("Cancelled or no plugins selected for deletion.")
		return nil
	}

	var confirm bool
	confirmPrompt := huh.NewConfirm().
		Title("Confirm Deletion").
		Description(fmt.Sprintf("Really delete the following plugin(s)?\n- %s\n\nTHIS IS IRREVERSIBLE!", strings.Join(pluginsToDelete, "\n- "))).
		Affirmative("Yes, DELETE").Negative("No, cancel")
	_ = confirmPrompt.Value(&confirm).WithTheme(huhTheme).Run()

	if !confirm {
		printInfo("Deletion Cancelled.")
		return nil
	}

	printInfo("Deleting selected plugins:", strings.Join(pluginsToDelete, ", "))
	// Run delete for each selected plugin
	var lastErr error
	for _, plugin := range pluginsToDelete {
		err := wpCLI(ctx, "plugin", "delete", plugin)
		if err != nil {
			lastErr = err // Keep track of the last error
			printError(fmt.Sprintf("Failed to delete plugin '%s'", plugin))
		}
	}
	return lastErr
}

// --- NEW Theme Management ---
func cmdManageThemes(ctx context.Context) error {
	printSectionHeader("Theme Management")
	for {
		var choice string
		prompt := huh.NewSelect[string]().
			Title("Theme Actions").
			Options(
				huh.NewOption("Install New", "install"),
				huh.NewOption("Update All", "update"),
				huh.NewOption("Activate Theme", "activate"),
				huh.NewOption("List Installed", "list"),
				huh.NewOption("Delete Theme", "delete"),
				huh.NewOption("Go Back", "back"),
			).Value(&choice)

		if err := prompt.WithTheme(huhTheme).Run(); err != nil || choice == "back" {
			printInfo("Exiting theme management.")
			return nil // Not an error
		}

		var cmdErr error
		switch choice {
		case "install":
			cmdErr = themeInstall(ctx)
		case "update":
			printInfo("Updating all themes...")
			cmdErr = wpCLI(ctx, "theme", "update", "--all")
		case "list":
			cmdErr = wpCLI(ctx, "theme", "list")
		case "activate":
			cmdErr = themeActivate(ctx)
		case "delete":
			cmdErr = themeDelete(ctx)
		}
		if cmdErr != nil {
			printError("Theme command failed.")
		}
		fmt.Println(subtleStyle.Render("\n--------------------\n"))
	}
}

func themeInstall(ctx context.Context) error {
	var themeSlug string
	input := huh.NewInput().Title("Enter theme slug or ZIP URL").Value(&themeSlug).Prompt("▸ ")
	if err := input.WithTheme(huhTheme).Run(); err != nil || themeSlug == "" {
		printInfo("Cancelled.")
		return nil
	}

	var activate bool
	confirm := huh.NewConfirm().Title("Activate after install?").Value(&activate)
	_ = confirm.WithTheme(huhTheme).Run()

	args := []string{"theme", "install", themeSlug}
	if activate {
		args = append(args, "--activate")
	}
	printInfo(fmt.Sprintf("Installing theme: %s (Activate: %v)", themeSlug, activate))
	return wpCLI(ctx, args...)
}

func themeActivate(ctx context.Context) error {
	themeListOutput, err := wpCLIGetOutput(ctx, "theme", "list", "--field=name", "--status=inactive")
	if err != nil {
		printError("Could not list inactive themes.")
		return err
	}
	themes := strings.Fields(themeListOutput)
	if len(themes) == 0 {
		printWarning("No Inactive Themes", "No inactive themes available to activate.")
		return nil
	}

	var themeToActivate string
	selectTheme := huh.NewSelect[string]().Title("Select theme to activate").Options(huh.NewOptions(themes...)...).Value(&themeToActivate)
	if err := selectTheme.WithTheme(huhTheme).Run(); err != nil || themeToActivate == "" {
		printInfo("Cancelled.")
		return nil
	}

	printInfo(fmt.Sprintf("Activating theme: %s", themeToActivate))
	return wpCLI(ctx, "theme", "activate", themeToActivate)
}

func themeDelete(ctx context.Context) error {
	// List only inactive themes for safety
	themeListOutput, err := wpCLIGetOutput(ctx, "theme", "list", "--field=name", "--status=inactive")
	if err != nil {
		printError("Could not list inactive themes.")
		return err
	}
	themes := strings.Fields(themeListOutput)
	if len(themes) == 0 {
		printWarning("No Inactive Themes", "No inactive themes available to delete.")
		return nil
	}

	var themesToDelete []string
	multiSelect := huh.NewMultiSelect[string]().
		Title("Select inactive theme(s) to DELETE").
		Description("WARNING: This will delete theme files!").
		Options(huh.NewOptions(themes...)...).
		Value(&themesToDelete)

	if err := multiSelect.WithTheme(huhTheme).Run(); err != nil || len(themesToDelete) == 0 {
		printInfo("Cancelled or no themes selected for deletion.")
		return nil
	}

	var confirm bool
	confirmPrompt := huh.NewConfirm().
		Title("Confirm Deletion").
		Description(fmt.Sprintf("Really delete the following theme(s)?\n- %s\n\nTHIS IS IRREVERSIBLE!", strings.Join(themesToDelete, "\n- "))).
		Affirmative("Yes, DELETE").Negative("No, cancel")
	_ = confirmPrompt.Value(&confirm).WithTheme(huhTheme).Run()
	if !confirm {
		printInfo("Deletion Cancelled.")
		return nil
	}

	printInfo("Deleting selected themes:", strings.Join(themesToDelete, ", "))
	var lastErr error
	for _, theme := range themesToDelete {
		err := wpCLI(ctx, "theme", "delete", theme)
		if err != nil {
			lastErr = err
			printError(fmt.Sprintf("Failed to delete theme '%s'", theme))
		}
	}
	return lastErr
}

// --- NEW User Management ---
func cmdManageUsers(ctx context.Context) error {
	printSectionHeader("User Management")
	for {
		var choice string
		prompt := huh.NewSelect[string]().
			Title("User Actions").
			Options(
				huh.NewOption("List Users", "list"),
				huh.NewOption("Create New User", "create"),
				huh.NewOption("Update User", "update"),
				huh.NewOption("Delete User", "delete"),
				huh.NewOption("Go Back", "back"),
			).Value(&choice)

		if err := prompt.WithTheme(huhTheme).Run(); err != nil || choice == "back" {
			printInfo("Exiting user management.")
			return nil // Not an error
		}

		var cmdErr error
		switch choice {
		case "list":
			cmdErr = wpCLI(ctx, "user", "list")
		case "create":
			cmdErr = userCreate(ctx)
		case "update":
			cmdErr = userUpdate(ctx)
		case "delete":
			cmdErr = userDelete(ctx)
		}
		if cmdErr != nil {
			printError("User command failed.")
		}
		fmt.Println(subtleStyle.Render("\n--------------------\n"))
	}
}

func userCreate(ctx context.Context) error {
	var username, email, role string
	var sendEmail bool
	password, _ := generateRandomString(14) // Suggest a password

	// Get available roles from WP-CLI
	rolesOutput, err := wpCLIGetOutput(ctx, "role", "list", "--field=name")
	var roleOptions []huh.Option[string]
	if err == nil {
		roles := strings.Fields(rolesOutput)
		for _, r := range roles {
			roleOptions = append(roleOptions, huh.NewOption(r, r))
		}
	} else {
		// Fallback if WP-CLI fails
		roleOptions = []huh.Option[string]{
			huh.NewOption("subscriber", "subscriber"),
			huh.NewOption("contributor", "contributor"),
			huh.NewOption("author", "author"),
			huh.NewOption("editor", "editor"),
			huh.NewOption("administrator", "administrator"),
		}
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Username").Value(&username).Validate(huh.ValidateNotEmpty()),
			huh.NewInput().
				Title("Email").
				Value(&email).
				Validate(func(s string) error {
					if s == "" {
						return errors.New("Email cannot be empty")
					}
					re := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
					if !re.MatchString(s) {
						return errors.New("Invalid email format")
					}
					return nil
				}),
			huh.NewSelect[string]().
				Title("Role").
				Options(roleOptions...).
				Value(&role).
				Description("Default: subscriber"),
			huh.NewNote().Title("Generated Password").Description(commandStyle.Render(password)),
			huh.NewConfirm().Title("Send user notification email?").Value(&sendEmail),
		),
	).WithTheme(huhTheme)

	if err := form.Run(); err != nil {
		printInfo("Cancelled.")
		return nil
	}
	if role == "" {
		role = "subscriber"
	} // Apply default if left blank

	args := []string{"user", "create", username, email, "--role=" + role, "--user_pass=" + password}
	if sendEmail {
		args = append(args, "--send-email")
	}

	printInfo(fmt.Sprintf("Creating user: %s (%s) role: %s", username, email, role))
	err = wpCLI(ctx, args...)
	if err == nil {
		printSuccess("User Created", fmt.Sprintf("Username: %s", username), fmt.Sprintf("Password: %s", password))
	}
	return err
}

func userUpdate(ctx context.Context) error {
	usersOutput, err := wpCLIGetOutput(ctx, "user", "list", "--field=user_login")
	if err != nil {
		printError("Could not list users.")
		return err
	}
	users := strings.Fields(usersOutput)
	if len(users) == 0 {
		printWarning("No Users Found.")
		return nil
	}

	var userToUpdate string
	selectUser := huh.NewSelect[string]().Title("Select user to update").Options(huh.NewOptions(users...)...).Value(&userToUpdate)
	if err := selectUser.WithTheme(huhTheme).Run(); err != nil || userToUpdate == "" {
		printInfo("Cancelled.")
		return nil
	}

	// Ask what to update
	var actions []string
	multiSelect := huh.NewMultiSelect[string]().
		Title("What to update?").
		Options(
			huh.NewOption("Password", "password"),
			huh.NewOption("Role", "role"),
			huh.NewOption("Email", "email"),
			// Add more fields like display_name, user_url etc. if needed
		).Value(&actions)
	if err := multiSelect.WithTheme(huhTheme).Run(); err != nil || len(actions) == 0 {
		printInfo("No update actions selected.")
		return nil
	}

	var newPassword, newRole, newEmail string
	var updateArgs []string = []string{"user", "update", userToUpdate}

	groupFields := []huh.Field{}
	for _, action := range actions {
		switch action {
		case "password":
			genPassword, _ := generateRandomString(14)
			groupFields = append(groupFields, huh.NewInput().Title("New Password").Value(&newPassword).Placeholder(genPassword).EchoMode(huh.EchoModePassword))
		case "role":
			groupFields = append(groupFields, huh.NewInput().Title("New Role").Value(&newRole).Placeholder("e.g., editor, author"))
		case "email":
			groupFields = append(groupFields, huh.NewInput().Title("New Email").Value(&newEmail))
		}
	}

	if len(groupFields) > 0 {
		updateForm := huh.NewForm(huh.NewGroup(groupFields...)).WithTheme(huhTheme)
		if err := updateForm.Run(); err != nil {
			printInfo("Cancelled.")
			return nil
		}
	} else {
		printInfo("No updatable fields selected.")
		return nil // Should not happen based on multiselect check
	}

	// Construct wp-cli args
	if newPassword != "" {
		updateArgs = append(updateArgs, "--user_pass="+newPassword)
	}
	if newRole != "" {
		updateArgs = append(updateArgs, "--role="+newRole)
	}
	if newEmail != "" {
		updateArgs = append(updateArgs, "--user_email="+newEmail)
	}

	if len(updateArgs) <= 3 { // Only contains "user update <username>"
		printInfo("No new values provided for update.")
		return nil
	}

	printInfo(fmt.Sprintf("Updating user: %s", userToUpdate))
	errUpdate := wpCLI(ctx, updateArgs...)
	if errUpdate == nil && newPassword != "" {
		printSuccess("User Updated", fmt.Sprintf("Password for %s updated to: %s", userToUpdate, newPassword))
	} else if errUpdate == nil {
		printSuccess("User Updated")
	}
	return errUpdate
}

func userDelete(ctx context.Context) error {
	usersOutput, err := wpCLIGetOutput(ctx, "user", "list", "--field=user_login")
	if err != nil {
		printError("Could not list users.")
		return err
	}
	users := strings.Fields(usersOutput)
	if len(users) == 0 {
		printWarning("No Users Found.")
		return nil
	}

	var usersToDelete []string
	multiSelect := huh.NewMultiSelect[string]().
		Title("Select user(s) to DELETE").
		Description("WARNING: This permanently deletes users!").
		Options(huh.NewOptions(users...)...).
		Value(&usersToDelete)

	if err := multiSelect.WithTheme(huhTheme).Run(); err != nil || len(usersToDelete) == 0 {
		printInfo("Cancelled or no users selected.")
		return nil
	}

	// Reassign posts?
	var reassignUserID string
	var reassign bool
	confirmReassign := huh.NewConfirm().Title("Reassign posts?").Description("Reassign posts belonging to deleted user(s) to another user?").Value(&reassign) // Fixed to use a bool
	_ = confirmReassign.Value(&reassign).WithTheme(huhTheme).Run()

	if reassign {
		// Get list of users again to select reassignment target
		usersOutputReassign, _ := wpCLIGetOutput(ctx, "user", "list", "--field=ID", "--orderby=ID") // Get IDs
		userIDs := strings.Fields(usersOutputReassign)
		if len(userIDs) > 0 {
			// Filter out the users being deleted? Ideally yes.
			inputReassign := huh.NewInput().Title("Reassign Posts To User ID").Description("Enter the User ID to reassign posts to.").Value(&reassignUserID).Validate(huh.ValidateNotEmpty())
			if err := inputReassign.WithTheme(huhTheme).Run(); err != nil {
				printInfo("Cancelled.")
				return nil
			}
		} else {
			printWarning("Cannot Reassign", "No other users found to reassign posts to.")
			reassignUserID = "" // Clear it
		}
	}

	var confirm bool
	confirmPrompt := huh.NewConfirm().
		Title("Confirm User Deletion").
		Description(fmt.Sprintf("Really delete user(s): %s?\nReassign posts to user ID: %s\n\nTHIS IS IRREVERSIBLE!", strings.Join(usersToDelete, ", "), reassignUserID)).
		Affirmative("Yes, DELETE").Negative("No, cancel")
	_ = confirmPrompt.Value(&confirm).WithTheme(huhTheme).Run()
	if !confirm {
		printInfo("Deletion Cancelled.")
		return nil
	}

	args := []string{"user", "delete"}
	args = append(args, usersToDelete...) // Add users to delete
	if reassignUserID != "" {
		args = append(args, "--reassign="+reassignUserID)
	} else {
		args = append(args, "--network") // Need network flag if deleting without reassignment on multisite? Check wp-cli docs. Assume simple delete for now.
		// Or maybe no extra flag needed if not reassigning? Let's assume simple delete for now.
	}

	printInfo("Deleting user(s):", strings.Join(usersToDelete, ", "))
	return wpCLI(ctx, args...)
}

// --- NEW Backup & Restore ---
func getDBCredentials() (user, password, database, host string, err error) {
	// Assumes loadEnvOrFail() has been called
	user = os.Getenv("WORDPRESS_DB_USER")
	password = os.Getenv("WORDPRESS_DB_PASSWORD")
	database = os.Getenv("WORDPRESS_DB_NAME")
	host = os.Getenv("WORDPRESS_DB_HOST") // Should be 'db' or your db service name

	if user == "" || database == "" { // Password can sometimes be empty for local dev
		return "", "", "", "", errors.New("database user, or database name not found in .env file")
	}
	// Host is also critical, but wp-cli needs it more than direct mysqldump inside db container
	if host == "" {
		printWarning("WORDPRESS_DB_HOST not found in .env, assuming 'db'. This is needed for wp-cli search-replace.")
		host = "db"
	}
	return user, password, database, host, nil
}

// urlReplace handles search and replace of URLs in the database.
func urlReplace(ctx context.Context) error {
	printSectionHeader("Search and Replace URLs in Database")

	printInfo("Serialization-Safe Operation", "All search and replace operations use WP-CLI and are safe for PHP serialized data.")
	var oldURL, newURL string
	var dryRun bool

	// Prompt user for old URL, new URL, and dry run option
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Old URL").
				Description("Enter the URL to search for (e.g., https://oldsite.com).").
				Value(&oldURL).
				Validate(func(s string) error {
					if s == "" {
						return errors.New("Old URL cannot be empty")
					}
					if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
						return errors.New("Old URL must start with http:// or https://")
					}
					return nil
				}),
			huh.NewInput().
				Title("New URL").
				Description("Enter the URL to replace with (e.g., https://newsite.com).").
				Value(&newURL).
				Validate(func(s string) error {
					if s == "" {
						return errors.New("New URL cannot be empty")
					}
					if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
						return errors.New("New URL must start with http:// or https://")
					}
					return nil
				}),
			huh.NewConfirm().
				Title("Dry Run").
				Description("Perform a dry run to preview changes without applying them?").
				Affirmative("Yes, dry run").
				Negative("No, apply changes").
				Value(&dryRun),
		),
	).WithTheme(huhTheme)

	if err := form.Run(); err != nil {
		printInfo("URL replace operation cancelled.")
		return nil
	}

	// Construct WP-CLI arguments
	args := []string{"search-replace", oldURL, newURL, "--all-tables"}
	if dryRun {
		args = append(args, "--dry-run")
	}

	printInfo("Executing URL replace in database...",
		fmt.Sprintf("Old URL: %s", oldURL),
		fmt.Sprintf("New URL: %s", newURL),
		fmt.Sprintf("Dry Run: %v", dryRun))

	// Run WP-CLI command
	if err := wpCLI(ctx, args...); err != nil {
		printError("URL Replace Failed", err.Error())
		return err
	}

	if dryRun {
		printSuccess("Dry Run Completed", "No changes were made to the database.")
	} else {
		printSuccess("URL Replace Completed", "The database has been updated with the new URL.")
	}

	return nil
}

// --- Unified Database Operations (`cmdDB`) ---
func cmdDB(ctx context.Context) error {
	printSectionHeader("Database Operations")
	var operation string
	dbForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select Database Operation").
				Options(
					huh.NewOption("Import Database (from SQL file, updates URLs)", "import"),
					huh.NewOption("Export Database (for live site, updates URLs)", "export"),
					huh.NewOption("Go Back", "back"),
				).
				Value(&operation),
		),
	).WithTheme(huhTheme)

	if err := dbForm.Run(); err != nil {
		printInfo("Database operation prompt cancelled.")
		return nil
	}
	if operation == "back" || operation == "" {
		printInfo("Exiting database operations.")
		return nil
	}

	switch operation {
	case "import":
		return dbImport(ctx)
	case "export":
		return dbExport(ctx)
	case "url-replace":
		return urlReplace(ctx)
	}
	return nil
}

// getProductionURLFromEnvOrPrompt Helper
func getProductionURLFromEnvOrPrompt(ctx context.Context) (string, error) {
	prodURL := os.Getenv("PRODUCTION_URL")
	if prodURL != "" {
		printInfo("Using PRODUCTION_URL from .env:", commandStyle.Render(prodURL))
		return prodURL, nil
	}

	printWarning("PRODUCTION_URL not found in .env file.")
	var userInputURL string
	inputField := huh.NewInput().
		Title("Live Site URL (PRODUCTION_URL)").
		Description("Enter the full URL of your live production site (e.g., https://myawesomesite.com).\nThis will be saved to your .env file.").
		Value(&userInputURL).
		Prompt("▸ ").
		Validate(func(s string) error {
			if s == "" {
				return errors.New("URL cannot be empty")
			}
			if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
				return errors.New("URL must start with http:// or https://")
			}
			// Potentially add more robust URL validation here if needed
			return nil
		})

	form := huh.NewForm(huh.NewGroup(inputField)).WithTheme(huhTheme)
	if err := form.Run(); err != nil {
		return "", fmt.Errorf("input cancelled: %w", err)
	}

	if userInputURL == "" {
		return "", errors.New("production URL not provided")
	}

	envMap, _ := godotenv.Read(envFileName) // Read existing, ignore error if not found
	if envMap == nil {
		envMap = make(map[string]string)
	}
	envMap["PRODUCTION_URL"] = userInputURL
	if err := godotenv.Write(envMap, envFileName); err != nil {
		printWarning("Failed to Update .env", fmt.Sprintf("Could not save PRODUCTION_URL: %v", err))
	} else {
		printSuccess("PRODUCTION_URL saved to .env file.")
		_ = godotenv.Overload(envFileName) // Reload env for current session
	}
	return userInputURL, nil
}

// dbImport handles importing a database.
func dbImport(ctx context.Context) error {
	printInfo("Starting database import process...")

	var dbFileHostPath string
	var selectedFile string

	// Look for .sql files in db/ directory
	sqlFiles, err := filepath.Glob("db/*.sql")
	if err != nil {
		printWarning("Error scanning db/ directory for SQL files: " + err.Error())
	}

	// Helper: validation function for custom path input
	validateFilePath := func(s string) error {
		if s == "" {
			return errors.New("file path cannot be empty")
		}
		expandedPath := s
		if strings.HasPrefix(s, "~"+string(os.PathSeparator)) || s == "~" {
			home, errHome := os.UserHomeDir()
			if errHome != nil {
				return errors.New("could not resolve home directory")
			}
			if s == "~" {
				expandedPath = home
			} else {
				expandedPath = filepath.Join(home, s[2:])
			}
		}
		if _, err := os.Stat(expandedPath); os.IsNotExist(err) {
			return fmt.Errorf("file not found: %s", expandedPath)
		} else if err != nil {
			return fmt.Errorf("error checking file %s: %v", expandedPath, err)
		}
		return nil
	}

	// Form logic
	if len(sqlFiles) > 0 {
		// User can pick a file or enter a custom path
		sqlFiles = append(sqlFiles, "Enter custom file path...")
		fileSelectForm := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Select a SQL file to import").
					Options(huh.NewOptions(sqlFiles...)...).
					Value(&selectedFile),
			),
		).WithTheme(huhTheme)

		if err := fileSelectForm.Run(); err != nil {
			printInfo("Database import prompt cancelled.")
			return nil
		}

		if selectedFile == "Enter custom file path..." {
			// Prompt for custom path
			fileInputForm := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Database Dump File Path").
						Description("Enter the local path to the SQL file to import.").
						Value(&dbFileHostPath).
						Validate(validateFilePath),
				),
			).WithTheme(huhTheme)

			if err := fileInputForm.Run(); err != nil {
				printInfo("Database import prompt cancelled.")
				return nil
			}
		} else {
			dbFileHostPath = selectedFile
		}
	} else {
		// No files found, prompt for path
		fileInputForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Database Dump File Path").
					Description("Enter the local path to the SQL file to import.").
					Value(&dbFileHostPath).
					Validate(validateFilePath),
			),
		).WithTheme(huhTheme)

		if err := fileInputForm.Run(); err != nil {
			printInfo("Database import prompt cancelled.")
			return nil
		}
	}

	if dbFileHostPath == "" {
		printInfo("No file selected. Import cancelled.")
		return nil
	}

	// Expand ~ if present
	if strings.HasPrefix(dbFileHostPath, "~"+string(os.PathSeparator)) || dbFileHostPath == "~" {
		home, errHome := os.UserHomeDir()
		if errHome != nil {
			printError("Error resolving home directory for SQL file path.", errHome.Error())
			return errHome
		}
		if dbFileHostPath == "~" {
			dbFileHostPath = home
		} else {
			dbFileHostPath = filepath.Join(home, dbFileHostPath[2:])
		}
	}

	absDbFileHostPath, err := filepath.Abs(dbFileHostPath)
	if err != nil {
		printError("Invalid File Path", fmt.Sprintf("Could not determine absolute path for %s: %v", dbFileHostPath, err))
		return err
	}

	dbUser, dbPassword, dbName, _, errCred := getDBCredentials()
	if errCred != nil {
		printError("DB Credentials Error", errCred.Error())
		return errCred
	}

	sqlFile, err := os.Open(absDbFileHostPath)
	if err != nil {
		printError("Failed to open SQL file", err.Error())
		return err
	}
	defer sqlFile.Close()

	printInfo(fmt.Sprintf("Importing database '%s' from '%s' into 'db' container...", dbName, filepath.Base(absDbFileHostPath)))

	mysqlCmdArgs := []string{"compose", "exec", "-T", "db", "mysql", "-u" + dbUser}
	if dbPassword != "" {
		mysqlCmdArgs = append(mysqlCmdArgs, "-p"+dbPassword)
	}
	mysqlCmdArgs = append(mysqlCmdArgs, dbName)

	cmd := exec.CommandContext(ctx, "docker", mysqlCmdArgs...)
	cmd.Stdin = sqlFile
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		printError("Database Import Failed.", fmt.Sprintf("Docker command failed: %v", err), "Stderr: "+stderr.String())
		return err
	}
	printSuccess("Database Imported Successfully into 'db' container.")

	var confirmUpdateURLs bool
	urlConfirmForm := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Update Site URLs?").
				Description("Update URLs from production to local development values in the database?").
				Affirmative("Yes, update URLs").
				Negative("No, skip URL update").
				Value(&confirmUpdateURLs),
		),
	).WithTheme(huhTheme)

	if err := urlConfirmForm.Run(); err != nil {
		printInfo("URL update prompt cancelled.")
		return nil // User cancelled
	}

	if confirmUpdateURLs {
		productionURL, errProdURL := getProductionURLFromEnvOrPrompt(ctx)
		if errProdURL != nil {
			printError("Failed to Get Production URL for replacement.", errProdURL.Error())
			return errProdURL
		}
		localWPPort := os.Getenv("WORDPRESS_PORT")
		if localWPPort == "" {
			printWarning("WORDPRESS_PORT not found in .env. Cannot determine local URL for replacement.")
			return errors.New("WORDPRESS_PORT not set in .env")
		}
		localURL := fmt.Sprintf("http://localhost:%s", localWPPort)

		printInfo("Updating database URLs (via wordpress container)...", fmt.Sprintf("Replacing '%s' with '%s'", productionURL, localURL))
		if errSR := wpCLI(ctx, "search-replace", productionURL, localURL, "--all-tables", "--quiet"); errSR != nil {
			printError("URL Update Failed.", errSR.Error())
			return errSR
		}
		printSuccess("URLs Updated Successfully in the database.")
	} else {
		printInfo("Skipping URL update.")
	}
	return nil
}

// dbExport handles exporting a database.
func dbExport(ctx context.Context) error {
	printInfo("Starting database export process...")
	timestamp := time.Now().Format("20060102-150405")
	currentDir, _ := os.Getwd() // Default to current directory for export
	defaultFileName := filepath.Join(currentDir, "db", fmt.Sprintf("db-export-%s.sql", timestamp))
	var exportFileHostPath string

	fileInputForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Export SQL File Path on Host").
				Description(fmt.Sprintf("Enter local path to save the SQL export (default: %s).", defaultFileName)).
				Value(&exportFileHostPath).
				Placeholder(defaultFileName).
				Validate(func(s string) error { // Basic validation for non-empty, can be expanded
					if s == "" && defaultFileName == "" { // Only error if both are empty
						return errors.New("export file path cannot be empty if default is not set")
					}
					return nil
				}),
		),
	).WithTheme(huhTheme)

	if err := fileInputForm.Run(); err != nil {
		printInfo("Export prompt cancelled.")
		return nil // User cancelled
	}
	if exportFileHostPath == "" {
		exportFileHostPath = defaultFileName
	}

	// Expand ~ if present
	if strings.HasPrefix(exportFileHostPath, "~"+string(os.PathSeparator)) || exportFileHostPath == "~" {
		home, errHome := os.UserHomeDir()
		if errHome != nil {
			printError("Error resolving home directory for export path.", errHome.Error())
			return errHome
		}
		if exportFileHostPath == "~" {
			exportFileHostPath = filepath.Join(home, fmt.Sprintf("db-export-%s.sql", timestamp))
		} else {
			exportFileHostPath = filepath.Join(home, exportFileHostPath[2:])
		}
	}

	absExportFileHostPath, err := filepath.Abs(exportFileHostPath)
	if err != nil {
		printError("Invalid Export File Path", fmt.Sprintf("Could not determine absolute path for %s: %v", exportFileHostPath, err))
		return err
	}

	if err := os.MkdirAll(filepath.Dir(absExportFileHostPath), 0755); err != nil {
		printError("Cannot Create Export Directory", fmt.Sprintf("Failed to create parent directory for %s: %v", absExportFileHostPath, err))
		return err
	}

	dbUser, dbPassword, dbName, _, errCred := getDBCredentials()
	if errCred != nil {
		printError("DB Credentials Error", errCred.Error())
		return errCred
	}

	var confirmUpdateURLs bool
	urlConfirmForm := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Update URLs for Live Site in Export?").
				Description("Update local development URLs to production URLs in the database *before* exporting? This change will be reverted in the local database after export.").
				Affirmative("Yes, update URLs in export").
				Negative("No, export with current (local) URLs").
				Value(&confirmUpdateURLs),
		),
	).WithTheme(huhTheme)

	if err := urlConfirmForm.Run(); err != nil {
		printInfo("URL update prompt for export cancelled.")
		return nil // User cancelled
	}

	var productionURLForRevert, localURLForRevert string
	if confirmUpdateURLs {
		localWPPort := os.Getenv("WORDPRESS_PORT")
		if localWPPort == "" {
			printWarning("WORDPRESS_PORT not found in .env. Cannot determine local URL for replacement.")
			return errors.New("WORDPRESS_PORT not set in .env")
		}
		localURL := fmt.Sprintf("http://localhost:%s", localWPPort)

		productionURL, errProdURL := getProductionURLFromEnvOrPrompt(ctx)
		if errProdURL != nil {
			printError("Failed to Get Production URL for export.", errProdURL.Error())
			return errProdURL
		}

		productionURLForRevert = productionURL
		localURLForRevert = localURL

		printInfo("Temporarily updating URLs in database for export (via wordpress container)...", fmt.Sprintf("Replacing '%s' with '%s'", localURL, productionURL))
		if errSR := wpCLI(ctx, "search-replace", localURL, productionURL, "--all-tables", "--quiet"); errSR != nil {
			printError("Temporary URL Update for Export Failed.", errSR.Error())
			return errSR
		}
		printSuccess("URLs temporarily updated in database for export.")
	}

	printInfo(fmt.Sprintf("Exporting database '%s' from 'db' container to host file: %s", dbName, absExportFileHostPath))

	dumpCmdArgs := []string{"compose", "exec", "-T", "db", "mysqldump", "--no-tablespaces", "-u" + dbUser}
	if dbPassword != "" {
		dumpCmdArgs = append(dumpCmdArgs, "-p"+dbPassword)
	}
	dumpCmdArgs = append(dumpCmdArgs, dbName)

	cmd := exec.CommandContext(ctx, "docker", dumpCmdArgs...)

	outfile, err := os.Create(absExportFileHostPath)
	if err != nil {
		printError("Failed to create export file on host", err.Error())
		if confirmUpdateURLs {
			printWarning("Attempting to revert URL changes in local database due to export file creation failure...")
			_ = wpCLI(context.Background(), "search-replace", productionURLForRevert, localURLForRevert, "--all-tables", "--quiet")
		}
		return err
	}
	defer outfile.Close()
	cmd.Stdout = outfile

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		printError("Database Export Failed.", fmt.Sprintf("Docker command failed: %v", err), "Stderr: "+stderr.String())
		if confirmUpdateURLs {
			printWarning("Attempting to revert URL changes in local database due to export failure...")
			_ = wpCLI(context.Background(), "search-replace", productionURLForRevert, localURLForRevert, "--all-tables", "--quiet")
		}
		return err
	}
	printSuccess("Database Exported Successfully to host:", absExportFileHostPath)

	if confirmUpdateURLs {
		printInfo("Reverting URL changes in the local database (via wordpress container)...")
		if errSRRevert := wpCLI(ctx, "search-replace", productionURLForRevert, localURLForRevert, "--all-tables", "--quiet"); errSRRevert != nil {
			printWarning("Failed to revert URL changes in the database.", errSRRevert.Error(), "Your local site URLs might now point to production! Please check manually.")
		} else {
			printSuccess("Local database URLs reverted.")
		}
	}
	return nil
}

// createTarGz creates a gzipped tar archive of the source directory.
func createTarGz(sourceDir, targetFile string) error {
	outFile, err := os.Create(targetFile)
	if err != nil {
		return fmt.Errorf("error creating archive file %s: %w", targetFile, err)
	}
	defer outFile.Close()

	gzipWriter := gzip.NewWriter(outFile)
	if gzipWriter == nil { // Should not happen with os.Create result
		return fmt.Errorf("failed to create gzip writer")
	}
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	sourceDir = filepath.Clean(sourceDir)

	// Walk the source directory
	return filepath.Walk(sourceDir, func(filePath string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Skip the source directory itself if needed, depends on desired archive structure
		// if filePath == sourceDir { return nil }

		// Create header
		header, err := tar.FileInfoHeader(info, info.Name()) // Use file's name as link name
		if err != nil {
			return fmt.Errorf("error creating tar header for %s: %w", filePath, err)
		}

		// IMPORTANT: Modify header name to be relative to the sourceDir base
		relPath, err := filepath.Rel(filepath.Dir(sourceDir), filePath) // Path relative to parent of sourceDir
		if err != nil {
			return fmt.Errorf("could not get relative path for %s: %w", filePath, err)
		}
		header.Name = relPath // Store relative path in tar header

		// Write header
		if err := tarWriter.WriteHeader(header); err != nil {
			return fmt.Errorf("error writing tar header for %s: %w", header.Name, err)
		}

		// If not a directory, write file content
		if !info.IsDir() {
			file, err := os.Open(filePath)
			if err != nil {
				return fmt.Errorf("error opening file %s: %w", filePath, err)
			}
			defer file.Close()
			if _, err := io.Copy(tarWriter, file); err != nil {
				return fmt.Errorf("error writing file %s to tar: %w", filePath, err)
			}
		}
		return nil
	})
}

// extractTarGz extracts a gzipped tar archive to the destination directory.
func extractTarGz(sourceFile, destinationDir string) error {
	file, err := os.Open(sourceFile)
	if err != nil {
		return fmt.Errorf("error opening archive %s: %w", sourceFile, err)
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("error creating gzip reader for %s: %w", sourceFile, err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return fmt.Errorf("error reading tar header: %w", err)
		}

		// Construct the full path for the file/directory within the destination
		targetPath := filepath.Join(destinationDir, header.Name)

		// Check file type
		switch header.Typeflag {
		case tar.TypeDir:
			// Create directory if it doesn't exist
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("error creating directory %s: %w", targetPath, err)
			}
		case tar.TypeReg:
			// Create file
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("error creating parent directory for %s: %w", targetPath, err)
			}

			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("error creating file %s: %w", targetPath, err)
			}
			// Use defer inside loop needs care, maybe close immediately?
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close() // Close on error too
				return fmt.Errorf("error writing file content %s: %w", targetPath, err)
			}
			outFile.Close() // Close successfully written file
		case tar.TypeSymlink:
			// Handle symlinks if necessary - more complex, requires os.Symlink
			printWarning("Skipping symlink extraction:", header.Name, header.Linkname)
		default:
			printWarning(fmt.Sprintf("Unsupported tar entry type %c for file %s", header.Typeflag, header.Name))

		}
	}
	return nil
}

func cmdRestore(ctx context.Context) error {
	printSectionHeader("Restore Instance from Backup")

	backupDir := backupsDirDefault
	// List potential backup sets (look for .sql files)
	sqlFiles := []string{}
	err := filepath.WalkDir(backupDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		} // Propagate errors
		if !d.IsDir() && strings.HasSuffix(path, ".sql") && strings.HasPrefix(d.Name(), "db-backup-") {
			sqlFiles = append(sqlFiles, path)
		}
		return nil
	})

	if err != nil {
		printError("Error Listing Backups", fmt.Sprintf("Could not scan '%s': %v", backupDir, err))
		return err
	}
	if len(sqlFiles) == 0 {
		printWarning("No Backups Found", fmt.Sprintf("No database backup files (*.sql) found in '%s'.", backupDir))
		return nil
	}

	// Let user select the SQL file (implies the set)
	var selectedSqlFile string
	sqlOptions := make([]huh.Option[string], len(sqlFiles))
	for i, f := range sqlFiles {
		// Display just the filename, value is the full path
		sqlOptions[i] = huh.NewOption(filepath.Base(f), f)
	}
	sort.Slice(sqlOptions, func(i, j int) bool { // Sort by filename descending (newest first)
		return sqlOptions[i].Key > sqlOptions[j].Key
	})

	selectBackup := huh.NewSelect[string]().
		Title("Select Database Backup to Restore").
		Description("This will also attempt to restore the corresponding wp-content archive and plugins.").
		Options(sqlOptions...).
		Value(&selectedSqlFile)

	if err := selectBackup.WithTheme(huhTheme).Run(); err != nil || selectedSqlFile == "" {
		printInfo("Restore Cancelled.")
		return nil
	}

	// Derive corresponding files archive name
	baseName := strings.TrimSuffix(filepath.Base(selectedSqlFile), ".sql") // e.g., db-backup-2023...
	timeStampPart := strings.TrimPrefix(baseName, "db-backup-")
	filesArchiveName := fmt.Sprintf("wp-content-backup-%s.tar.gz", timeStampPart)
	filesArchivePath := filepath.Join(backupDir, filesArchiveName)
	pluginsBackupFile := filepath.Join(backupDir, fmt.Sprintf("active-plugins-%s.json", timeStampPart))

	printInfo("Selected Backup Set:",
		fmt.Sprintf("Database: %s", filepath.Base(selectedSqlFile)),
		fmt.Sprintf("Files:    %s", filesArchiveName),
		fmt.Sprintf("Plugins:  %s", filepath.Base(pluginsBackupFile)))

	// Check if files archive exists
	if _, err := os.Stat(filesArchivePath); os.IsNotExist(err) {
		printError("File Archive Missing", fmt.Sprintf("Corresponding file archive '%s' not found.", filesArchiveName), "Cannot proceed with restore.")
		return err
	}

	// Check if plugins backup exists
	if _, err := os.Stat(pluginsBackupFile); os.IsNotExist(err) {
		printWarning("Plugins Backup Missing", fmt.Sprintf("Corresponding plugins backup '%s' not found. Plugins will not be restored.", filepath.Base(pluginsBackupFile)))
	}

	// Strong Confirmation
	var confirm bool
	confirmPrompt := huh.NewConfirm().
		Title("CONFIRM RESTORE").
		Description(errorMsgStyle.Render("WARNING: This will completely overwrite your current database and wp-content directory!") +
			fmt.Sprintf("\n\nRestore from:\n- DB: %s\n- Files: %s\n- Plugins: %s\n\nAre you absolutely sure?",
				filepath.Base(selectedSqlFile), filesArchiveName, filepath.Base(pluginsBackupFile))).
		Affirmative("Yes, OVERWRITE and restore").
		Negative("No, cancel")
	_ = confirmPrompt.Value(&confirm).WithTheme(huhTheme).Run()
	if !confirm {
		printInfo("Restore Cancelled.")
		return nil
	}

	// Proceed with restore
	printInfo("Stopping services before restore...")
	if err := cmdStop(ctx); err != nil {
		printError("Failed to stop services. Restore aborted.")
		return err
	} // Stop if stop fails

	// Start services so containers are running for import
	printInfo("Starting services for restore...")
	if err := cmdStart(ctx); err != nil {
		printError("Failed to start services for restore. Restore aborted.")
		return err
	}

	// Restore Database
	printInfo("Importing database...", fmt.Sprintf("File: %s", selectedSqlFile))
	if err := wpCLI(ctx, "db", "import", selectedSqlFile); err != nil {
		printError("Database Import Failed", err.Error())
		return err // Stop if DB import fails
	}
	printSuccess("Database Restored")

	// Restore Files
	printInfo("Extracting wp-content files...", fmt.Sprintf("Archive: %s", filesArchivePath))
	// We need to extract *into* the current directory, potentially overwriting wp-content
	// ExtractTarGz should handle overwriting. We extract to "."
	if err := extractTarGz(filesArchivePath, "."); err != nil {
		printError("File Extraction Failed", err.Error())
		return err
	}
	printSuccess("Files Restored")

	// Restore Plugins
	if _, err := os.Stat(pluginsBackupFile); err == nil {
		printInfo("Restoring active plugins from:", filepath.Base(pluginsBackupFile))
		pluginsData, err := os.ReadFile(pluginsBackupFile)
		if err != nil {
			printError("Failed to read plugins backup file", err.Error())
			return err
		}

		var plugins []map[string]interface{}
		if err := json.Unmarshal(pluginsData, &plugins); err != nil {
			printError("Failed to parse plugins backup file", err.Error())
			return err
		}

		for _, plugin := range plugins {
			if name, ok := plugin["name"].(string); ok {
				printInfo("Installing and activating plugin:", name)
				if err := wpCLI(ctx, "plugin", "install", name, "--activate"); err != nil {
					printWarning(fmt.Sprintf("Failed to restore plugin '%s'", name), err.Error())
				} else {
					printSuccess(fmt.Sprintf("Plugin '%s' restored and activated.", name))
				}
			}
		}
	} else {
		printWarning("Plugins Backup Missing", "Skipping plugin restoration.")
	}

	printInfo("Restarting services...")
	if err := cmdStart(ctx); err != nil {
		printWarning("Failed to restart services after restore.", err.Error())
	}

	printSuccess("Restore Complete!")
	return nil
}

func cmdBackup(ctx context.Context) error {
	printSectionHeader("Backup Instance")

	backupDir := backupsDirDefault
	timestamp := time.Now().Format("20060102-150405")
	dbBackupFile := filepath.Join(backupDir, fmt.Sprintf("db-backup-%s.sql", timestamp))
	filesBackupFile := filepath.Join(backupDir, fmt.Sprintf("wp-content-backup-%s.tar.gz", timestamp))
	pluginsBackupFile := filepath.Join(backupDir, fmt.Sprintf("active-plugins-%s.json", timestamp))

	// Ensure backup directory exists
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		printError("Failed to create backup directory", err.Error())
		return err
	}

	// Step 1: Backup Database
	printInfo("Exporting database to:", dbBackupFile)
	if err := dbExportToFile(ctx, dbBackupFile); err != nil {
		printError("Database Backup Failed", err.Error())
		return err
	}
	printSuccess("Database backup completed.")

	// Step 2: Backup wp-content
	printInfo("Creating wp-content archive:", filesBackupFile)
	if err := createTarGz("./wp-content", filesBackupFile); err != nil {
		printError("Failed to create wp-content archive", err.Error())
		return err
	}
	printSuccess("wp-content backup completed.")

	// Step 3: Backup Active Plugins
	printInfo("Saving active plugins to:", pluginsBackupFile)
	activePluginsJSON, err := getActivePluginsAsJSON(ctx)
	if err != nil {
		printError("Failed to retrieve active plugins", err.Error())
		return err
	}
	if err := os.WriteFile(pluginsBackupFile, []byte(activePluginsJSON), 0644); err != nil {
		printError("Failed to save active plugins JSON", err.Error())
		return err
	}
	printSuccess("Active plugins backup completed.")

	printSuccess("Backup Completed Successfully",
		fmt.Sprintf("Database: %s", dbBackupFile),
		fmt.Sprintf("Files: %s", filesBackupFile),
		fmt.Sprintf("Active Plugins: %s", pluginsBackupFile),
	)
	return nil
}

// dbExportToFile is a helper function to export the database to a specific file.
func dbExportToFile(ctx context.Context, filePath string) error {
	dbUser, dbPassword, dbName, _, errCred := getDBCredentials()
	if errCred != nil {
		return errCred
	}

	dumpCmdArgs := []string{"compose", "exec", "-T", "db", "mysqldump", "--no-tablespaces", "-u" + dbUser}
	if dbPassword != "" {
		dumpCmdArgs = append(dumpCmdArgs, "-p"+dbPassword)
	}
	dumpCmdArgs = append(dumpCmdArgs, dbName)

	cmd := exec.CommandContext(ctx, "docker", dumpCmdArgs...)

	outfile, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create export file: %w", err)
	}
	defer outfile.Close()
	cmd.Stdout = outfile

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("database export failed: %w\nStderr: %s", err, stderr.String())
	}
	return nil
}

// --- NEW Cache Clearing ---
func cmdClearCache(ctx context.Context) error {
	printSectionHeader("Clear WordPress Caches")
	printInfo("Flushing WordPress object cache...")
	err := wpCLI(ctx, "cache", "flush")
	if err != nil {
		printWarning("Failed to flush WP object cache.", err.Error())
		// Continue to try transients
	} else {
		printSuccess("WP object cache flushed.")
	}

	printInfo("Deleting all transients...")
	errTransients := wpCLI(ctx, "transient", "delete", "--all")
	if errTransients != nil {
		printWarning("Failed to delete transients.", errTransients.Error())
	} else {
		printSuccess("All transients deleted.")
	}

	// Return the first error encountered, if any
	if err != nil {
		return err
	}
	return errTransients
}

// --- NEW Convenience Commands ---
func cmdOpen(target string) error { // target can be "site", "admin", "mail"
	printSectionHeader(fmt.Sprintf("Opening %s Interface", strings.Title(target)))
	// Requires .env to be loaded
	wpPortStr := os.Getenv("WORDPRESS_PORT")
	mailPortStr := os.Getenv("MAILPIT_PORT_WEB")

	var url string
	switch target {
	case "site":
		if wpPortStr == "" {
			return errors.New("WORDPRESS_PORT not found in .env")
		}
		url = fmt.Sprintf("http://localhost:%s", wpPortStr)
	case "admin":
		if wpPortStr == "" {
			return errors.New("WORDPRESS_PORT not found in .env")
		}
		url = fmt.Sprintf("http://localhost:%s/wp-admin/", wpPortStr)
	case "mail":
		if mailPortStr == "" {
			return errors.New("MAILPIT_PORT_WEB not found in .env")
		}
		url = fmt.Sprintf("http://localhost:%s", mailPortStr)
	default:
		return fmt.Errorf("unknown open target: %s", target)
	}

	printInfo("Attempting to open:", commandStyle.Render(url))
	err := browser.OpenURL(url)
	if err != nil {
		printError("Failed to Open URL", fmt.Sprintf("Could not open %s in browser.", url), err.Error())
		return err
	}
	printSuccess("Opened in browser (hopefully!).")
	return nil
}

func extractDBVersion(raw string) string {
	// Example raw: "MySQL version: 8.0.23" or just "8.0.23" from some wp-cli versions
	re := regexp.MustCompile(`(?:(\d+\.\d+\.\d+)|(\d+\.\d+))`) // Matches X.Y.Z or X.Y
	match := re.FindStringSubmatch(raw)
	if len(match) > 1 {
		for i := 1; i < len(match); i++ {
			if match[i] != "" {
				return match[i]
			}
		}
	}
	// Fallback if regex fails, take last word
	parts := strings.Fields(raw)
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return "Unknown"
}

func generateRandomString(length int) (string, error) {
	// Simplified from previous manager - no crypto/rand for this password suggestion
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		// This is not cryptographically secure, but fine for a suggested password
		// For truly secure, use crypto/rand as in the global manager.
		// However, wp-cli takes it as input, so it's more about the user seeing it.
		result[i] = chars[time.Now().UnixNano()%int64(len(chars))]
	}
	return string(result), nil
}

func cmdShowStatus() {
	printSectionHeader("Instance Status")
	meta, err := readLocalMeta()
	if err != nil {
		printError("Could Not Read Metadata", err.Error())
		return
	}

	ctx := context.Background()
	dockerPSOutput, _ := runCommandGetOutput(ctx, "docker", "compose", "ps", "--format", "json")
	// The output from docker compose ps --format json is a stream of JSON objects, one per line.
	// We need to parse this.

	var services []struct {
		Name    string `json:"Name"`
		Service string `json:"Service"` // docker compose ps adds this, `docker ps` uses 'Names'
		State   string `json:"State"`
		Status  string `json:"Status"` // More detailed, like "Up 2 minutes" or "Exited (0)"
		Health  string `json:"Health,omitempty"`
	}

	decoder := json.NewDecoder(strings.NewReader(dockerPSOutput))
	for {
		var s struct {
			Name    string `json:"Name"`
			Service string `json:"Service"`
			State   string `json:"State"`
			Status  string `json:"Status"`
			Health  string `json:"Health,omitempty"`
		}
		if err := decoder.Decode(&s); err == io.EOF {
			break
		} else if err != nil {
			// printWarning("Could not parse Docker status line", err.Error()) // Can be noisy
			continue
		}
		services = append(services, s)
	}

	var statusOutput []string
	statusOutput = append(statusOutput, fmt.Sprintf("%s: %s", boldStyle.Render("WordPress Version"), meta.WordPressVersion))
	statusOutput = append(statusOutput, fmt.Sprintf("%s: %s", boldStyle.Render("Database Version"), meta.DBVersion))
	statusOutput = append(statusOutput, fmt.Sprintf("%s: %s", boldStyle.Render("Overall Status (from meta)"), meta.Status))
	statusOutput = append(statusOutput, "")
	statusOutput = append(statusOutput, boldStyle.Render("Docker Container Status:"))

	if len(services) == 0 {
		statusOutput = append(statusOutput, listItemStyle.Render(subtleStyle.Render("No running or stopped containers found for this instance.")))
	} else {
		for _, s := range services {
			serviceName := s.Service
			if serviceName == "" {
				serviceName = s.Name
			} // Fallback if Service field is empty

			stateColor := lipgloss.NewStyle()
			if strings.Contains(strings.ToLower(s.State), "running") || strings.Contains(strings.ToLower(s.State), "up") {
				stateColor = successTitle
			} else if strings.Contains(strings.ToLower(s.State), "exited") || strings.Contains(strings.ToLower(s.State), "stopped") {
				stateColor = warningTitle
			} else {
				stateColor = errorTitle
			}
			statusLine := fmt.Sprintf("%s: %s (%s)",
				boldStyle.Render(serviceName),
				stateColor.Render(s.State),
				subtleStyle.Render(s.Status))
			if s.Health != "" {
				statusLine += fmt.Sprintf(" - Health: %s", s.Health)
			}
			statusOutput = append(statusOutput, listItemStyle.Render(statusLine))
		}
	}
	printInfo("Current Instance Details", statusOutput...)
}

// --- Show Ports/Addresses Command ---
func cmdShowPorts() {
	printSectionHeader("Instance Ports & Addresses")
	// Load .env file
	envMap, err := godotenv.Read(envFileName)
	if err != nil {
		printError("Failed to read .env file", err.Error())
		return
	}

	// Filter for likely port/address variables
	var rows [][2]string
	for k, v := range envMap {
		uk := strings.ToUpper(k)
		if strings.Contains(uk, "PORT") || strings.Contains(uk, "ADDR") || strings.Contains(uk, "ADDRESS") || strings.Contains(uk, "HOST") || strings.Contains(uk, "URL") {
			rows = append(rows, [2]string{k, v})
		}
	}
	if len(rows) == 0 {
		printWarning("No port or address variables found in .env")
		return
	}

	// Sort rows by variable name
	sort.Slice(rows, func(i, j int) bool { return rows[i][0] < rows[j][0] })

	// Table styles
	headStyle := lipgloss.NewStyle().Bold(true).Foreground(colorPrimary)
	rowKeyStyle := lipgloss.NewStyle().Foreground(colorSecondary)
	rowValStyle := lipgloss.NewStyle().Foreground(colorInfo)

	table := headStyle.Render("Variable") + "\t" + headStyle.Render("Value") + "\n"
	for _, row := range rows {
		table += rowKeyStyle.Render(row[0]) + "\t" + rowValStyle.Render(row[1]) + "\n"
	}
	fmt.Println(lipgloss.NewStyle().Padding(1).Border(lipgloss.NormalBorder()).Render(table))
}

// --- Main ---
func main() {
	// Define flags
	flag.BoolVar(&verbose, "v", false, "Enable verbose output")
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose output")
	flagXdebugOn := flag.NewFlagSet("xdebug-on", flag.ExitOnError)
	flagXdebugOff := flag.NewFlagSet("xdebug-off", flag.ExitOnError)
	// Add other flags here if needed

	// Custom flag usage message
	flag.Usage = func() {
		// Use our styled help printer instead of default flag usage
		showHelp()
	}

	flag.Parse() // Parse flags first

	// Determine instance name for title
	instanceName := "WordPress Instance"
	if pwd, err := os.Getwd(); err == nil {
		instanceName = filepath.Base(pwd)
	}
	printAppTitle(fmt.Sprintf("Managing: %s", instanceName))

	// Load .env variables AFTER parsing flags, as loading might print info
	loadEnvOrFail() // Exits on failure

	// Use flag.Args() for positional arguments (the command)
	args := flag.Args()
	if len(args) < 1 {
		showHelp()
		os.Exit(0) // Exit cleanly after showing help if no command given
	}

	action := strings.ToLower(args[0])
	actionArgs := args[1:] // Remaining args for commands like wpcli

	// Use a cancellable context for commands
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop() // Ensure context cancellation is cleaned up

	var cmdErr error // Variable to store error from commands

	switch action {
	case "start":
		cmdErr = cmdStart(ctx)
	case "stop":
		cmdErr = cmdStop(ctx)
	case "restart":
		cmdErr = cmdRestart(ctx)
	case "update":
		cmdErr = cmdUpdate(ctx)
	case "console":
		cmdErr = cmdConsole(ctx)
	case "logs":
		cmdErr = cmdLogs(ctx)
	case "status":
		cmdShowStatus() // Status doesn't need context and doesn't return error currently
	case "install":
		cmdErr = cmdWPInstall(ctx)
	case "plugins":
		cmdErr = cmdManagePlugins(ctx)
	case "themes":
		cmdErr = cmdManageThemes(ctx) // Added themes command
	case "users":
		cmdErr = cmdManageUsers(ctx) // Added users command
	case "restore":
		cmdErr = cmdRestore(ctx)
	case "backup":
		cmdErr = cmdBackup(ctx)
	case "cache":
		cmdErr = cmdClearCache(ctx) // Added cache command
	case "open":
		cmdErr = cmdOpen("site") // Added open command
	case "browse":
		cmdErr = cmdOpen("site") // Alias for open
	case "admin":
		cmdErr = cmdOpen("admin") // Added admin command
	case "mail":
		cmdErr = cmdOpen("mail") // Added mail command
	case "wpcli":
		if len(actionArgs) == 0 {
			printError("No WP-CLI Command Provided", "Usage: ./manage wpcli <your wp-cli arguments>")
			os.Exit(1)
		}
		cmdErr = cmdRunWPCLI(ctx, actionArgs) // Pass context and remaining args
	case "fix-perms": // New command
		cmdErr = cmdFixWPContentPermissions(ctx)
	case "db": // New unified command
		cmdErr = cmdDB(ctx)
	case "prod-check":
		cmdErr = cmdProdCheck(ctx)
	case "prod-prep":
		cmdErr = cmdProdPrep(ctx)
	case "mysql-logs": // New command
		cmdErr = cmdMySQLLogs(ctx)
	case "show-ports":
		cmdShowPorts()
	case "help", "--help", "-h":
		showHelp()
	case "xdebug": // New command
		if len(actionArgs) < 1 {
			printError("No Xdebug action provided", "Usage: ./manage xdebug <enable|disable>")
			os.Exit(1)
		}
		enable := strings.ToLower(actionArgs[0]) == "enable"
		cmdErr = cmdXdebug(ctx, enable)
	case "xdebug-on":
		ctx := context.Background()
		_ = flagXdebugOn.Parse(flag.Args()[1:])
		cmdXdebug(ctx, true)
	case "xdebug-off":
		ctx := context.Background()
		_ = flagXdebugOff.Parse(flag.Args()[1:])
		cmdXdebug(ctx, false)
	case "detail-status":
		cmdErr = cmdDetailStatus(ctx)
	default:
		printError("Unknown Command", fmt.Sprintf("Command '%s' is not recognized.", action))
		showHelp()
		os.Exit(1)
	}

	// Check if any command returned an error
	if cmdErr != nil {
		// Specific errors should have been printed by the command function
		// Just exit with non-zero status
		os.Exit(1)
	}
}

// cmdRunWPCLI modified to accept args and context
func cmdRunWPCLI(ctx context.Context, wpcliArgs []string) error {
	printSectionHeader("Run Custom WP-CLI Command")
	printInfo("Executing WP-CLI:", commandStyle.Render(strings.Join(wpcliArgs, " ")))
	fmt.Println(subtleStyle.Render("--- WP-CLI Output Start ---"))
	err := wpCLI(ctx, wpcliArgs...) // Pass context and args
	fmt.Println(subtleStyle.Render("--- WP-CLI Output End ---"))
	if err != nil {
		printError("WP-CLI Command Failed") // Error details should have been streamed
	} else {
		printSuccess("WP-CLI Command Executed")
	}
	return err // Return the error status
}

// showHelp updated with new commands
func showHelp() {
	printSectionHeader("WordPress Instance Manager - Help")
	helpText := []string{
		boldStyle.Render("Service Management:"),
		fmt.Sprintf("  %s %s", commandStyle.Render("start"), subtleStyle.Render("- Start Docker services")),
		fmt.Sprintf("  %s %s", commandStyle.Render("stop"), subtleStyle.Render("- Stop Docker services")),
		fmt.Sprintf("  %s %s", commandStyle.Render("restart"), subtleStyle.Render("- Restart Docker services")),
		fmt.Sprintf("  %s %s", commandStyle.Render("update"), subtleStyle.Render("- Pull Docker images & recreate services")),
		fmt.Sprintf("  %s %s", commandStyle.Render("status"), subtleStyle.Render("- Show instance and Docker container status")),
		fmt.Sprintf("  %s %s", commandStyle.Render("logs"), subtleStyle.Render("- Stream Docker service logs")),
		fmt.Sprintf("  %s %s", commandStyle.Render("console"), subtleStyle.Render("- Open a bash console in the WordPress container")),
		"",
		boldStyle.Render("WordPress Management:"),
		fmt.Sprintf("  %s %s", commandStyle.Render("install"), subtleStyle.Render("- Run initial WordPress installation wizard")),
		fmt.Sprintf("  %s %s", commandStyle.Render("plugins"), subtleStyle.Render("- Manage plugins (install, update, toggle, delete)")),
		fmt.Sprintf("  %s %s", commandStyle.Render("themes"), subtleStyle.Render("- Manage themes (install, update, activate, delete)")),
		fmt.Sprintf("  %s %s", commandStyle.Render("users"), subtleStyle.Render("- Manage users (list, create, update, delete)")),
		fmt.Sprintf("  %s %s", commandStyle.Render("cache"), subtleStyle.Render("- Clear WP object cache and transients")),
		fmt.Sprintf("  %s %s", commandStyle.Render("xdebug"), subtleStyle.Render("- Enable or disable Xdebug in the WordPress container")),
		fmt.Sprintf("  %s %s", commandStyle.Render("xdebug-on"), subtleStyle.Render("- Enable Xdebug")),
		fmt.Sprintf("  %s %s", commandStyle.Render("xdebug-off"), subtleStyle.Render("- Disable Xdebug")),
		"",
		boldStyle.Render("Database Operations:"), // Updated Section
		fmt.Sprintf("  %s %s", commandStyle.Render("db"), subtleStyle.Render("- Interactive import/export database (handles URL updates)")),
		fmt.Sprintf("    %s %s", commandStyle.Render("import"), subtleStyle.Render("- Import a database from a SQL file")),
		fmt.Sprintf("    %s %s", commandStyle.Render("export"), subtleStyle.Render("- Export the database to a SQL file")),
		fmt.Sprintf("  %s %s", commandStyle.Render("mysql-logs"), subtleStyle.Render("- Fetch MySQL logs from the container")),
		fmt.Sprintf("  %s %s", commandStyle.Render("backup"), subtleStyle.Render("- Backup the database and wp-content directory")),
		fmt.Sprintf("  %s %s", commandStyle.Render("restore"), subtleStyle.Render("- Restore the database and wp-content from a backup")),
		"",
		boldStyle.Render("Convenience:"),
		fmt.Sprintf("  %s %s", commandStyle.Render("open"), subtleStyle.Render("- Open instance site URL in browser")),
		fmt.Sprintf("  %s %s", commandStyle.Render("browse"), subtleStyle.Render("- (Alias for open)")),
		fmt.Sprintf("  %s %s", commandStyle.Render("admin"), subtleStyle.Render("- Open instance WP Admin URL in browser")),
		fmt.Sprintf("  %s %s", commandStyle.Render("mail"), subtleStyle.Render("- Open Mailpit web UI in browser")),
		fmt.Sprintf("  %s %s", commandStyle.Render("show-ports"), subtleStyle.Render("- Show all ports and addresses from .env in a table")),
		"",
		boldStyle.Render("WP-CLI Passthrough:"),
		fmt.Sprintf("  %s %s", commandStyle.Render("wpcli <args...>"), subtleStyle.Render("- Execute any raw WP-CLI command")),
		fmt.Sprintf("    %s %s", subtleStyle.Render("Example:"), commandStyle.Render("wpcli plugin list --status=active")),
		"",
		boldStyle.Render("Filesystem & Permissions:"), // New section or add to existing
		fmt.Sprintf("  %s %s", commandStyle.Render("fix-perms"), subtleStyle.Render("- Set wp-content host permissions for www-data group access")),
		"",
		boldStyle.Render("Production & Security:"),
		fmt.Sprintf("  %s %s", commandStyle.Render("prod-check"), subtleStyle.Render("- Run checks for production readiness")),
		fmt.Sprintf("  %s %s", commandStyle.Render("prod-prep"), subtleStyle.Render("- Guide and assist in preparing for production")),
		"",
		boldStyle.Render("Docker Details (Lazydocker):"),
		fmt.Sprintf("  %s %s", commandStyle.Render("detail-status"), subtleStyle.Render("- Launch lazydocker filtered to this instance's containers")),
		"",
		boldStyle.Render("Other:"),
		fmt.Sprintf("  %s %s", commandStyle.Render("help"), subtleStyle.Render("- Show this help message")),
		fmt.Sprintf("  %s %s", commandStyle.Render("-v, --verbose"), subtleStyle.Render("- Enable verbose output for debugging")),
	}
	// Use a different style for the overall help box if desired
	fmt.Println("\n" + lipgloss.NewStyle().Padding(1).Render(lipgloss.JoinVertical(lipgloss.Left, helpText...)))
}

// New command to fetch MySQL logs
func cmdMySQLLogs(ctx context.Context) error {
	printSectionHeader("MySQL Container Log File")
	// Common MySQL log file locations
	logPaths := []string{
		"/var/log/mysql/error.log",
		"/var/log/mysql/mysqld.log",
		"/var/log/mysql.log",
		"/var/log/mysqld.log",
	}
	var found bool
	for _, logPath := range logPaths {
		printVerbose("Checking for log file:", logPath)
		cmd := exec.CommandContext(ctx, "docker", "compose", "exec", "-T", "mysql", "sh", "-c", "test -f "+logPath+" && cat "+logPath)
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		if err := cmd.Run(); err == nil && out.Len() > 0 {
			printSuccess("MySQL log file found:", logPath)
			fmt.Println(out.String())
			found = true
			break
		}
	}
	if !found {
		printError("No MySQL log file found in common locations.")
		printInfo("Checked:", strings.Join(logPaths, ", "))
	}
	return nil
}

// --- Xdebug Toggle Command ---
func cmdXdebug(ctx context.Context, enable bool) error {
	printSectionHeader("Toggling Xdebug in WordPress Container")
	var action, dockerCmd string
	if enable {
		action = "Enabling"
		dockerCmd = "docker-php-ext-enable xdebug"
	} else {
		action = "Disabling"
		dockerCmd = "docker-php-ext-disable xdebug"
	}
	printInfo(action+" Xdebug...", "This may take a few seconds.")
	err := runCommand(ctx, "docker", "compose", "exec", "wordpress", "bash", "-c", dockerCmd)
	if err != nil {
		printError("Failed to "+strings.ToLower(action)+" Xdebug", err.Error())
		return err
	}
	printInfo("Restarting Apache in container...")
	err = runCommand(ctx, "docker", "compose", "exec", "wordpress", "apache2ctl", "restart")
	if err != nil {
		printWarning("Failed to restart Apache. You may need to restart the container manually.", err.Error())
	}
	// Check status
	output, _ := runCommandGetOutput(ctx, "docker", "compose", "exec", "wordpress", "php", "-m")
	if strings.Contains(output, "xdebug") {
		printSuccess("Xdebug is ENABLED in the container.")
	} else {
		printSuccess("Xdebug is DISABLED in the container.")
	}
	return nil
}

// --- Detail Status Command (Lazydocker) ---
func cmdDetailStatus(ctx context.Context) error {
	printSectionHeader("Detailed Docker Status (Lazydocker)")

	// Load .env to get COMPOSE_PROJECT_NAME
	envMap, err := godotenv.Read(envFileName)
	if err != nil {
		printError("Failed to read .env file", err.Error())
		return err
	}
	projectName := envMap["COMPOSE_PROJECT_NAME"]
	if projectName == "" {
		// Fallback: use current directory name
		if pwd, err := os.Getwd(); err == nil {
			projectName = filepath.Base(pwd)
			printInfo("COMPOSE_PROJECT_NAME not set in .env", "Using directory name as project: "+projectName)
		} else {
			printError("Could not determine project name from .env or directory.")
			return errors.New("no compose project name found")
		}
	}

	// Check if lazydocker is installed
	if _, err := exec.LookPath("lazydocker"); err != nil {
		printError("lazydocker is not installed.", "Please install lazydocker to use this command.")
		return err
	}

	printInfo("Launching lazydocker for project:", projectName)
	cmd := exec.CommandContext(ctx, "lazydocker", "--compose-project-name", projectName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
