/*
 * wpod - WordPress management tool
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
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

const (
	appName                 = "wpod"
	defaultInstallDirUnix   = "/usr/local/bin"
	distDir                 = "./dist"
	projectRootLinkNameUnix = "./" + appName
)

var (
	styleSuccess  = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	styleError    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	styleWarning  = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	styleInfo     = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	styleCommand  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	styleBold     = lipgloss.NewStyle().Bold(true)
	styleAppTitle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("13")).PaddingBottom(1)
	styleHeader   = lipgloss.NewStyle().Bold(true).MarginTop(1).MarginBottom(1)
	theme         = huh.ThemeBase()
)

func printSuccess(msg string) { fmt.Println(styleSuccess.Render("✔ " + msg)) }
func printError(msg string)   { fmt.Fprintln(os.Stderr, styleError.Render("✖ "+msg)) }
func printWarning(msg string) { fmt.Println(styleWarning.Render("⚠ " + msg)) }
func printInfo(msg string)    { fmt.Println(styleInfo.Render("ℹ " + msg)) }
func printHeader(msg string)  { fmt.Println(styleHeader.Render("--- " + msg + " ---")) }

// checkGoPresence checks if the 'go' command is available.
func checkGoPresence() (string, bool) {
	path, err := exec.LookPath("go")
	if err != nil {
		return "", false
	}
	return path, true
}

func getCurrentOSArch() (osName, archName, binarySuffix string) {
	osName = runtime.GOOS
	archName = runtime.GOARCH
	if osName == "windows" {
		binarySuffix = ".exe"
	}
	return
}

// findTargetBinaryInDist tries to find the binary and optionally offers to build it.
func findTargetBinaryInDist(offerToBuild bool) (string, error) {
	osName, archName, binarySuffix := getCurrentOSArch()
	currentBuildBinaryName := appName + binarySuffix
	pathToCurrentBuildBinary := filepath.Join(distDir, currentBuildBinaryName)

	if _, err := os.Stat(pathToCurrentBuildBinary); err == nil {
		printInfo(fmt.Sprintf("Using binary: %s", pathToCurrentBuildBinary))
		return pathToCurrentBuildBinary, nil
	}

	suffixedBuildBinaryName := fmt.Sprintf("%s-%s-%s%s", appName, osName, archName, binarySuffix)
	pathToSuffixedBuildBinary := filepath.Join(distDir, suffixedBuildBinaryName)
	if _, err := os.Stat(pathToSuffixedBuildBinary); err == nil {
		printInfo(fmt.Sprintf("Using binary: %s", pathToSuffixedBuildBinary))
		return pathToSuffixedBuildBinary, nil
	}

	// Also check for legacy names for backward compatibility (optional)
	legacyNames := []string{"wp-manager", "wp-manager-" + osName + "-" + archName + binarySuffix}
	for _, legacy := range legacyNames {
		legacyPath := filepath.Join(distDir, legacy)
		if _, err := os.Stat(legacyPath); err == nil {
			printWarning(fmt.Sprintf("Found legacy binary: %s (please rename to 'wpod' for consistency)", legacyPath))
			return legacyPath, nil
		}
	}

	// Binary not found
	errMsg := fmt.Sprintf("Binary not found in %s. Checked for '%s' and '%s'.",
		distDir, currentBuildBinaryName, suffixedBuildBinaryName)

	if offerToBuild {
		goPath, goAvailable := checkGoPresence()
		if goAvailable {
			printWarning(errMsg) // Print the original error first
			var confirmBuild bool
			form := huh.NewForm(
				huh.NewGroup(
					huh.NewConfirm().
						Title(fmt.Sprintf("%s binary not found.", appName)).
						Description(fmt.Sprintf("The required %s binary for your system (%s-%s) was not found in the '%s' directory.\nWould you like to attempt to build it now using 'make build-current'?\n(Requires Go compiler and Make to be installed and project to be a Git clone.)", appName, osName, archName, distDir)).
						Affirmative("Yes, try to build").
						Negative("No, I'll build it manually or download it").
						Value(&confirmBuild),
				),
			).WithTheme(theme)

			err := form.Run()
			if err != nil {
				printInfo("Build cancelled by user.")
				return "", fmt.Errorf("%s Build cancelled", errMsg)
			}

			if confirmBuild {
				printInfo("Attempting to build with 'make build-current'...")
				cmd := exec.Command("make", "build-current")
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				// cmd.Dir = "."
				if err := cmd.Run(); err != nil {
					printError(fmt.Sprintf("Build attempt failed: %v", err))
					printInfo(fmt.Sprintf("Please ensure Go (%s) and Make are installed and you are in the project root.", goPath))
					return "", fmt.Errorf("%s Build failed: %w", errMsg, err)
				}
				printSuccess(fmt.Sprintf("%s built successfully!", appName))
				// After successful build, try finding it again (should be current build)
				if _, err := os.Stat(pathToCurrentBuildBinary); err == nil {
					printInfo(fmt.Sprintf("Now using newly built binary: %s", pathToCurrentBuildBinary))
					return pathToCurrentBuildBinary, nil
				}
				return "", fmt.Errorf("%s Built, but still not found at %s", errMsg, pathToCurrentBuildBinary)
			} else {
				printInfo("Build declined by user.")
				return "", fmt.Errorf("%s Build declined", errMsg)
			}
		} else {
			return "", fmt.Errorf("%s Go compiler not found, cannot offer to build", errMsg)
		}
	}
	return "", fmt.Errorf("%s", errMsg+" Please build it first.")
}

func getCurrentOSArchSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

func printWindowsInstallInstructions(binaryPathInDist string) {
	binSuffix := getCurrentOSArchSuffix()
	if binaryPathInDist == "" {
		binaryPathInDist = filepath.Join(distDir, appName+binSuffix)
	}

	printWarning(fmt.Sprintf("Windows Installation Guide (%s)", appName+binSuffix))
	fmt.Println()
	printInfo(fmt.Sprintf("This setup tool cannot automatically install %s on Windows.", appName))
	printInfo("Please follow these manual steps:")
	fmt.Println()
	printInfo("1. Build the Windows binary:")
	printInfo("   Run 'make build-current' (if on Windows) or 'make wp-manager-windows' in the project root.")
	printInfo(fmt.Sprintf("   The binary will be in '%s' (e.g., '%s').", distDir, binaryPathInDist))
	fmt.Println()
	printInfo(fmt.Sprintf("2. Choose an installation directory (e.g., 'C:\\Tools\\%s').", appName))
	fmt.Println()
	printInfo("3. Copy the binary:")
	printInfo(fmt.Sprintf("   Copy '%s' from '%s' into your chosen directory.", filepath.Base(binaryPathInDist), distDir))
	printInfo(fmt.Sprintf("   Rename it to '%s' in the destination if needed.", appName+binSuffix))
	fmt.Println()
	printInfo("4. Add the installation directory to your system's PATH (see Environment Variables).")
	fmt.Println()
	printInfo("5. Verify installation (in a *new* terminal):")
	printInfo(fmt.Sprintf("   %s --version", appName))
	fmt.Println()
	printInfo(fmt.Sprintf("To uninstall: Delete '%s' and remove its directory from PATH.", appName+binSuffix))
}

func installDevUnix(binaryPathInDist string) {
	printInfo(fmt.Sprintf("Setting up %s for local development (Unix-like)...", appName))
	linkName := projectRootLinkNameUnix
	if _, err := os.Lstat(linkName); err == nil {
		printInfo(fmt.Sprintf("Removing existing file/symlink: %s", linkName))
		if err := os.Remove(linkName); err != nil {
			printError(fmt.Sprintf("Failed to remove existing %s: %v. Attempting with sudo.", linkName, err))
			if errSudo := runCommandWithSudo("rm", "-f", linkName); errSudo != nil {
				printError(fmt.Sprintf("Sudo removal of %s also failed: %v. Please remove manually.", linkName, errSudo))
				return
			}
			printSuccess(fmt.Sprintf("Successfully removed %s with sudo.", linkName))
		}
	}

	printInfo(fmt.Sprintf("Creating symlink: %s -> %s", linkName, binaryPathInDist))
	if err := os.Symlink(binaryPathInDist, linkName); err != nil {
		printError(fmt.Sprintf("Failed to create symlink: %v", err))
		return
	}
	_ = os.Chmod(binaryPathInDist, 0755)

	printSuccess(fmt.Sprintf("%s dev symlink created at %s", appName, linkName))
	printInfo(fmt.Sprintf("You can now run '%s' from the project root.", linkName))
	printInfo(fmt.Sprintf("To remove the dev symlink, run: rm %s", linkName))
}

func installAppUnix(binaryPathInDist, installDir string) {
	if installDir == "" {
		installDir = defaultInstallDirUnix
	}
	destPath := filepath.Join(installDir, appName)
	printInfo(fmt.Sprintf("Attempting to install %s system-wide to %s (Unix-like)...", appName, destPath))

	if _, err := os.Stat(installDir); os.IsNotExist(err) {
		printWarning(fmt.Sprintf("Installation directory %s does not exist.", installDir))
		printInfo("Attempting to create it with sudo...")
		if err := runCommandWithSudo("mkdir", "-p", installDir); err != nil {
			printError(fmt.Sprintf("Failed to create directory %s with sudo: %v", installDir, err))
			return
		}
		printSuccess(fmt.Sprintf("Created directory %s with sudo.", installDir))
	}

	printInfo(fmt.Sprintf("Installing %s to %s using sudo...", binaryPathInDist, destPath))
	if err := runCommandWithSudo("cp", binaryPathInDist, destPath); err != nil {
		printError(fmt.Sprintf("Failed to copy %s to %s with sudo: %v", binaryPathInDist, destPath, err))
		return
	}
	if err := runCommandWithSudo("chmod", "+x", destPath); err != nil {
		printError(fmt.Sprintf("Failed to set execute permission on %s with sudo: %v", destPath, err))
		_ = runCommandWithSudo("rm", "-f", destPath)
		return
	}

	printSuccess(fmt.Sprintf("%s installed successfully to %s.", appName, destPath))
	printInfo("You might need to open a new terminal session.")
	printInfo(fmt.Sprintf("To uninstall, run: %s setup uninstall %s", filepath.Base(os.Args[0]), installDir))
}

func uninstallAppUnix(installDir string) {
	if installDir == "" {
		installDir = defaultInstallDirUnix
	}
	destPath := filepath.Join(installDir, appName)
	printInfo(fmt.Sprintf("Attempting to uninstall %s system-wide from %s (Unix-like)...", appName, destPath))

	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		printWarning(fmt.Sprintf("%s not found at %s. Nothing to uninstall.", appName, destPath))
		return
	}

	printInfo(fmt.Sprintf("Removing %s using sudo...", destPath))
	if err := runCommandWithSudo("rm", "-f", destPath); err != nil {
		printError(fmt.Sprintf("Failed to remove %s with sudo: %v", destPath, err))
		return
	}
	printSuccess(fmt.Sprintf("%s uninstalled successfully from %s.", appName, destPath))
}

func runCommandWithSudo(command string, args ...string) error {
	cmdArgs := append([]string{command}, args...)
	cmd := exec.Command("sudo", cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	fmt.Println(styleCommand.Render(fmt.Sprintf("Running: sudo %s %s", command, strings.Join(args, " "))))
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("sudo command 'sudo %s %s' failed: %w", command, strings.Join(args, " "), err)
	}
	return nil
}

func main() {
	fmt.Println(styleAppTitle.Render(fmt.Sprintf("%s Setup Utility", strings.Title(appName))))

	osName, _, _ := getCurrentOSArch()

	var mode string
	var installDirArg string
	offerToBuildBinary := false

	if len(os.Args) < 2 {
		usage()
		return
	}
	mode = strings.ToLower(os.Args[1])

	if (mode == "install" || mode == "uninstall") && len(os.Args) > 2 {
		installDirArg = os.Args[2]
	}

	if osName != "windows" {
		if mode == "dev" || mode == "install" {
			offerToBuildBinary = true
		}
	}

	binaryPathInDist, errBinary := findTargetBinaryInDist(offerToBuildBinary)

	switch mode {
	case "install":
		printHeader("System-wide Installation")
		if osName == "windows" {
			printWindowsInstallInstructions(binaryPathInDist)
		} else {
			if errBinary != nil {
				printError(errBinary.Error())
				printInfo("Installation aborted.")
				os.Exit(1)
			}
			installAppUnix(binaryPathInDist, installDirArg)
		}
	case "uninstall":
		printHeader("System-wide Uninstallation")
		if osName == "windows" {
			printInfo(fmt.Sprintf("For Windows uninstallation, please refer to manual steps (run '%s setup install' for details).", filepath.Base(os.Args[0])))
		} else {
			uninstallAppUnix(installDirArg)
		}
	case "dev":
		printHeader("Developer Setup")
		if osName == "windows" {
			printInfo(fmt.Sprintf("For local development on Windows with %s:", appName))
			if errBinary == nil && binaryPathInDist != "" { // If binary was found or built
				printInfo(fmt.Sprintf("Binary available at: .\\%s", binaryPathInDist))
			} else {
				printInfo(fmt.Sprintf("1. Ensure binary is built (e.g., 'make build-current'). It will be in '%s'.", distDir))
				printInfo(fmt.Sprintf("   (If not found, this tool may have offered to build it if Go is present.)"))
			}
			printInfo(fmt.Sprintf("2. Run directly: .\\%s\\%s%s", distDir, appName, getCurrentOSArchSuffix()))
			printInfo("3. Or, add project's 'dist' directory to your PowerShell PATH.")
		} else {
			if errBinary != nil {
				printError(errBinary.Error())
				printInfo("Developer setup aborted.")
				os.Exit(1)
			}
			installDevUnix(binaryPathInDist)
		}
	case "help", "--help", "-h":
		usage()
	default:
		printError(fmt.Sprintf("Unknown command: %s", mode))
		usage()
	}
}

func usage() {
	selfName := filepath.Base(os.Args[0])
	fmt.Printf("Usage: %s <command> [arguments...]\n\n", selfName)
	fmt.Println(styleBold.Render("Available Commands:"))
	fmt.Printf("  %s %s\n", styleCommand.Render("install [DIR]"), " (Unix-like) Installs system-wide. Default: "+defaultInstallDirUnix)
	fmt.Printf("                 %s\n", " (Windows) Prints manual installation instructions.")
	fmt.Printf("  %s %s\n", styleCommand.Render("uninstall [DIR]"), "(Unix-like) Uninstalls system-wide. Default: "+defaultInstallDirUnix)
	fmt.Printf("                 %s\n", " (Windows) Prints manual uninstallation instructions.")
	fmt.Printf("  %s %s\n", styleCommand.Render("dev"), "           (Unix-like) Creates local symlink for development.")
	fmt.Printf("                 %s\n", " (Windows) Prints instructions for local development.")
	fmt.Printf("  %s %s\n", styleCommand.Render("help"), "          Displays this help message.")
	fmt.Println()
	fmt.Println("Run after building wp-manager (e.g., 'make build-current').")
	os.Exit(1)
}

func promptYN(prompt string, defaultValue bool) bool {
	reader := bufio.NewReader(os.Stdin)
	defaultHint := "Y/n"
	if !defaultValue {
		defaultHint = "y/N"
	}
	fmt.Printf("%s (%s): ", prompt, defaultHint)
	input, _ := reader.ReadString('\n')
	input = strings.ToLower(strings.TrimSpace(input))
	if input == "" {
		return defaultValue
	}
	return input == "y" || input == "yes"
}

func copyFile(src, dst string, perm os.FileMode) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()
	destFile, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer destFile.Close()
	_, err = io.Copy(destFile, sourceFile)
	return err
}
