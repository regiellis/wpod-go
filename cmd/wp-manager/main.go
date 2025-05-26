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
	"bytes"
	"crypto/rand"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

//go:embed all:templates/docker-default-wordpress
var defaultWordpressTemplate embed.FS
var caddyfileTemplateContent string

const (
	embeddedTemplateRoot = "templates/docker-default-wordpress"
	globalConfigFileName = ".wpid-config.json"
	metaFileName         = ".wordpress-meta.json"
	envTemplateFileName  = "env-template"
	managerMetaFileName  = ".wpid-instances.json"
	defaultWordPressPort = 8080
	defaultMailpitSMTP   = 1025
	defaultMailpitWeb    = 8025
)

// Initialize theme in init()
func init() {
	flag.BoolVar(&useLightTheme, "light", false, "Use light theme for light terminal backgrounds (toggle off with --light=false)")
}

// Function to load theme preference from global config
func loadThemePreferenceFromConfig() bool {
	globalConfig, err := readGlobalManagerConfig()
	if err == nil && globalConfig.Theme != "" {
		return globalConfig.Theme == "light"
	}
	return false
}

// Function to save theme preference to global config
func saveThemePreferenceToConfig(light bool) {
	globalConfig, err := readGlobalManagerConfig()
	if err != nil {
		globalConfig = GlobalManagerConfig{}
	}
	if light {
		globalConfig.Theme = "light"
	} else {
		globalConfig.Theme = "dark"
	}
	_ = writeGlobalManagerConfig(globalConfig)
}

// --- Metadata Types ---
type InstanceMeta struct {
	Directory        string `json:"directory"`
	CreationDate     string `json:"creation_date"`
	WordPressVersion string `json:"wordpress_version"`
	DBVersion        string `json:"db_version"`
	WordPressPort    int    `json:"wordpress_port"`
	Status           string `json:"status"`
}

// In cmd/wp-manager/main.go
type InstanceCaddyConfigData struct {
	InstanceName     string // Full instance directory name, e.g., www-myblog-wordpress
	DevHostName      string // e.g., myblog.example.local
	WordPressPort    int    // Host port WordPress is mapped to (for external access reference)
	InstancePort     int    // Port inside the container (default 80)
	CaddyHTTPPort    int    // NEW: for Caddyfile.template
	InstanceNameBase string // e.g., myblog (for subdomains like adminer.myblog...)
	DevDomainSuffix  string // e.g., .example.local (for subdomains)
}

type GlobalManagerConfig struct {
	SitesBaseDirectory string `json:"sites_base_directory,omitempty"`
	Theme              string `json:"theme,omitempty"`
}

// Represents the structure of the central manager metadata file
type ManagerMeta map[string]InstanceMeta

// --- Enhanced Helper Functions (Updated for new styles) ---
func printSectionHeader(msg string) {
	fmt.Println(sectionHeaderStyle.Render("╭─ " + msg))
}

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

// --- Helper function to check for executable ---
func checkExecutable(name string) (string, bool) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", false
	}
	return path, true
}

func generateRandomString(length int) (string, error) {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, length)
	for i := range result {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return "", err
		}
		result[i] = chars[num.Int64()]
	}
	return string(result), nil
}

func generateRandomName() (string, error) {
	adjectives := []string{"brave", "calm", "eager", "fancy", "gentle", "happy", "jolly", "kind", "lively", "mighty"}
	nouns := []string{"lion", "tiger", "eagle", "panda", "whale", "fox", "wolf", "bear", "shark", "falcon"}

	adjIndex, err := rand.Int(rand.Reader, big.NewInt(int64(len(adjectives))))
	if err != nil {
		return "", fmt.Errorf("failed to generate random adjective: %w", err)
	}

	nounIndex, err := rand.Int(rand.Reader, big.NewInt(int64(len(nouns))))
	if err != nil {
		return "", fmt.Errorf("failed to generate random noun: %w", err)
	}

	randomName := fmt.Sprintf("%s-%s", adjectives[adjIndex.Int64()], nouns[nounIndex.Int64()])
	return randomName, nil
}

func generateRandomPort() int {
	// Define a range for server ports that avoids well-known ports (0-1023) and common conflicts
	const minPort = 1024
	const maxPort = 12000 // Avoid dynamic/private ports (49152-65535)

	portRange := maxPort - minPort + 1
	portBig, err := rand.Int(rand.Reader, big.NewInt(int64(portRange)))
	if err != nil {
		return 0
	}
	return int(portBig.Int64()) + minPort
}

// --- Metadata Handling ---

func getConfigStorageDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		fallbackDir := ".wpid-data" // Fallback to a local directory in CWD
		// Only print warning once if multiple calls hit this fallback
		// This could be improved with a global flag if necessary
		if _, errStat := os.Stat(fallbackDir); os.IsNotExist(errStat) {
			printWarning("User config directory not found, using fallback directory in current path.", fallbackDir, err.Error())
		}
		if errMk := os.MkdirAll(fallbackDir, 0700); errMk != nil {
			return "", fmt.Errorf("could not create fallback data directory %s: %w", fallbackDir, errMk)
		}
		return fallbackDir, nil
	}

	wpidDataDir := filepath.Join(configDir, "wpid")
	if err := os.MkdirAll(wpidDataDir, 0700); err != nil { // 0700: owner rwx
		return "", fmt.Errorf("could not create data directory %s: %w", wpidDataDir, err)
	}
	return wpidDataDir, nil
}

// getGlobalConfigPath uses getConfigStorageDir
func getGlobalConfigPath() (string, error) {
	storageDir, err := getConfigStorageDir()
	if err != nil {
		return "", fmt.Errorf("failed to get storage directory for global config: %w", err)
	}
	return filepath.Join(storageDir, globalConfigFileName), nil
}

func getManagerMetaPath() (string, error) {
	storageDir, err := getConfigStorageDir()
	if err != nil {
		// If getConfigStorageDir returned the fallback path without error, use it.
		// If it returned an actual error (e.g. couldn't create fallback), propagate that.
		if (!strings.HasSuffix(storageDir, ".wpid-data") && storageDir != "") || storageDir == "" { // Check if it's not the successful fallback
			return "", fmt.Errorf("failed to get storage directory for manager meta: %w", err)
		}
	}
	return filepath.Join(storageDir, managerMetaFileName), nil
}

// File: cmd/wp-manager/main.go

func readGlobalManagerConfig() (GlobalManagerConfig, error) {
	config := GlobalManagerConfig{}
	configPath, err := getGlobalConfigPath()
	if err != nil {
		return config, err // Error getting path
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return config, nil // File doesn't exist, return empty (default) config
		}
		return config, fmt.Errorf("failed to read global config file %s: %w", configPath, err)
	}
	if len(data) == 0 {
		return config, nil // Empty file is fine, means no settings
	}
	if err := json.Unmarshal(data, &config); err != nil {
		printWarning("Global configuration file might be corrupt.", fmt.Sprintf("Error unmarshalling %s: %v. Using defaults/empty config.", configPath, err))
		return GlobalManagerConfig{}, nil // Return empty on unmarshal error
	}
	return config, nil
}

func writeGlobalManagerConfig(config GlobalManagerConfig) error {
	configPath, err := getGlobalConfigPath()
	if err != nil {
		return fmt.Errorf("could not determine global config path for writing: %w", err)
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal global config data: %w", err)
	}

	// Atomic write
	tempFile, err := os.CreateTemp(filepath.Dir(configPath), filepath.Base(configPath)+".*.tmp")
	if err != nil {
		return fmt.Errorf("could not create temp file for global config: %w", err)
	}
	tempPath := tempFile.Name()
	defer func() {
		if _, statErr := os.Stat(tempPath); statErr == nil { // Check if temp file still exists
			os.Remove(tempPath)
		}
	}()

	if _, err = tempFile.Write(data); err != nil {
		tempFile.Close()
		return fmt.Errorf("failed to write data to temp global config file %s: %w", tempPath, err)
	}
	if err = tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp global config file %s: %w", tempPath, err)
	}
	return os.Rename(tempPath, configPath)
}

// readManagerMeta reads the central instance metadata file.
// If the file doesn't exist, it returns an empty map and nil error.
func readManagerMeta() (ManagerMeta, error) {
	meta := make(ManagerMeta)
	metaPath, err := getManagerMetaPath()
	if err != nil {
		return nil, fmt.Errorf("could not determine manager meta path: %w", err)
	}

	data, err := os.ReadFile(metaPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return meta, nil // File doesn't exist is okay
		}
		return nil, fmt.Errorf("failed to read manager meta file %s: %w", metaPath, err)
	}

	if len(data) == 0 {
		return meta, nil
	} // Empty file is okay

	if err := json.Unmarshal(data, &meta); err != nil {
		var emptyCheck interface{}
		if json.Unmarshal(data, &emptyCheck) == nil {
			if m, ok := emptyCheck.(map[string]interface{}); ok && len(m) == 0 {
				return meta, nil
			}
		}
		printWarning("Manager metadata file might be corrupt.", fmt.Sprintf("Error unmarshalling %s: %v", metaPath, err))
		return make(ManagerMeta), nil
	}
	return meta, nil
}

// writeManagerMeta writes the central instance metadata file.
func writeManagerMeta(meta ManagerMeta) error {
	metaPath, err := getManagerMetaPath()
	if err != nil {
		return fmt.Errorf("could not determine manager meta path: %w", err)
	}

	// Add file locking for safety against concurrent writes
	lockPath := metaPath + ".lock"
	fileLock := NewFileLock(lockPath) // Assuming you have a simple file lock helper or use flock lib
	err = fileLock.Lock()
	if err != nil {
		return fmt.Errorf("could not acquire lock on metadata file %s: %w", lockPath, err)
	}
	defer fileLock.Unlock() // Ensure unlock happens

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manager meta data: %w", err)
	}

	// Write atomically if possible (write to temp then rename)
	tempFile, err := os.CreateTemp(filepath.Dir(metaPath), managerMetaFileName+".*.tmp")
	if err != nil {
		return fmt.Errorf("could not create temp file for metadata: %w", err)
	}
	tempPath := tempFile.Name()
	// Defer cleanup in case of errors after create but before rename
	defer func() {
		if _, statErr := os.Stat(tempPath); statErr == nil { // Check if temp file still exists
			os.Remove(tempPath)
		}
	}()

	if _, err = tempFile.Write(data); err != nil {
		tempFile.Close() // Close even on error
		return fmt.Errorf("failed to write data to temp metadata file %s: %w", tempPath, err)
	}
	if err = tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp metadata file %s: %w", tempPath, err)
	}

	// Rename temp file to actual file path (atomic on most Unix)
	if err = os.Rename(tempPath, metaPath); err != nil {
		return fmt.Errorf("failed to rename temp metadata file %s to %s: %w", tempPath, metaPath, err)
	}

	return nil
}

func handleConfigCommand(args []string) {
	if len(args) == 0 {
		printError("Config subcommand required.", "Usage: wpid config <get|set|show> <key> [value]")
		printInfo("Available keys for config: sites_base_directory")
		return
	}
	subcommand := strings.ToLower(args[0])
	switch subcommand {
	case "get":
		if len(args) < 2 {
			printError("Key required for 'get'.", "Usage: wpid config get <key>")
			return
		}
		configGet(args[1])
	case "set":
		if len(args) < 2 {
			printError("Key and value required for 'set'.", "Usage: wpid config set <key> <value>")
			return
		}
		key := args[1]
		var value string
		if len(args) > 2 {
			value = strings.Join(args[2:], " ")
		} // Allows spaces in path if quoted
		configSet(key, value)
	case "show", "view":
		configShow()
	default:
		printError("Unknown config subcommand.", fmt.Sprintf("Subcommand '%s' not recognized. Valid: get, set, show.", subcommand))
	}
}

func configShow() {
	printSectionHeader("Current Global Configuration")
	config, err := readGlobalManagerConfig()
	if err != nil {
		printError("Failed to read global configuration.", err.Error())
		return
	}

	configPath, pathErr := getGlobalConfigPath()
	if pathErr == nil {
		printInfo("Config File Location:", configPath)
	} else {
		printWarning("Could not determine config file path.")
	}

	if config.SitesBaseDirectory == "" {
		printInfo(infoMsgStyle.Render("Sites Base Directory:"), infoMsgStyle.Render("(not set)"))
		printInfo("  New instances will prompt for location or use the current directory.")
	} else {
		printInfo(infoMsgStyle.Render("Sites Base Directory:"), commandStyle.Render(config.SitesBaseDirectory))
	}
	// Add display for other global settings here if any
}

func configGet(key string) {
	config, err := readGlobalManagerConfig()
	if err != nil {
		printError("Failed to read global configuration.", err.Error())
		return
	}
	key = strings.ToLower(key)
	switch key {
	case "sites_base_directory":
		if config.SitesBaseDirectory == "" {
			printInfo("sites_base_directory is not set.")
		} else {
			fmt.Println(config.SitesBaseDirectory) // Raw output for scripting
		}
	default:
		printError("Unknown configuration key.", fmt.Sprintf("Key '%s' not recognized.", key))
	}
}

func configSet(key, value string) {
	config, err := readGlobalManagerConfig()
	if err != nil {
		printError("Failed to read existing global configuration.", err.Error())
		return
	}

	key = strings.ToLower(key)
	changed := false
	switch key {
	case "sites_base_directory":
		if value == "" { // User wants to unset
			if config.SitesBaseDirectory != "" {
				config.SitesBaseDirectory = ""
				changed = true
				printSuccess("Sites base directory has been unset.")
			} else {
				printInfo("Sites base directory is already unset. No changes made.")
			}
			break // from switch
		}
		// Validate and make absolute if setting a new path
		resolvedValue := value
		if strings.HasPrefix(value, "~") {
			home, _ := os.UserHomeDir()
			if home != "" {
				resolvedValue = filepath.Join(home, value[1:])
			}
		}
		absPath, errAbs := filepath.Abs(resolvedValue)
		if errAbs != nil {
			printError("Invalid path value.", fmt.Sprintf("Could not determine absolute path for '%s': %v", value, errAbs))
			return
		}

		// Check if path exists and is a directory (optional, can be informational)
		info, errStat := os.Stat(absPath)
		if os.IsNotExist(errStat) {
			printWarning("Path Verification", fmt.Sprintf("Directory '%s' does not currently exist. It will be used for new instances and created if needed.", absPath))
		} else if errStat != nil {
			printError("Path Error", fmt.Sprintf("Error accessing path '%s': %v", absPath, errStat))
			return
		} else if !info.IsDir() {
			printError("Invalid Path", fmt.Sprintf("Path '%s' exists but is not a directory.", absPath))
			return
		}

		if config.SitesBaseDirectory != absPath {
			config.SitesBaseDirectory = absPath
			changed = true
			printSuccess("Sites base directory set to:", commandStyle.Render(absPath))
		} else {
			printInfo("Sites base directory is already set to this path. No changes made.")
		}
	default:
		printError("Unknown configuration key.", fmt.Sprintf("Key '%s' is not recognized for setting.", key))
		return
	}

	if changed {
		if err := writeGlobalManagerConfig(config); err != nil {
			printError("Failed to write updated global configuration.", err.Error())
		}
	}
}

// Simple file lock helper (replace with gofrs/flock for production)
type FileLock struct{ path string }

func NewFileLock(path string) *FileLock { return &FileLock{path: path} }
func (fl *FileLock) Lock() error {
	f, err := os.OpenFile(fl.path, os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("lock exists or creation failed: %w", err)
	}
	return f.Close() // Close immediately, existence is the lock
}
func (fl *FileLock) Unlock() error { return os.Remove(fl.path) }

func readInstanceMeta(instancePath string) (*InstanceMeta, error) {
	metaFilePath := filepath.Join(instancePath, metaFileName)
	data, err := os.ReadFile(metaFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read meta file %s: %w", metaFilePath, err)
	}
	var meta InstanceMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("failed to unmarshal meta file %s: %w", metaFilePath, err)
	}
	return &meta, nil
}

func writeInstanceMeta(instancePath string, meta *InstanceMeta) error {
	metaFilePath := filepath.Join(instancePath, metaFileName)
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal meta data: %w", err)
	}
	return os.WriteFile(metaFilePath, data, 0644)
}

func parseEnvValue(envContent []byte, key string) string {
	re := regexp.MustCompile(fmt.Sprintf(`(?m)^%s=(.*)$`, regexp.QuoteMeta(key)))
	match := re.FindSubmatch(envContent)
	if len(match) > 1 {
		return strings.TrimSpace(string(match[1]))
	}
	return ""
}

// Add this function
func getModulePath() (string, error) {
	// This assumes 'go' executable is in PATH and the command is run
	// from within a directory that is part of a Go module, or a subdirectory.
	// For 'wpid', it should be run from the project root ideally.
	cmd := exec.Command("go", "list", "-m")
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	// If wpid is run from a different directory than the project root,
	// you might need to set cmd.Dir to the project root.
	// Discovering the project root programmatically can be complex if not run from there.
	// For now, we assume it's run in a context where 'go list -m' works for the project.

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("failed to get module path with 'go list -m': %w. Stderr: %s", err, stderr.String())
	}
	return strings.TrimSpace(out.String()), nil
}

func isPortAvailable(port int) bool {
	address := fmt.Sprintf(":%d", port)
	listerner, err := net.Listen("tcp", address)
	if err != nil {
		return false
	}
	listerner.Close()
	return true
}

// findAvailablePort finds an available TCP port within a given range,
// also checking against ports already used by managed instances.
func findAvailablePort(startPort, endPort int, usedPorts map[int]bool) (int, error) {
	if startPort > endPort {
		return 0, errors.New("invalid port range")
	}
	maxAttempts := (endPort - startPort + 1) * 2 // Allow some retries

	for i := 0; i < maxAttempts; i++ {
		// Generate random port in range
		portBig, err := rand.Int(rand.Reader, big.NewInt(int64(endPort-startPort+1)))
		if err != nil {
			continue // Error generating random number, try again
		}
		port := int(portBig.Int64()) + startPort

		// Check if already used by our managed instances
		if _, used := usedPorts[port]; used {
			continue // Port known to be used by us, try again
		}

		// Check if actually available on the host
		if isPortAvailable(port) {
			return port, nil // Found an available port
		}
		// If not available on host, mark it as used for this search session
		// to avoid repeatedly checking a known busy port.
		usedPorts[port] = true
	}

	return 0, fmt.Errorf("could not find an available port between %d and %d", startPort, endPort)
}

// In cmd/wp-manager/main.go

func createInstance() {
	printSectionHeader("Create New WordPress Instance")

	// List templates with meta for selection
	templates, err := listAvailableTemplatesWithMeta()
	if err != nil || len(templates) == 0 {
		printError("No templates found", "Ensure at least one template with blueprint.json exists in templates/.")
		return
	}

	selectedTemplate := templates[0].Dir // default
	if len(templates) > 1 {
		var templateChoice string
		options := make([]huh.Option[string], 0, len(templates))
		for _, t := range templates {
			desc := t.Name
			if t.Description != "" {
				desc += " — " + t.Description
			}
			options = append(options, huh.NewOption(desc, t.Dir))
		}
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Select a template").
					Description("Choose a template for your new instance").
					Options(options...).
					Value(&templateChoice),
			),
		).WithTheme(theme)
		form.Run()
		if templateChoice != "" {
			selectedTemplate = templateChoice
		}
	}

	printInfo("Using template:", selectedTemplate)

	if _, err := exec.LookPath("docker"); err != nil {
		printError("Docker Not Found", "Docker is required but not installed or not in PATH.")
		os.Exit(1)
	}
	managerMeta, err := readManagerMeta()
	if err != nil {
		printWarning("Could not read instance metadata (for port conflict check).", err.Error())
		managerMeta = make(ManagerMeta)
	}
	usedPorts := make(map[int]bool)
	for _, meta := range managerMeta {
		if meta.WordPressPort > 0 {
			usedPorts[meta.WordPressPort] = true
		}
	}

	var finalInstanceParentDir string
	globalConfig, errConfig := readGlobalManagerConfig()
	if errConfig != nil {
		printWarning("Could not read global config; will prompt for instance location.", errConfig.Error())
	}

	if globalConfig.SitesBaseDirectory != "" {
		absGlobalPath, errAbs := filepath.Abs(globalConfig.SitesBaseDirectory)
		if errAbs != nil {
			printError("Invalid configured sites base directory path.", fmt.Sprintf("Could not make '%s' absolute: %v", globalConfig.SitesBaseDirectory, errAbs))
			printInfo("Please fix or clear it using 'wpid config set sites_base_directory ...'. Aborting.")
			return
		}
		targetBaseDirFromConfig := absGlobalPath
		printInfo("Using configured global sites base directory:", commandStyle.Render(targetBaseDirFromConfig))

		info, errStat := os.Stat(targetBaseDirFromConfig)
		if os.IsNotExist(errStat) {
			printWarning(fmt.Sprintf("Configured sites base directory '%s' does not exist yet.", targetBaseDirFromConfig))
			var confirmUseAndCreatePath bool
			form := huh.NewForm(
				huh.NewGroup(
					huh.NewConfirm().
						Title("Confirm Base Directory").
						Description(fmt.Sprintf("The configured sites base directory '%s' does not exist.\nDo you want to use this path? It will be created when the instance is made.", targetBaseDirFromConfig)).
						Affirmative("Yes, use this path (will be created)").
						Negative("No, let me choose a different location for this instance").
						Value(&confirmUseAndCreatePath),
				),
			).WithTheme(theme) // Added theme
			if errRun := form.Run(); errRun != nil {
				printError("Input cancelled.", errRun.Error())
				return
			}
			if confirmUseAndCreatePath {
				finalInstanceParentDir = targetBaseDirFromConfig
			} else {
				printInfo("Opted to choose a different location for this instance.")
			}
		} else if errStat != nil {
			printError(fmt.Sprintf("Error accessing configured sites base directory '%s': %v", targetBaseDirFromConfig, errStat))
			printInfo("Please fix or clear it. Aborting.")
			return
		} else if !info.IsDir() {
			printError(fmt.Sprintf("Configured sites base directory '%s' exists but is not a directory.", targetBaseDirFromConfig))
			printInfo("Please fix or clear it. Aborting.")
			return
		} else {
			finalInstanceParentDir = targetBaseDirFromConfig
		}
	}

	if finalInstanceParentDir == "" {
		if globalConfig.SitesBaseDirectory == "" {
			printInfo("No global sites base directory is configured.")
		}
		var createInCurrentDir bool
		formLocation := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("Instance Location").
					Description("Where would you like to create the new WordPress instance?").
					Affirmative("In the current directory (.) as the parent").
					Negative("Specify a different parent directory").
					Value(&createInCurrentDir),
			),
		).WithTheme(theme) // Added theme
		if errRun := formLocation.Run(); errRun != nil {
			printError("Input cancelled.", errRun.Error())
			return
		}

		if createInCurrentDir {
			cwd, errCwd := os.Getwd()
			if errCwd != nil {
				printError("Cannot get current working directory.", errCwd.Error())
				return
			}
			finalInstanceParentDir = cwd
		} else {
			var customParentDir string
			inputPathForm := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Parent Directory for New Instance").
						Description("Enter the path to the directory where the new instance's folder should be created.\n(e.g., /path/to/projects, or ~/my-wp-sites).").
						Value(&customParentDir).
						Validate(func(s string) error {
							if s == "" {
								return errors.New("path cannot be empty")
							}
							expandedPath := s
							if strings.HasPrefix(s, "~"+string(os.PathSeparator)) || s == "~" { // Handle ~/path and just ~
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
							absPath, errAbsVal := filepath.Abs(expandedPath)
							if errAbsVal != nil {
								return fmt.Errorf("could not determine absolute path: %v", errAbsVal)
							}
							info, errStatVal := os.Stat(absPath)
							if os.IsNotExist(errStatVal) {
								parent := filepath.Dir(absPath)
								if _, errP := os.Stat(parent); os.IsNotExist(errP) {
									return fmt.Errorf("parent directory '%s' for '%s' does not exist", parent, absPath)
								} else if errP != nil {
									return fmt.Errorf("error checking parent dir '%s': %v", parent, errP)
								}
							} else if errStatVal != nil {
								return fmt.Errorf("error accessing path '%s': %v", absPath, errStatVal)
							} else if !info.IsDir() {
								return fmt.Errorf("path '%s' exists but is not a directory", absPath)
							}
							return nil
						}),
				),
			).WithTheme(theme) // Added theme
			if errRun := inputPathForm.Run(); errRun != nil {
				printError("Input cancelled.", errRun.Error())
				return
			}

			resolvedCustomParentDir := customParentDir
			if strings.HasPrefix(customParentDir, "~"+string(os.PathSeparator)) || customParentDir == "~" {
				home, _ := os.UserHomeDir()
				if home != "" {
					if customParentDir == "~" {
						resolvedCustomParentDir = home
					} else {
						resolvedCustomParentDir = filepath.Join(home, customParentDir[2:])
					}
				}
			}
			absPath, errAbsVal := filepath.Abs(resolvedCustomParentDir)
			if errAbsVal != nil {
				printError("Invalid path.", fmt.Sprintf("Could not determine absolute path for '%s': %v", customParentDir, errAbsVal))
				return
			}
			finalInstanceParentDir = absPath
		}
	}
	printInfo("New WordPress instances will be created inside:", commandStyle.Render(finalInstanceParentDir))

	var instanceNameBase string
	var fullInstanceName string
	formInstanceName := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Instance Name").
				Description("Short name (e.g., 'myblog'). Prefixed 'www-' & suffixed '-wordpress'.").
				Placeholder(func() string {
					name, err := generateRandomName()
					if err != nil {
						return "default-name"
					}
					return name
				}()).
				Value(&instanceNameBase).
				Validate(func(s string) error {
					if s == "" {
						randomName, err := generateRandomName()
						if err != nil {
							return errors.New("failed to generate random name")
						}
						instanceNameBase = randomName
						return nil
					}
					if strings.ContainsAny(s, "/\\:*?\"<>| ") {
						return errors.New("name has invalid chars/spaces")
					}
					tempFullInstanceName := filepath.Join(finalInstanceParentDir, "www-"+s+"-wordpress")
					if _, errFS := os.Stat(tempFullInstanceName); errFS == nil {
						return fmt.Errorf("dir exists: %s", tempFullInstanceName)
					} else if !os.IsNotExist(errFS) {
						return fmt.Errorf("err checking dir %s: %v", tempFullInstanceName, errFS)
					}
					return nil
				}),
		),
	).WithTheme(theme)
	if errRun := formInstanceName.Run(); errRun != nil {
		printError("Input cancelled.", errRun.Error())
		return
	}
	fullInstanceName = filepath.Join(finalInstanceParentDir, "www-"+instanceNameBase+"-wordpress")

	var customizeSettings bool
	formCustomize := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Customize Settings?").
				Description("Customize DB credentials, ports, URLs? Unique ports auto-assigned for defaults.").
				Affirmative("Yes, customize").
				Negative("No, use defaults").
				Value(&customizeSettings),
		),
	).WithTheme(theme) // Added theme
	if errRun := formCustomize.Run(); errRun != nil {
		printError("Input cancelled.", errRun.Error())
		return
	}

	var wordpressPort = 0
	var productionURL string
	wpUser := instanceNameBase + "_user"
	wpPassword, _ := generateRandomString(16)
	wpDBName := instanceNameBase + "_db"
	mysqlRootPassword, _ := generateRandomString(16)

	var mailpitSMTPPort, mailpitWebPort, adminerWebPort int
	var errPort error
	mailpitSMTPPort, errPort = findAvailablePort(10000, 10999, usedPorts)
	if errPort != nil {
		printError("Failed to find port for Mailpit SMTP.", errPort.Error())
		os.RemoveAll(fullInstanceName)
		return
	}

	mailpitWebPort, errPort = findAvailablePort(8000, 8999, usedPorts)
	if errPort != nil {
		printError("Failed to find port for Mailpit Web.", errPort.Error())
		os.RemoveAll(fullInstanceName)
		return
	}
	adminerWebPort, errPort = findAvailablePort(8081, 8999, usedPorts)
	if errPort != nil {
		printError("Failed to find port for Adminer Web.", errPort.Error())
		os.RemoveAll(fullInstanceName)
		return

	}

	salts := make(map[string]string)
	saltKeys := []string{"AUTH_KEY", "SECURE_AUTH_KEY", "LOGGED_IN_KEY", "NONCE_KEY", "AUTH_SALT", "SECURE_AUTH_SALT", "LOGGED_IN_SALT", "NONCE_SALT"}
	for _, key := range saltKeys {
		salts[key], _ = generateRandomString(64)
	}

	// --- Caddy Port Assignment and Prompting Logic ---
	caddyHTTPPort := 80
	caddyHTTPSPort := 443
	caddyEnabled := false
	if !isPortAvailable(80) || !isPortAvailable(443) {
		printWarning("Ports 80 and/or 443 are already in use.", "A host-level web server (Caddy, Nginx, Apache, etc.) may be running.")
		printInfo("Caddy container will NOT be enabled by default.", "You can use the provided Caddyfile template for your host server.")
	} else {
		var enableCaddy bool
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("Enable Caddy container?").
					Description("No web server detected on ports 80/443. Would you like to use the built-in Caddy container for this instance?").
					Affirmative("Yes, use Caddy container").
					Negative("No, I'll use my own web server").
					Value(&enableCaddy),
			),
		).WithTheme(theme)
		_ = form.Run()
		caddyEnabled = enableCaddy
		if caddyEnabled {
			printSuccess("Caddy container will be enabled for this instance.")
		} else {
			printInfo("Caddy container will NOT be enabled. Use the Caddyfile template for your host server.")
		}
	}

	// Allow user to pick custom Caddy ports if enabled
	if caddyEnabled {
		var customPorts bool
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("Customize Caddy ports?").
					Description("Would you like to use ports other than 80/443 for the Caddy container?").
					Affirmative("Yes, pick ports").
					Negative("No, use 80/443").
					Value(&customPorts),
			),
		).WithTheme(theme)
		_ = form.Run()
		if customPorts {
			var httpPortStr, httpsPortStr string
			form := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().Title("Caddy HTTP Port").Description("Enter the HTTP port for Caddy (default 80)").Value(&httpPortStr).Placeholder("80").Validate(func(s string) error {
						if s == "" {
							return nil
						}
						p, err := strconv.Atoi(s)
						if err != nil || p < 1 || p > 65535 {
							return errors.New("invalid port")
						}
						return nil
					}),
					huh.NewInput().Title("Caddy HTTPS Port").Description("Enter the HTTPS port for Caddy (default 443)").Value(&httpsPortStr).Placeholder("443").Validate(func(s string) error {
						if s == "" {
							return nil
						}
						p, err := strconv.Atoi(s)
						if err != nil || p < 1 || p > 65535 {
							return errors.New("invalid port")
						}
						return nil
					}),
				),
			).WithTheme(theme)
			_ = form.Run()
			if httpPortStr != "" {
				caddyHTTPPort, _ = strconv.Atoi(httpPortStr)
			}
			if httpsPortStr != "" {
				caddyHTTPSPort, _ = strconv.Atoi(httpsPortStr)
			}
		}
	}

	WORDPRESS_VERSION := "latest"
	if customizeSettings {
		var wpVersionInput string
		wpVersionPrompt := huh.NewInput().
			Title("WordPress Version").
			Description("Enter the WordPress version to use (default: latest)").
			Placeholder("latest").
			Value(&wpVersionInput)
		if err := wpVersionPrompt.WithTheme(theme).Run(); err == nil && strings.TrimSpace(wpVersionInput) != "" {
			WORDPRESS_VERSION = strings.TrimSpace(wpVersionInput)
		}

		printInfo("Custom Configuration Required")
		suggestedWPPort, errPortFind := findAvailablePort(11000, 19999, usedPorts)
		if errPortFind != nil {
			printWarning("Could not find suggested WP port.", "Defaulting prompt.", errPortFind.Error())
			suggestedWPPort = defaultWordPressPort
		}

		wordpressPortStr := strconv.Itoa(suggestedWPPort)
		mailpitSMTPPortStr := strconv.Itoa(mailpitSMTPPort)
		mailpitWebPortStr := strconv.Itoa(mailpitWebPort)
		adminerWebPortStr := strconv.Itoa(adminerWebPort)
		productionURL = fmt.Sprintf("http://0.0.0.0:%d", suggestedWPPort)

		groupFields := []*huh.Input{
			huh.NewInput().Title("WordPress Port").Description("Enter port (checked for availability).").Value(&wordpressPortStr).Validate(func(s string) error {
				p, errV := strconv.Atoi(s)
				if errV != nil {
					return errors.New("invalid port")
				}
				if p <= 0 || p > 65535 {
					return errors.New("port out of range")
				}
				// Allow keeping the current suggested port if it's still available
				if p == suggestedWPPort && isPortAvailable(p) {
					return nil
				}
				// If different from suggested, check if it's in usedPorts (by other instances) or generally unavailable
				if used, exists := usedPorts[p]; exists && used {
					return fmt.Errorf("port %d may be used by another managed instance", p)
				}
				if !isPortAvailable(p) {
					return fmt.Errorf("port %d not available on the host", p)
				}
				return nil
			}),
			huh.NewInput().Title("Production URL (WP_SITEURL & WP_HOME)").Description("e.g., http://myblog.com or http://0.0.0.0:PORT").Value(&productionURL),
			huh.NewInput().Title("WordPress DB User").Value(&wpUser),
			huh.NewInput().Title("WordPress DB Password").Value(&wpPassword).EchoMode(huh.EchoModePassword),
			huh.NewInput().Title("WordPress DB Name").Value(&wpDBName),
			huh.NewInput().Title("MySQL Root Password").Value(&mysqlRootPassword).EchoMode(huh.EchoModePassword),
			huh.NewInput().Title("Mailpit SMTP Port").Value(&mailpitSMTPPortStr).Validate(func(s string) error {
				p, e := strconv.Atoi(s)
				if e != nil || p <= 0 {
					return errors.New("invalid port")
				}
				return nil
			}),
			huh.NewInput().Title("Mailpit Web UI Port").Value(&mailpitWebPortStr).Validate(func(s string) error {
				p, e := strconv.Atoi(s)
				if e != nil || p <= 0 {
					return errors.New("invalid port")
				}
				return nil
			}),
			huh.NewInput().Title("Adminer Web UI Port").Value(&adminerWebPortStr).Validate(func(s string) error {
				p, e := strconv.Atoi(s)
				if e != nil || p <= 0 {
					return errors.New("invalid port")
				}
				return nil
			}),
		}
		formConfig := huh.NewForm(huh.NewGroup(func() []huh.Field {
			fields := make([]huh.Field, len(groupFields))
			for i, field := range groupFields {
				fields[i] = field
			}
			return fields
		}()...)).WithTheme(theme)
		if errRun := formConfig.Run(); errRun != nil {
			printError("Config input error.", errRun.Error())
			os.RemoveAll(fullInstanceName)
			return
		}

		var parseErr error
		wordpressPort, parseErr = strconv.Atoi(wordpressPortStr)
		if parseErr != nil {
			printError("Invalid WP port.", parseErr.Error())
			os.RemoveAll(fullInstanceName)
			return
		}
		mailpitSMTPPort, parseErr = strconv.Atoi(mailpitSMTPPortStr)
		if parseErr != nil {
			printError("Invalid Mailpit SMTP.", parseErr.Error())
			os.RemoveAll(fullInstanceName)
			return
		}
		mailpitWebPort, parseErr = strconv.Atoi(mailpitWebPortStr)
		if parseErr != nil {
			printError("Invalid Mailpit Web.", parseErr.Error())
			os.RemoveAll(fullInstanceName)
			return
		}
		adminerWebPort, parseErr = strconv.Atoi(adminerWebPortStr)
		if parseErr != nil {
			printError("Invalid Adminer Web.", parseErr.Error())
			os.RemoveAll(fullInstanceName)
			return
		}

		// Final check for WordPress port if customized
		if !isPortAvailable(wordpressPort) {
			// Check if it was previously in usedPorts (from another instance)
			if used, exists := usedPorts[wordpressPort]; exists && used {
				// If it's used by another instance, this is a definite conflict.
				// If it was the suggestedPort, then something else took it between suggestion and now.
				printError("Port Conflict Post-Input", fmt.Sprintf("Selected WordPress port %d is in use or unavailable.", wordpressPort))
			} else {
				printError("Port Conflict Post-Input", fmt.Sprintf("Selected WordPress port %d became unavailable.", wordpressPort))
			}
			os.RemoveAll(fullInstanceName)
			return
		}
	} else {
		printInfo("Using Generated Defaults")
		port, errPortFind := findAvailablePort(11000, 19999, usedPorts)
		if errPortFind != nil {
			printError("Failed to Find Available WordPress Port", errPortFind.Error())
			os.RemoveAll(fullInstanceName)
			return
		}
		wordpressPort = port
		productionURL = fmt.Sprintf("http://0.0.0.0:%d", wordpressPort)
		printInfo("Assigned WordPress Port:", fmt.Sprintf("%d", wordpressPort))
		printInfo("Assigned Mailpit SMTP Port:", fmt.Sprintf("%d", mailpitSMTPPort))
		printInfo("Assigned Mailpit Web Port:", fmt.Sprintf("%d", mailpitWebPort))
		printInfo("Assigned Adminer Web Port:", fmt.Sprintf("%d", adminerWebPort))
	}
	if wordpressPort == 0 {
		printError("Internal Error", "WordPress port not assigned.")
		os.RemoveAll(fullInstanceName)
		return
	}

	printInfo("Setting up instance directory structure...", fmt.Sprintf("Target: %s", commandStyle.Render(fullInstanceName)))
	if err := os.MkdirAll(fullInstanceName, 0755); err != nil {
		printError("Directory Creation Failed", fmt.Sprintf("Failed to create %s: %v", fullInstanceName, err))
		return
	}

	manageBinaryNameInProject := "manage"
	if runtime.GOOS == "windows" {
		manageBinaryNameInProject = "manage.exe"
	}

	// Use the selectedTemplate variable for template file copying
	templateRoot := filepath.Join("cmd/wp-manager/templates", selectedTemplate)
	errCopy := fs.WalkDir(os.DirFS(templateRoot), ".", func(pathInTemplate string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk err at %s: %w", pathInTemplate, err)
		}
		relativePath := pathInTemplate
		if relativePath == "." {
			return nil
		}
		targetPath := filepath.Join(fullInstanceName, relativePath)
		if d.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}
		srcFile, errRead := os.ReadFile(filepath.Join(templateRoot, pathInTemplate))
		if errRead != nil {
			return fmt.Errorf("read template %s: %w", pathInTemplate, errRead)
		}
		perm := fs.FileMode(0644)
		if d.Name() == "manage" || d.Name() == "manage.exe" {
			perm = 0755
		}
		return os.WriteFile(targetPath, srcFile, perm)
	})
	if errCopy != nil {
		printError("Template Copy Failed", errCopy.Error())
		os.RemoveAll(fullInstanceName)
		return
	}
	printSuccess("Template Files Copied", fmt.Sprintf("Copied files (incl. '%s') to %s", manageBinaryNameInProject, fullInstanceName))

	envFilePath := filepath.Join(fullInstanceName, ".env")
	envTemplatePathInInstance := filepath.Join(fullInstanceName, envTemplateFileName)
	if _, err := os.Stat(envTemplatePathInInstance); os.IsNotExist(err) {
		printError(fmt.Sprintf("'%s' Missing", envTemplateFileName), fmt.Sprintf("%s not found after copy.", envTemplatePathInInstance))
		os.RemoveAll(fullInstanceName)
		return
	}
	if err := os.Rename(envTemplatePathInInstance, envFilePath); err != nil {
		printError("Failed to Prepare .env", fmt.Sprintf("Rename %s failed: %v", envTemplatePathInInstance, err))
		os.RemoveAll(fullInstanceName)
		return
	}
	envContent, err := os.ReadFile(envFilePath)
	if err != nil {
		printError("Failed to Read .env", fmt.Sprintf("Read %s failed: %v", envFilePath, err))
		os.RemoveAll(fullInstanceName)
		return
	}

	replacements := map[string]string{
		"WORDPRESS_CONTAINER_NAME": "wp-" + instanceNameBase,
		"WORDPRESS_PORT":           strconv.Itoa(wordpressPort),
		"WORDPRESS_URL":            fmt.Sprintf("http://0.0.0.0:%d", wordpressPort),
		"PRODUCTION_URL":           productionURL,
		"WORDPRESS_DB_USER":        wpUser,
		"WORDPRESS_DB_PASSWORD":    wpPassword,
		"WORDPRESS_DB_NAME":        wpDBName,
		"WORDPRESS_DB_HOST":        "db",
		"MYSQL_USER":               wpUser,
		"MYSQL_PASSWORD":           wpPassword,
		"MYSQL_DATABASE":           wpDBName,
		"MYSQL_ROOT_PASSWORD":      mysqlRootPassword,
		"MAILPIT_PORT_SMTP":        strconv.Itoa(mailpitSMTPPort),
		"MAILPIT_PORT_WEB":         strconv.Itoa(mailpitWebPort),
		"ADMINER_PORT":             strconv.Itoa(adminerWebPort),
		"WORDPRESS_AUTH_KEY":       salts["AUTH_KEY"], "WORDPRESS_SECURE_AUTH_KEY": salts["SECURE_AUTH_KEY"], "WORDPRESS_LOGGED_IN_KEY": salts["LOGGED_IN_KEY"], "WORDPRESS_NONCE_KEY": salts["NONCE_KEY"], "WORDPRESS_AUTH_SALT": salts["AUTH_SALT"], "WORDPRESS_SECURE_AUTH_SALT": salts["SECURE_AUTH_SALT"], "WORDPRESS_LOGGED_IN_SALT": salts["LOGGED_IN_SALT"], "WORDPRESS_NONCE_SALT": salts["NONCE_SALT"],
		"CADDY_HTTP_PORT":   strconv.Itoa(caddyHTTPPort),
		"CADDY_HTTPS_PORT":  strconv.Itoa(caddyHTTPSPort),
		"WORDPRESS_VERSION": WORDPRESS_VERSION,
	}
	newEnvContentStr := string(envContent)
	for key, value := range replacements {
		re := regexp.MustCompile(fmt.Sprintf(`(?m)^%s=.*$`, regexp.QuoteMeta(key)))
		placeholderRe := regexp.MustCompile(fmt.Sprintf(`(?m)^%s=$`, regexp.QuoteMeta(key)))
		if re.MatchString(newEnvContentStr) {
			newEnvContentStr = re.ReplaceAllString(newEnvContentStr, fmt.Sprintf("%s=%s", key, value))
		} else if placeholderRe.MatchString(newEnvContentStr) {
			newEnvContentStr = placeholderRe.ReplaceAllString(newEnvContentStr, fmt.Sprintf("%s=%s", key, value))
		} else {
			if !strings.Contains(newEnvContentStr, "\n"+key+"=") && !strings.HasPrefix(newEnvContentStr, key+"=") {
				printWarning(fmt.Sprintf(".env Key Missing/Malformed: %s", key), "Template keys should be 'KEY=' or 'KEY=default'.")
			}
		}
	}
	if err := os.WriteFile(envFilePath, []byte(newEnvContentStr), 0644); err != nil {
		printError("Failed to Write .env", fmt.Sprintf("Update .env failed: %v", err))
		os.RemoveAll(fullInstanceName)
		return
	}
	printSuccess(".env File Configured")

	// --- Generate Instance-Specific Caddyfile ---
	printInfo("Generating instance-specific Caddyfile...")

	// Prompt for dev domain suffix (default: .example.local)
	devDomainSuffix := ".example.local"
	var customDomainSuffix string
	formDomain := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Dev Domain Suffix").
				Description("Enter the dev domain suffix for this instance (e.g., .local, .test, .example.local).").
				Placeholder(devDomainSuffix).
				Value(&customDomainSuffix),
		),
	).WithTheme(theme)
	formDomain.Run()
	if strings.TrimSpace(customDomainSuffix) != "" {
		if !strings.HasPrefix(customDomainSuffix, ".") {
			customDomainSuffix = "." + customDomainSuffix
		}
		devDomainSuffix = customDomainSuffix
	}
	devHostName := instanceNameBase + devDomainSuffix

	caddyData := InstanceCaddyConfigData{
		InstanceName:     filepath.Base(fullInstanceName),
		DevHostName:      devHostName,
		WordPressPort:    wordpressPort,    // The host port
		CaddyHTTPPort:    caddyHTTPPort,    // For Caddyfile.template compatibility
		InstanceNameBase: instanceNameBase, // for Caddyfile.template compatibility
		DevDomainSuffix:  devDomainSuffix,  // for Caddyfile.template compatibility
	}
	caddyTemplatePathInEmbedFS := "templates/docker-default-wordpress/config/Caddyfile.template"

	templateContent, err := defaultWordpressTemplate.ReadFile(caddyTemplatePathInEmbedFS)
	if err != nil {
		printWarning("Could not read embedded Caddyfile.template.", fmt.Sprintf("Path: %s, Error: %v", caddyTemplatePathInEmbedFS, err))
		// Decide if this is fatal or if the instance can be created without it.
		// For now, let's continue but warn.
	} else {
		tmpl, err := template.New("instanceCaddyfile").Parse(string(templateContent))
		if err != nil {
			printWarning("Failed to parse embedded Caddyfile.template.", err.Error())
		} else {
			// Ensure the config directory exists in the new instance
			instanceConfigDir := filepath.Join(fullInstanceName, "config")
			if err := os.MkdirAll(instanceConfigDir, 0755); err != nil {
				printWarning("Could not create config directory in instance.", instanceConfigDir, err.Error())
			} else {
				caddyFileOutputPath := filepath.Join(instanceConfigDir, "Caddyfile") // Output as "Caddyfile"
				file, err := os.Create(caddyFileOutputPath)
				if err != nil {
					printWarning("Failed to create Caddyfile in instance.", caddyFileOutputPath, err.Error())
				} else {
					defer file.Close()
					if err := tmpl.Execute(file, caddyData); err != nil {
						printWarning("Failed to execute Caddyfile template for instance.", err.Error())
					} else {
						printSuccess("Instance-specific Caddyfile generated:", caddyFileOutputPath)
						printInfo("  Use this if running Caddy within this instance's Docker Compose setup,")
						printInfo(fmt.Sprintf("  or configure your host Caddy to reverse_proxy to %s on port %d.", devHostName, wordpressPort))
					}
				}
			}
		}
	}

	localMeta := InstanceMeta{
		Directory:        fullInstanceName,
		CreationDate:     time.Now().Format("2006-01-02 15:04:05"),
		WordPressVersion: parseEnvValue([]byte(newEnvContentStr), "WORDPRESS_VERSION"),
		DBVersion:        parseEnvValue([]byte(newEnvContentStr), "MYSQL_VERSION"),
		WordPressPort:    wordpressPort, Status: "Stopped",
	}
	if err := writeInstanceMeta(fullInstanceName, &localMeta); err != nil {
		printError("Local Meta Write Failed", fmt.Sprintf("Write %s failed: %v", metaFileName, err))
		os.RemoveAll(fullInstanceName)
		return
	}
	printSuccess(fmt.Sprintf("Local instance metadata file (%s) created.", metaFileName))

	currentManagerMeta, errReadMeta := readManagerMeta()
	if errReadMeta != nil {
		printError("Failed to Read Manager Meta Before Final Write", errReadMeta.Error())
		printWarning("Instance created but not registered centrally.", "Check "+managerMetaFileName)
	} else {
		instanceKey := filepath.Base(fullInstanceName)
		currentManagerMeta[instanceKey] = localMeta
		if errWriteMgr := writeManagerMeta(currentManagerMeta); errWriteMgr != nil {
			printError("Failed to Write Central Manager Meta", errWriteMgr.Error())
			printWarning("Instance created but central registration failed.", "Check "+managerMetaFileName)
			localMeta.Status = "Stopped"
			_ = writeInstanceMeta(fullInstanceName, &localMeta)
		} else {
			printSuccess("Instance registered with central manager.")
		}
	}

	successDetails := []string{
		fmt.Sprintf("Name: %s", commandStyle.Render(instanceNameBase)),
		fmt.Sprintf("Directory: %s", commandStyle.Render(fullInstanceName)),
		fmt.Sprintf("WordPress Port (on host): %d", wordpressPort),
		fmt.Sprintf("Suggested Dev Hostname: %s (Add to hosts file: 127.0.0.1 %s)", commandStyle.Render(devHostName), devHostName),
		fmt.Sprintf("Mailpit Web UI: http://0.0.0.0:%d (SMTP on port %d)", mailpitWebPort, mailpitSMTPPort),
		fmt.Sprintf("Adminer Web UI: http://0.0.0.0:%d", adminerWebPort),
		"",
		lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Render("Next steps:"),
		fmt.Sprintf("  cd %s", commandStyle.Render(fullInstanceName)),
		fmt.Sprintf("  Run: %s", commandStyle.Render(fmt.Sprintf("./%s start", manageBinaryNameInProject))),
		fmt.Sprintf("  Access via browser: %s (if hosts/Caddy configured) or http://0.0.0.0:%d", commandStyle.Render(devHostName), wordpressPort),
	}
	printSuccess("🎉 Instance Created Successfully!", successDetails...)

	promptAndStartInstance(fullInstanceName)

	// --- Copy and Patch docker-compose.yml ---
	dockerComposeInstancePath := filepath.Join(fullInstanceName, "docker-compose.yml")

	dockerComposeContent, err := os.ReadFile(dockerComposeInstancePath)
	if err != nil {
		printError("Failed to read docker-compose.yml after copy", err.Error())
		os.RemoveAll(fullInstanceName)
		return
	}
	if !caddyEnabled {
		// Remove or comment out the Caddy service block
		lines := strings.Split(string(dockerComposeContent), "\n")
		var newLines []string
		inCaddy := false
		for i := 0; i < len(lines); i++ {
			line := lines[i]
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "caddy:") && strings.HasPrefix(line, "  caddy:") {
				inCaddy = true
				// Optionally, add a comment to indicate Caddy is disabled
				newLines = append(newLines, "  # caddy: (disabled by WPID)")
				continue
			}
			if inCaddy {
				// End of service block is when we hit a non-indented line or another top-level service
				if len(line) > 0 && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
					inCaddy = false
				}
			}
			if !inCaddy {
				newLines = append(newLines, line)
			}
		}
		// Remove caddy volumes if present
		var finalLines []string
		inCaddyVolume := false
		for i := 0; i < len(newLines); i++ {
			line := newLines[i]
			if strings.HasPrefix(strings.TrimSpace(line), "caddy_data:") || strings.HasPrefix(strings.TrimSpace(line), "caddy_config:") {
				inCaddyVolume = true
				continue // skip these lines
			}
			if inCaddyVolume {
				if len(line) > 0 && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
					inCaddyVolume = false
				}
				if inCaddyVolume {
					continue
				}
			}
			finalLines = append(finalLines, line)
		}
		if err := os.WriteFile(dockerComposeInstancePath, []byte(strings.Join(finalLines, "\n")), 0644); err != nil {
			printError("Failed to patch docker-compose.yml to disable Caddy", err.Error())
			os.RemoveAll(fullInstanceName)
			return
		}
		printInfo("Caddy service disabled in docker-compose.yml for this instance.")
	}
}

func deleteInstance() {
	printSectionHeader("Delete WordPress Instance")

	// Read manager meta to get list of instances
	managerMeta, err := readManagerMeta()
	if err != nil {
		printError("Failed to Read Manager Metadata", err.Error())
		return
	}
	if len(managerMeta) == 0 {
		printWarning("No Instances Found", "No instances registered to delete.")
		return
	}

	var instanceNames []string
	instanceMap := make(map[string]InstanceMeta) // Keep track of meta by name for easy lookup
	for name, meta := range managerMeta {
		instanceNames = append(instanceNames, name)
		instanceMap[name] = meta
	}
	sort.Strings(instanceNames)

	var instanceToDelete string
	options := make([]huh.Option[string], len(instanceNames))
	for i, inst := range instanceNames {
		// Corrected: The value is the second argument to NewOption
		options[i] = huh.NewOption(inst, inst)
	}

	selectInstance := huh.NewSelect[string]().
		Title("Select Instance to Delete").
		Description("Choose the WordPress instance you wish to permanently remove.").
		Options(options...). // Pass the correctly created slice of options
		Value(&instanceToDelete)

	if err := selectInstance.WithTheme(theme).Run(); err != nil {
		// User likely cancelled (e.g., Esc)
		printInfo("Selection Cancelled", fmt.Sprintf("Instance deletion cancelled: %v", err))
		return
	}
	// Double-check if a value was actually selected, though huh usually handles this.
	if instanceToDelete == "" {
		printWarning("No Selection Made", "No instance selected. Deletion cancelled.")
		return
	}

	// Get the metadata for the selected instance using the map
	selectedMeta, ok := instanceMap[instanceToDelete]
	if !ok {
		// This should theoretically not happen if the selection came from the map keys
		printError("Internal Error", fmt.Sprintf("Selected instance '%s' not found in metadata map.", instanceToDelete))
		return
	}
	instancePath := selectedMeta.Directory // Use the stored directory path

	// Confirmation prompt
	var confirmDelete bool
	confirmDeletePrompt := huh.NewConfirm().
		Title(fmt.Sprintf("Confirm Deletion: %s", instanceToDelete)).
		Description(lipgloss.JoinVertical(lipgloss.Left,
			fmt.Sprintf("Instance directory: %s", commandStyle.Render(instancePath)), // Show path
			"", // Add spacing
			errorMsgStyle.Render("This action is irreversible!"), // Use error style for warning
			"It will:",
			"  - Stop and remove associated Docker containers.",
			"  - Delete all data volumes for this instance.",
			"  - Remove the instance directory and all its contents.",
		)).
		Affirmative("Yes, delete this instance").
		Negative("No, keep it")

	// Run returns error on cancellation as well, treat cancellation as "No"
	_ = confirmDeletePrompt.Value(&confirmDelete).WithTheme(theme).Run()

	if !confirmDelete {
		printInfo("Deletion Cancelled", fmt.Sprintf("Instance '%s' was not deleted.", instanceToDelete))
		return
	}

	// Proceed with deletion
	printInfo(fmt.Sprintf("Proceeding with deletion of: %s", instanceToDelete))

	// Check if directory actually exists before trying docker/rm
	if _, statErr := os.Stat(instancePath); os.IsNotExist(statErr) {
		printWarning("Directory Not Found", fmt.Sprintf("Instance directory %s not found. Removing from manager list.", instancePath))
		// Remove from manager meta even if dir is gone
		delete(managerMeta, instanceToDelete)
		if writeErr := writeManagerMeta(managerMeta); writeErr != nil {
			printError("Failed to Update Manager Metadata", writeErr.Error())
		} else {
			printSuccess("Instance removed from manager list.")
		}
		return // Exit deletion process
	}

	// Stop Docker containers
	printInfo(fmt.Sprintf("Stopping Docker containers for %s...", instanceToDelete))
	cmd := exec.Command("docker", "compose", "down", "--volumes", "--remove-orphans")
	cmd.Dir = instancePath // IMPORTANT: Run docker compose in the instance directory
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		errMsg := stderr.String()
		if errMsg == "" {
			errMsg = err.Error()
		}
		printWarning(fmt.Sprintf("Docker Cleanup Issue for %s", instanceToDelete),
			"Could not stop/remove all Docker resources (they might already be stopped or gone).",
			fmt.Sprintf("Details: %s", strings.TrimSpace(errMsg)))
	} else {
		printSuccess(fmt.Sprintf("Docker Containers Removed for %s", instanceToDelete))
	}

	// Delete directory
	printInfo(fmt.Sprintf("Deleting instance directory: %s...", instancePath))
	if err := os.RemoveAll(instancePath); err != nil {
		if os.IsPermission(err) {
			printWarning("Permission Denied", fmt.Sprintf("Failed to delete directory %s due to insufficient permissions.", instancePath))
			var confirmElevate bool
			confirmPrompt := huh.NewConfirm().
				Title("Elevated Permission Required").
				Description(fmt.Sprintf("Do you want to attempt deletion of '%s' with elevated permissions?", instancePath)).
				Affirmative("Yes, try with elevated permissions").
				Negative("No, skip deletion")
			_ = confirmPrompt.Value(&confirmElevate).WithTheme(theme).Run()

			if confirmElevate {
				var cmd *exec.Cmd
				if runtime.GOOS == "windows" {
					cmd = exec.Command("powershell", "-Command", fmt.Sprintf("Remove-Item -Recurse -Force '%s'", instancePath))
				} else {
					cmd = exec.Command("sudo", "rm", "-rf", instancePath)
				}
				cmd.Stdin = os.Stdin
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				if err := cmd.Run(); err != nil {
					printError(fmt.Sprintf("Failed to Delete Directory %s with Elevated Permissions", instancePath), fmt.Sprintf("%v", err))
					printWarning("Directory deletion failed, but attempting to remove from manager list.")
				} else {
					printSuccess("Instance Directory Deleted with Elevated Permissions")
				}
			} else {
				printWarning("Directory deletion skipped due to insufficient permissions.")
			}
		} else {
			printError(fmt.Sprintf("Failed to Delete Directory %s", instancePath), fmt.Sprintf("%v", err))
			// Even if dir deletion fails, try removing from manager list but warn user
			printWarning("Directory deletion failed, but attempting to remove from manager list.")
		}
	} else {
		printSuccess("Instance Directory Deleted")
	}

	// Remove from manager meta and write back
	delete(managerMeta, instanceToDelete)
	if err := writeManagerMeta(managerMeta); err != nil {
		printError("Failed to Update Manager Metadata", err.Error())
	} else {
		printSuccess("Instance Removed", fmt.Sprintf("Successfully removed instance '%s' from manager list.", instanceToDelete))
	}
} // End of deleteInstance

func updateStatuses() {
	printSectionHeader("Update Instance Statuses")

	// Read manager meta
	managerMeta, err := readManagerMeta()
	if err != nil {
		printError("Failed to Read Manager Metadata", err.Error())
		return
	}
	if len(managerMeta) == 0 {
		printWarning("No Instances Found", "No instances registered to update.")
		return
	}

	updatedCount := 0
	somethingChanged := false
	printInfo("Checking status for each registered instance...")

	// Iterate through manager meta
	for instanceName, meta := range managerMeta {
		instancePath := meta.Directory // Get path from manager meta
		originalStatus := meta.Status
		newStatus := "Unknown"

		if _, statErr := os.Stat(instancePath); os.IsNotExist(statErr) {
			newStatus = "Directory Missing"
		} else {
			// Run docker compose ps in the specific instance directory
			cmd := exec.Command("docker", "compose", "ps", "--services", "--filter", "status=running")
			cmd.Dir = instancePath
			var out bytes.Buffer
			cmd.Stdout = &out
			// Ignore error, as non-zero exit often means no running services
			_ = cmd.Run()

			runningServices := out.String()
			if strings.Contains(runningServices, "wordpress") ||
				strings.Contains(runningServices, "wp") ||
				strings.Contains(runningServices, "web") ||
				strings.Contains(runningServices, "app") {
				newStatus = "Running"
			} else {
				newStatus = "Stopped"
			}
		}

		// Update status in the managerMeta map if changed
		if originalStatus != newStatus {
			fmt.Printf("  %s Status: %s -> %s\n",
				boldStyle.Render(instanceName),
				subtleStyle.Render(originalStatus),
				renderStatus(newStatus)) // Uses the existing helper

			meta.Status = newStatus          // Update the meta struct (which is a copy)
			managerMeta[instanceName] = meta // Put the updated copy back in the map
			somethingChanged = true
			updatedCount++

			// --- Optional: Update the local .wordpress-meta.json file ---
			// Check if dir exists before trying to write local meta
			if _, statErr := os.Stat(instancePath); statErr == nil {
				// Read local meta first to preserve other fields
				localMeta, readErr := readInstanceMeta(instancePath)
				if readErr != nil {
					printWarning(fmt.Sprintf("Local Meta Read Error for %s", instanceName), fmt.Sprintf("Could not update local status: %v", readErr))
				} else {
					localMeta.Status = newStatus // Only update the status field
					if writeErr := writeInstanceMeta(instancePath, localMeta); writeErr != nil {
						printWarning(fmt.Sprintf("Local Meta Write Error for %s", instanceName), fmt.Sprintf("Could not update local status: %v", writeErr))
					}
				}
			}
			// --- End Optional Local Update ---
		}
	} // End loop through instances

	// Write the manager meta back to file *once* if anything changed
	if somethingChanged {
		if err := writeManagerMeta(managerMeta); err != nil {
			printError("Failed to Write Updated Manager Metadata", err.Error())
		} else {
			printSuccess("Statuses Updated", fmt.Sprintf("%d instance(s) had their status refreshed in manager metadata.", updatedCount))
		}
	} else {
		printInfo("Statuses Up-to-Date", "All instance statuses are current.")
	}
}

func renderStatus(status string) string {
	switch status {
	case "Running":
		return statusRunningStyle.Render(status)
	case "Stopped":
		return statusStoppedStyle.Render(status)
	case "Directory Missing", "Unknown":
		return statusErrorStyle.Render(status)
	default:
		return status
	}
}

func listInstances() {
	printSectionHeader("List WordPress Instances")

	// Read central manager metadata
	managerMeta, err := readManagerMeta()
	if err != nil {
		printError("Failed to Read Manager Metadata", err.Error())
		return
	}

	if len(managerMeta) == 0 {
		printWarning("No Instances Found", "No instances registered with the manager.")
		printInfo("Tip:", "Use '"+commandStyle.Render("create")+"' to add a new instance.")
		return
	}

	// Define column widths (adjust as needed)
	nameWidth := 30
	portWidth := 8
	dateWidth := 20
	wpVerWidth := 15 // Keep this
	dbVerWidth := 15 // Keep this
	statusWidth := 18
	dirWidth := 50

	// Header row - UNCOMMENT WP Ver and DB Ver headers
	header := lipgloss.JoinHorizontal(lipgloss.Top,
		tableHeaderStyle.Width(nameWidth).Render("Instance Name"),
		tableHeaderStyle.Width(portWidth).Render("Port"),
		tableHeaderStyle.Width(dateWidth).Render("Created"),
		tableHeaderStyle.Width(wpVerWidth).Render("WP Ver"), // Uncommented
		tableHeaderStyle.Width(dbVerWidth).Render("DB Ver"), // Uncommented
		tableHeaderStyle.Width(dirWidth).Render("Directory"),
		tableHeaderStyle.Width(statusWidth).Render("Status"),
	)

	var rows []string
	rows = append(rows, header)

	// Iterate through the manager metadata map
	for instanceName, meta := range managerMeta {
		var rowCells []string

		wpVer := meta.WordPressVersion
		if wpVer == "" {
			wpVer = "N/A"
		}
		dbVer := meta.DBVersion
		if dbVer == "" {
			dbVer = "N/A"
		}

		displayDir := shortenPath(meta.Directory, dirWidth-3)

		rowCells = append(rowCells, tableCellStyle.Width(nameWidth).Render(instanceName))
		rowCells = append(rowCells, tableCellStyle.Width(portWidth).Render(strconv.Itoa(meta.WordPressPort)))
		rowCells = append(rowCells, tableCellStyle.Width(dateWidth).Render(meta.CreationDate))
		// UNCOMMENTED: Add WP Ver and DB Ver cells back
		rowCells = append(rowCells, tableCellStyle.Width(wpVerWidth).Render(wpVer))
		rowCells = append(rowCells, tableCellStyle.Width(dbVerWidth).Render(dbVer))
		rowCells = append(rowCells, tableCellStyle.Width(dirWidth).Render(displayDir))
		rowCells = append(rowCells, tableCellStyle.Width(statusWidth).Render(renderStatus(meta.Status)))

		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, rowCells...))
	}

	fmt.Println(tableBorderStyle.Render(lipgloss.JoinVertical(lipgloss.Left, rows...)))
}

// Helper function to shorten paths for display (keep this)
func shortenPath(path string, maxLen int) string {
	// ... (implementation remains the same) ...
	if len(path) <= maxLen {
		return path
	}
	home, err := os.UserHomeDir()
	if err == nil && strings.HasPrefix(path, home) {
		shortened := "~" + path[len(home):]
		if len(shortened) <= maxLen {
			return shortened
		}
	}
	if maxLen < 4 { // Need space for ellipsis
		return "..."
	}
	return "..." + path[len(path)-(maxLen-3):]
}

// --- Enhanced Doctor Function ---

func doctor() {
	printSectionHeader("System Doctor: Environment Check")
	var issuesFound = 0 // Reset or initialize counter

	fmt.Println(lipgloss.NewStyle().Bold(true).Render("\n--- System ---"))
	printInfo("Operating System", runtime.GOOS)
	printInfo("Architecture", runtime.GOARCH)

	fmt.Println(lipgloss.NewStyle().Bold(true).MarginTop(1).Render("\n--- Docker Environment ---"))
	dockerPath, dockerOk := checkExecutable("docker")
	if !dockerOk {
		printError("Docker Not Found", "The 'docker' command is required but not found in your system's PATH.")
		issuesFound++
		printInfo("Doctor Check Complete (stopped early due to missing Docker)")
		return
	}
	printSuccess("Docker Found", fmt.Sprintf("Executable path: %s", dockerPath))
	cmdDockerInfo := exec.Command("docker", "info")
	cmdDockerInfo.Stdout = io.Discard
	var stderrDockerInfo bytes.Buffer
	cmdDockerInfo.Stderr = &stderrDockerInfo
	if err := cmdDockerInfo.Run(); err != nil {
		printError("Docker Daemon Not Responding", "Could not connect to the Docker daemon.", fmt.Sprintf("Error: %v", err), stderrDockerInfo.String())
		issuesFound++
	} else {
		printSuccess("Docker Daemon Responding")
	}

	if dockerOk {
		cmdComposeV2 := exec.Command("docker", "compose", "version")
		var outV2 bytes.Buffer
		errV2Run := cmdComposeV2.Run() // We only care about the error for existence check

		cmdComposeV1 := exec.Command("docker-compose", "version")
		var outV1 bytes.Buffer
		errV1Run := cmdComposeV1.Run() // We only care about the error for existence check

		if errV2Run == nil {
			cmdComposeV2.Stdout = &outV2 // Re-run to get output if successful
			_ = cmdComposeV2.Run()
			versionLine := strings.SplitN(strings.TrimSpace(outV2.String()), "\n", 2)[0]
			printSuccess("Docker Compose Found", fmt.Sprintf("'docker compose' (v2) is available. (%s)", versionLine))
		} else if errV1Run == nil {
			cmdComposeV1.Stdout = &outV1 // Re-run to get output
			_ = cmdComposeV1.Run()
			versionLine := strings.SplitN(strings.TrimSpace(outV1.String()), "\n", 2)[0]
			printWarning("Docker Compose Found (v1)", fmt.Sprintf("'docker-compose' (v1) is available. (%s)", versionLine), "Consider upgrading to Docker Compose v2.")
		} else {
			printError("Docker Compose Not Found", "Neither 'docker compose' (v2) nor 'docker-compose' (v1) executed successfully.")
			issuesFound++
		}
	}

	fmt.Println(lipgloss.NewStyle().Bold(true).MarginTop(1).Render("\n--- Project Template (Embedded) ---"))
	templateHealthy := true
	manageBinaryNameInTemplateForEmbedCheck := "manage" // Default for Linux/Darwin
	// The doctor checks what *should have been embedded* based on the OS wpid was built for.
	if runtime.GOOS == "windows" {
		manageBinaryNameInTemplateForEmbedCheck = "manage.exe"
	}

	requiredFilesInEmbeddedTemplate := []string{
		envTemplateFileName,
		"docker-compose.yml",
		manageBinaryNameInTemplateForEmbedCheck, // Check for OS-specific manage binary
	}

	for _, file := range requiredFilesInEmbeddedTemplate {
		filePathInEmbed := filepath.Join(embeddedTemplateRoot, file)
		if _, err := defaultWordpressTemplate.ReadFile(filePathInEmbed); err != nil {
			printError(fmt.Sprintf("Embedded Template File Missing: %s", file),
				fmt.Sprintf("File '%s' not found within the embedded template at '%s'.", file, filePathInEmbed),
				"This is likely an issue with the build process (Makefile) or embed directives.",
				fmt.Sprintf("Error: %v", err))
			issuesFound++
			templateHealthy = false
		} else {
			printSuccess(fmt.Sprintf("Embedded Template File Found: %s", file), fmt.Sprintf("'%s' exists in embedded template.", filePathInEmbed))
		}
	}
	if templateHealthy {
		printSuccess("Embedded Template Structure OK", "Essential files found in the embedded template.")
	}

	fmt.Println(lipgloss.NewStyle().Bold(true).MarginTop(1).Render("\n--- Go Build Environment (for wpid itself) ---"))
	goPath, goOk := checkExecutable("go")
	if !goOk {
		printWarning("Go Compiler Not Found", "The 'go' command was not found in your system's PATH.", "This is not critical for running pre-compiled wpid, but needed for development.")
	} else {
		printSuccess("Go Compiler Found", fmt.Sprintf("Executable path: %s", goPath))
	}

	fmt.Println(lipgloss.NewStyle().Bold(true).MarginTop(1).Render("\n--- Filesystem Permissions ---"))
	cwd, errCwd := os.Getwd()
	if errCwd != nil {
		printError("Cannot Check Permissions", "Failed to get current working directory.", errCwd.Error())
		issuesFound++
	} else {
		tempFileName := ".wpid-doctor-write-test." + strconv.FormatInt(time.Now().UnixNano(), 10)
		tempFilePath := filepath.Join(cwd, tempFileName)
		if errWrite := os.WriteFile(tempFilePath, []byte("test"), 0600); errWrite != nil {
			printError("Write Permission Denied (Current Directory)",
				fmt.Sprintf("Cannot write files in '%s'.", cwd),
				fmt.Sprintf("Error: %v", errWrite))
			issuesFound++
		} else {
			printSuccess("Write Permission Granted (Current Directory)", fmt.Sprintf("Can write files in '%s'.", cwd))
			_ = os.Remove(tempFilePath)
		}
	}

	fmt.Println(lipgloss.NewStyle().Bold(true).MarginTop(1).Render("\n--- Network Connectivity ---"))
	hostsToCheck := []string{"hub.docker.com", "github.com", "raw.githubusercontent.com"}
	networkOk := true
	for _, host := range hostsToCheck {
		if _, err := net.LookupHost(host); err != nil {
			printWarning("Network Resolution Failed", fmt.Sprintf("Could not resolve '%s'.", host), fmt.Sprintf("Error: %v", err))
			networkOk = false
		}
	}
	if networkOk {
		printSuccess("Network Resolution OK", "Able to resolve common external hostnames.")
	}

	fmt.Println(lipgloss.NewStyle().Bold(true).MarginTop(1).Render("\n--- Doctor Check Summary ---"))
	if issuesFound == 0 {
		printSuccess("All critical checks passed!", "Your environment seems ready.")
	} else {
		errorStr := "issues"
		if issuesFound == 1 {
			errorStr = "issue"
		}
		printError(fmt.Sprintf("Found %d critical %s.", issuesFound, errorStr), "Please review the errors above.")
	}
}

// --- NEW: Metadata Management Commands ---

// registerInstance prompts for an existing instance path and adds it to the manager.
func registerInstance() {
	printSectionHeader("Register Existing Instance")

	var instancePath string
	var instanceName string

	// 1. Prompt for Path
	inputPath := huh.NewInput().
		Title("Instance Directory Path").
		Description("Enter the ABSOLUTE path to the existing WordPress instance directory.").
		Value(&instancePath).
		Validate(func(s string) error {
			absPath := s
			if !filepath.IsAbs(s) {
				var err error
				absPath, err = filepath.Abs(s)
				if err != nil {
					return fmt.Errorf("could not determine absolute path: %w", err)
				}
				// Update the variable in the outer scope if it was relative
				// Note: This closure behavior can be subtle. Better to get absolute path *after* Run().
			}

			info, err := os.Stat(absPath) // Check the potentially absolute path
			if os.IsNotExist(err) {
				return fmt.Errorf("directory not found: %s", absPath)
			}
			if err != nil {
				return fmt.Errorf("cannot access path %s: %w", absPath, err)
			}
			if !info.IsDir() {
				return fmt.Errorf("path is not a directory: %s", absPath)
			}
			// Basic check for expected files
			if _, err := os.Stat(filepath.Join(absPath, "docker-compose.yml")); os.IsNotExist(err) {
				return errors.New("docker-compose.yml not found in directory")
			}
			if _, err := os.Stat(filepath.Join(absPath, ".env")); os.IsNotExist(err) {
				return errors.New(".env not found in directory")
			}
			return nil
		})

	if err := inputPath.WithTheme(theme).Run(); err != nil {
		printError("Input Cancelled", "Path not provided.", err.Error())
		return
	}
	// Ensure path is absolute AFTER getting input
	if !filepath.IsAbs(instancePath) {
		absPath, err := filepath.Abs(instancePath)
		if err != nil {
			printError("Path Error", fmt.Sprintf("Could not get absolute path: %v", err))
			return
		}
		instancePath = absPath
	}

	instanceName = filepath.Base(instancePath) // Default name suggestion

	// 2. Try reading local meta
	localMeta, errReadLocal := readInstanceMeta(instancePath)
	if errReadLocal != nil && !errors.Is(errReadLocal, os.ErrNotExist) { // Only warn if error is not "not found"
		printWarning("Could Not Read Local Meta", fmt.Sprintf("Error reading '%s': %v", metaFileName, errReadLocal))
	}
	if localMeta == nil { // If file didn't exist or read failed badly
		localMeta = &InstanceMeta{} // Create empty struct
	}
	// Always ensure directory is the correct absolute path we validated
	localMeta.Directory = instancePath

	// 3. Confirm/Get Essential Info if Missing (Port)
	if localMeta.WordPressPort == 0 {
		envFilePath := filepath.Join(instancePath, ".env")
		envContent, errReadEnv := os.ReadFile(envFilePath)
		port := 0 // Initialize port
		if errReadEnv == nil {
			portStr := parseEnvValue(envContent, "WORDPRESS_PORT")
			p, errAtoi := strconv.Atoi(portStr)
			if errAtoi == nil && p > 0 {
				port = p // Found port in .env
			}
		}

		if port == 0 { // If still not found, prompt
			printWarning("WordPress Port Unknown", "Could not determine the WordPress port from local meta or .env.")
			var portStr string
			inputPort := huh.NewInput().
				Title("WordPress Port for this Instance").
				Description("Enter the primary port number used by WordPress (from .env).").
				Value(&portStr).
				Validate(func(s string) error {
					p, errV := strconv.Atoi(s)
					if errV != nil || p <= 0 || p > 65535 {
						return errors.New("invalid port")
					}
					return nil
				})
			if err := inputPort.WithTheme(theme).Run(); err != nil {
				printError("Input Cancelled")
				return
			}
			port, _ = strconv.Atoi(portStr)
		}
		localMeta.WordPressPort = port
	}
	// Set default/unknown values if missing
	if localMeta.CreationDate == "" {
		localMeta.CreationDate = "Unknown"
	}
	if localMeta.Status == "" {
		localMeta.Status = "Unknown"
	}

	// 4. Confirm Instance Name
	inputName := huh.NewInput().
		Title("Confirm Instance Name").
		Description("This name will be used to manage the instance.").
		Value(&instanceName) // Default is directory basename

	if err := inputName.WithTheme(theme).Run(); err != nil {
		printError("Input Cancelled")
		return
	}
	if instanceName == "" {
		printError("Invalid Input", "Instance name cannot be empty.")
		return
	}

	// 5. Read Manager Meta and Add/Update
	managerMeta, err := readManagerMeta()
	if err != nil {
		printError("Failed to Read Manager Metadata", err.Error())
		return
	}

	conflictFound := false
	conflictMessage := ""
	for name, meta := range managerMeta {
		if name == instanceName {
			conflictMessage = fmt.Sprintf("An instance named '%s' already exists (points to %s).", instanceName, meta.Directory)
			conflictFound = true
			break
		}
		if meta.Directory == instancePath {
			conflictMessage = fmt.Sprintf("The directory '%s' is already registered under the name '%s'.", instancePath, name)
			conflictFound = true
			break
		}
	}

	if conflictFound {
		printWarning("Registration Conflict", conflictMessage)
		var overwrite bool
		confirmOverwrite := huh.NewConfirm().
			Title("Conflict Found").
			Description("Overwrite existing registration entry?").
			Affirmative("Yes, overwrite").
			Negative("No, cancel registration")
		_ = confirmOverwrite.Value(&overwrite).WithTheme(theme).Run()
		if !overwrite {
			printInfo("Registration Cancelled.")
			return
		}
		printWarning("Overwriting previous registration entry.")
		// Note: If overwriting based on directory match, we might need to delete the old key first if the name is different.
		// For simplicity now, we just overwrite/add with the *new* name.
	}

	// Add/Update the entry
	managerMeta[instanceName] = *localMeta

	// 6. Write Manager Meta Back
	if err := writeManagerMeta(managerMeta); err != nil {
		printError("Failed to Write Manager Metadata", err.Error())
	} else {
		printSuccess("Instance Registered Successfully", fmt.Sprintf("'%s' (%s) added/updated.", instanceName, instancePath))
	}
}

// unregisterInstance removes an instance from the manager list without deleting files.
func unregisterInstance(instanceName string) {
	printSectionHeader("Unregister Instance")
	if instanceName == "" {
		printError("Instance Name Required", "Usage: wpid unregister <instance_name>")
		return
	}

	managerMeta, err := readManagerMeta()
	if err != nil {
		printError("Failed to Read Manager Metadata", err.Error())
		return
	}

	meta, exists := managerMeta[instanceName]
	if !exists {
		printError("Not Found", fmt.Sprintf("Instance '%s' is not registered.", instanceName))
		return
	}

	var confirm bool
	confirmPrompt := huh.NewConfirm().
		Title(fmt.Sprintf("Unregister '%s'?", instanceName)).
		Description(fmt.Sprintf("This will remove the instance from the manager's list.\nDirectory: %s\n\nIT WILL NOT DELETE ANY FILES OR DOCKER CONTAINERS/VOLUMES.", meta.Directory)).
		Affirmative("Yes, unregister").
		Negative("No, cancel")
	_ = confirmPrompt.Value(&confirm).WithTheme(theme).Run()

	if !confirm {
		printInfo("Unregistration Cancelled.")
		return
	}

	delete(managerMeta, instanceName)

	if err := writeManagerMeta(managerMeta); err != nil {
		printError("Failed to Write Manager Metadata", err.Error())
	} else {
		printSuccess("Instance Unregistered", fmt.Sprintf("'%s' removed from manager list.", instanceName))
	}
}

// pruneInstances removes registered instances whose directories are missing.
func pruneInstances() {
	printSectionHeader("Prune Missing Instances")

	managerMeta, err := readManagerMeta()
	if err != nil {
		printError("Failed to Read Manager Metadata", err.Error())
		return
	}
	if len(managerMeta) == 0 {
		printInfo("No Instances Registered", "Nothing to prune.")
		return
	}

	missingInstances := []string{}
	printInfo("Checking instance directories...")
	for name, meta := range managerMeta {
		fmt.Printf("  Checking: %s (%s)... ", name, meta.Directory)
		_, err := os.Stat(meta.Directory)
		if os.IsNotExist(err) {
			fmt.Println(errorMsgStyle.Render("Missing"))
			missingInstances = append(missingInstances, name)
		} else if err != nil {
			fmt.Println(warningMsgStyle.Render(fmt.Sprintf("Error (%v)", err)))
			// Treat access errors differently? For now, don't prune if error != NotExist
		} else {
			fmt.Println(successMsgStyle.Render("Found"))
		}
	}

	if len(missingInstances) == 0 {
		printSuccess("\nAll registered instance directories found.", "Nothing to prune.")
		return
	}

	fmt.Println()
	printWarning(fmt.Sprintf("%d Instance(s) Missing Directories:", len(missingInstances)))
	for _, name := range missingInstances {
		fmt.Println(lipgloss.NewStyle().MarginLeft(2).Render("- " + name))
	}
	fmt.Println()

	var confirm bool
	confirmPrompt := huh.NewConfirm().
		Title("Prune Missing Entries?").
		Description("Remove the registration entries for these missing instances?\nThis cannot be undone (unless you manually re-register).").
		Affirmative("Yes, prune them").
		Negative("No, keep them")
	_ = confirmPrompt.Value(&confirm).WithTheme(theme).Run()

	if !confirm {
		printInfo("Pruning Cancelled.")
		return
	}

	somethingChanged := false
	for _, name := range missingInstances {
		if _, exists := managerMeta[name]; exists {
			delete(managerMeta, name)
			somethingChanged = true
		}
	}

	if !somethingChanged {
		printInfo("No changes made to metadata.")
		return // Should not happen if missingInstances > 0
	}

	if err := writeManagerMeta(managerMeta); err != nil {
		printError("Failed to Write Pruned Manager Metadata", err.Error())
	} else {
		printSuccess("Pruning Complete", fmt.Sprintf("%d instance registration(s) removed.", len(missingInstances)))
	}
}

// locateInstance prints the directory path of a registered instance.
func locateInstance(instanceName string) {
	if instanceName == "" {
		printError("Instance Name Required", "Usage: wpid locate <instance_name>")
		return
	}

	managerMeta, err := readManagerMeta()
	if err != nil {
		printError("Failed to Read Manager Metadata", err.Error())
		return
	}

	meta, exists := managerMeta[instanceName]
	if !exists {
		printError("Not Found", fmt.Sprintf("Instance '%s' is not registered.", instanceName))
		os.Exit(1) // Exit non-zero for scripting use cases
	}

	// Print only the path for easy scripting use (e.g., cd $(wpid locate myblog))
	fmt.Println(meta.Directory)
}

// handleMetaCommand handles subcommands for 'wpid meta'.
func handleMetaCommand(args []string) {
	if len(args) < 1 {
		printError("Meta Subcommand Required", "Usage: wpid meta [show|edit]")
		return
	}
	subcommand := strings.ToLower(args[0])

	switch subcommand {
	case "show":
		metaShow(args[1:]) // Pass remaining args for flags like --json
	case "edit":
		metaEdit()
	default:
		printError("Unknown Meta Subcommand", fmt.Sprintf("Subcommand '%s' not recognized.", subcommand), "Valid subcommands are: show, edit")
	}
}

// metaShow displays the content of the manager metadata file.
func metaShow(args []string) {
	printSectionHeader("Show Manager Metadata")

	managerMeta, err := readManagerMeta()
	if err != nil {
		printError("Failed to Read Manager Metadata", err.Error())
		return
	}

	useJSON := false
	if len(args) > 0 && args[0] == "--json" {
		useJSON = true
	}

	metaPath, _ := getManagerMetaPath() // Ignore error here, already read successfully
	printInfo("Metadata File Location:", metaPath)
	fmt.Println()

	if useJSON {
		data, err := json.MarshalIndent(managerMeta, "", "  ")
		if err != nil {
			printError("Failed to Marshal Metadata to JSON", err.Error())
			return
		}
		fmt.Println(string(data))
	} else {
		if len(managerMeta) == 0 {
			printInfo("Metadata file is empty or contains no registered instances.")
			return
		}
		// Sort keys for consistent output
		keys := make([]string, 0, len(managerMeta))
		for k := range managerMeta {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		styleKey := lipgloss.NewStyle().Bold(true).Foreground(colorSecondary)
		styleValue := lipgloss.NewStyle().Foreground(colorInfo)

		for _, key := range keys {
			meta := managerMeta[key]
			fmt.Println(styleKey.Render(key + ":"))
			fmt.Printf("  %s %s\n", styleKey.Render("Directory:"), styleValue.Render(meta.Directory))
			fmt.Printf("  %s %s\n", styleKey.Render("Port:"), styleValue.Render(strconv.Itoa(meta.WordPressPort)))
			fmt.Printf("  %s %s\n", styleKey.Render("Status:"), styleValue.Render(meta.Status))
			fmt.Printf("  %s %s\n", styleKey.Render("Created:"), styleValue.Render(meta.CreationDate))
			if meta.WordPressVersion != "" {
				fmt.Printf("  %s %s\n", styleKey.Render("WP Ver:"), styleValue.Render(meta.WordPressVersion))
			}
			if meta.DBVersion != "" {
				fmt.Printf("  %s %s\n", styleKey.Render("DB Ver:"), styleValue.Render(meta.DBVersion))
			}
			fmt.Println() // Blank line between entries
		}
	}
}

// metaEdit opens the manager metadata file in the default editor.
func metaEdit() {
	printSectionHeader("Edit Manager Metadata File")

	metaPath, err := getManagerMetaPath()
	if err != nil {
		printError("Cannot Determine Metadata Path", err.Error())
		return
	}
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		// If file doesn't exist, create an empty one first
		if errWrite := writeManagerMeta(make(ManagerMeta)); errWrite != nil {
			printError("Cannot Create Initial Metadata File", errWrite.Error())
			return
		}
		printInfo("Metadata file did not exist, created empty file:", metaPath)
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		// Suggest common editors based on OS
		if runtime.GOOS == "windows" {
			editor = "notepad"
		} else {
			// Check common Linux/Mac editors
			if _, ok := checkExecutable("vim"); ok {
				editor = "vim"
			} else if _, ok := checkExecutable("nano"); ok {
				editor = "nano"
			} else if _, ok := checkExecutable("code"); ok {
				editor = "code --wait"
			} else // VS Code with wait flag
			if _, ok := checkExecutable("vi"); ok {
				editor = "vi"
			} else if _, ok := checkExecutable("emacs"); ok {
				editor = "emacs"
			}
		}

		if editor == "" {
			printError("Cannot Find Editor", "Environment variable $EDITOR is not set and common editors (vim, nano, code, vi, emacs) not found.", "Please set $EDITOR or install an editor.")
			return
		}
		printWarning("Using default/detected editor:", editor, "Set $EDITOR environment variable for preference.")
	}

	var confirm bool
	confirmPrompt := huh.NewConfirm().
		Title("Edit Metadata Directly?").
		Description(fmt.Sprintf("This will open '%s' in '%s'.\n\n%s",
			metaPath, editor,
			errorMsgStyle.Render("WARNING:"),
		) + "\n" + lipgloss.JoinVertical(lipgloss.Left,
			"- Invalid JSON will prevent wpid from reading instances.",
			"- Manually editing paths or names might break commands.",
			"- Consider making a backup of the file first.",
		)).
		Affirmative("Yes, open editor").
		Negative("No, cancel")
	_ = confirmPrompt.Value(&confirm).WithTheme(theme).Run()

	if !confirm {
		printInfo("Edit Cancelled.")
		return
	}

	printInfo("Opening metadata file in editor...", fmt.Sprintf("Command: %s %s", editor, metaPath))

	// Split editor command if it contains arguments (like "code --wait")
	editorParts := strings.Fields(editor)
	editorCmd := editorParts[0]
	editorArgs := append(editorParts[1:], metaPath)

	cmd := exec.Command(editorCmd, editorArgs...)
	cmd.Stdin = os.Stdin   // Connect editor to terminal stdin
	cmd.Stdout = os.Stdout // Connect editor to terminal stdout
	cmd.Stderr = os.Stderr // Connect editor to terminal stderr

	if err := cmd.Run(); err != nil {
		printError("Failed to Execute Editor", err.Error())
	} else {
		printSuccess("Editor closed.")
		// Optionally, try to read meta back to validate JSON?
		_, errRead := readManagerMeta()
		if errRead != nil {
			printError("Metadata Validation Failed After Edit", "The metadata file might be invalid JSON.", errRead.Error())
		} else {
			printInfo("Metadata file seems valid.")
		}
	}
}

// In cmd/wp-manager/main.go

func main() {
	flag.Parse()
	// Remove --light from os.Args so it's not treated as a command
	cleanArgs := []string{os.Args[0]}
	for _, arg := range os.Args[1:] {
		if (!strings.HasPrefix(arg, "--light") && !strings.HasPrefix(arg, "-light")) || strings.HasPrefix(arg, "--light=") || strings.HasPrefix(arg, "-light=") {
			cleanArgs = append(cleanArgs, arg)
		} else {
			lightFlagSet = true
		}
	}
	os.Args = cleanArgs

	// Load theme preference from config if not set by flag
	if !lightFlagSet {
		useLightTheme = loadThemePreferenceFromConfig()
	} else {
		saveThemePreferenceToConfig(useLightTheme)
		if useLightTheme {
			fmt.Println(infoMsgStyle.Render("ℹ Light theme selected. This preference will be used for future runs."))
		} else {
			fmt.Println(infoMsgStyle.Render("ℹ Dark theme selected. This preference will be used for future runs."))
		}
	}

	if useLightTheme {
		theme = huh.ThemeBase() // Use base as a light theme, or define your own
	} else {
		theme = huh.ThemeDracula()
	}

	// Add global config view/edit commands
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "config-view":
			viewGlobalConfig()
			return
		case "config-edit":
			editGlobalConfig()
			return
		}
	}

	if len(os.Args) < 2 {
		fmt.Println(appTitleStyle.Render("WordPress Instance Manager")) // Print title before usage on no args
		printUsage()                                                    // printUsage now calls os.Exit(1)
		return                                                          // For clarity, though os.Exit will terminate
	}

	action := strings.ToLower(os.Args[1])
	args := os.Args[2:] // Arguments after the action

	// Print title for actual commands being run
	if action != "help" && action != "-h" && action != "--help" { // Don't double print for help
		fmt.Println(appTitleStyle.Render("WordPress Instance Manager"))
	}

	switch action {
	case "create":
		createInstance()
	case "delete":
		deleteInstance() // Remember to add .WithTheme(theme) to huh prompts inside
	case "update":
		updateStatuses()
	case "list":
		listInstances()
	case "doctor":
		doctor()
	case "config": // Added
		handleConfigCommand(args)
	case "register":
		registerInstance() // Remember to add .WithTheme(theme)
	case "unregister":
		name := ""
		if len(args) > 0 {
			name = args[0]
		}
		unregisterInstance(name) // Remember to add .WithTheme(theme)
	case "prune":
		pruneInstances() // Remember to add .WithTheme(theme)
	case "locate":
		name := ""
		if len(args) > 0 {
			name = args[0]
		}
		locateInstance(name)
	case "meta":
		handleMetaCommand(args) // Remember to add .WithTheme(theme) if metaEdit uses huh
	case "help", "-h", "--help":
		fmt.Println(appTitleStyle.Render("WordPress Instance Manager")) // Print title for help too
		printUsage()
	default:
		fmt.Println(appTitleStyle.Render("WordPress Instance Manager"))
		printWarning("Unknown Action", fmt.Sprintf("Action '%s' is not recognized.", action))
		printUsage()
	}
}

func printUsage() {
	usage := lipgloss.JoinVertical(lipgloss.Left,
		warningTitle.Render("Usage:"),
		fmt.Sprintf("  %s %s", filepath.Base(os.Args[0]), commandStyle.Render("<command> [arguments...]")),
		"",
		warningTitle.Render("Available Commands:"),
		fmt.Sprintf("  %s %s", commandStyle.Render("create"), subtleStyle.Render("- Interactively create a new WP instance")),
		fmt.Sprintf("  %s %s", commandStyle.Render("list"), subtleStyle.Render("- List all registered WP instances")),
		fmt.Sprintf("  %s %s", commandStyle.Render("delete"), subtleStyle.Render("- Interactively delete a WP instance (files & Docker)")),
		fmt.Sprintf("  %s %s", commandStyle.Render("update"), subtleStyle.Render("- Check and update Docker status for all instances")),
		fmt.Sprintf("  %s %s", commandStyle.Render("doctor"), subtleStyle.Render("- Check system environment and embedded template integrity")),
		fmt.Sprintf("  %s %s", commandStyle.Render("register"), subtleStyle.Render("- Register an existing WP instance directory")),
		fmt.Sprintf("  %s %s", commandStyle.Render("unregister <name>"), subtleStyle.Render("- Remove instance <name> from manager list (files untouched)")),
		fmt.Sprintf("  %s %s", commandStyle.Render("prune"), subtleStyle.Render("- Check for & remove registrations of missing instance directories")),
		fmt.Sprintf("  %s %s", commandStyle.Render("locate <name>"), subtleStyle.Render("- Print the directory path of registered instance <name>")),
		fmt.Sprintf("  %s %s", commandStyle.Render("meta <subcommand>"), subtleStyle.Render("- Manage the central metadata file")),
		fmt.Sprintf("      %s", commandStyle.Render("show [--json]")),
		fmt.Sprintf("      %s", commandStyle.Render("edit")),
		fmt.Sprintf("  %s %s", commandStyle.Render("config"), subtleStyle.Render("- Manage global configuration")),
		fmt.Sprintf("      %s", commandStyle.Render("get <key>")),
		fmt.Sprintf("      %s", commandStyle.Render("set <key> <value>")),
		fmt.Sprintf("      %s", commandStyle.Render("show")),
		fmt.Sprintf("      %s", commandStyle.Render("edit")),
		"",
		warningTitle.Render("Configurable Keys:"),
		fmt.Sprintf("  %s %s", commandStyle.Render("sites_base_directory"), subtleStyle.Render("- Default parent directory for new instances")),
	)
	fmt.Println("\n" + infoBox.Render(usage))
}

func promptAndStartInstance(instanceDir string) {
	var startNow bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Start Instance?").
				Description(fmt.Sprintf("Do you want to change into '%s' and start the instance now?", instanceDir)).
				Affirmative("Yes, start now").
				Negative("No, just finish").
				Value(&startNow),
		),
	).WithTheme(theme)
	form.Run()

	if startNow {
		absDir, _ := filepath.Abs(instanceDir)
		if runtime.GOOS == "windows" {
			printInfo("To start your instance, run the following in a new Command Prompt:")
			fmt.Printf("cd %s && manage start\n", absDir)
			cmd := exec.Command("manage.exe", "start")
			cmd.Dir = absDir
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Run()
		} else {
			printInfo("To start your instance, run the following in your shell:")
			fmt.Printf("cd %s && ./manage start\n", absDir)
			cmd := exec.Command("./manage", "start")
			cmd.Dir = absDir
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Run()
		}
	}
}

func viewGlobalConfig() {
	cfgPath, err := getGlobalConfigPath()
	if err != nil {
		fmt.Printf("Could not determine global config path: %v\n", err)
		return
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		fmt.Printf("Could not read global config: %v\n", err)
		return
	}
	fmt.Println("--- Global Config ---")
	fmt.Println(string(data))
}

func editGlobalConfig() {
	cfgPath, err := getGlobalConfigPath()
	if err != nil {
		fmt.Printf("Could not determine global config path: %v\n", err)
		return
	}
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "nano" // fallback
	}
	cmd := exec.Command(editor, cfgPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Failed to open editor: %v\n", err)
	}
}

// List available templates (folders in templates/ with blueprint.json)
func listAvailableTemplatesWithMeta() ([]struct {
	Dir         string
	Name        string
	Description string
}, error) {
	templatesDir := "cmd/wp-manager/templates"
	dirs, err := os.ReadDir(templatesDir)
	if err != nil {
		return nil, err
	}
	var available []struct {
		Dir         string
		Name        string
		Description string
	}
	for _, d := range dirs {
		if d.IsDir() {
			blueprintPath := filepath.Join(templatesDir, d.Name(), "blueprint.json")
			data, err := os.ReadFile(blueprintPath)
			if err == nil {
				var meta struct {
					Name        string `json:"name"`
					Description string `json:"description"`
				}
				if json.Unmarshal(data, &meta) == nil {
					available = append(available, struct {
						Dir         string
						Name        string
						Description string
					}{d.Name(), meta.Name, meta.Description})
				}
			}
		}
	}
	return available, nil
}
