// Package main provides the entry point for Xray-core with enhanced configuration processing.
// This version includes automatic configuration modification capabilities for proxy and fragment settings.
//
// Configuration File Priority:
// 1. Always uses config.json if it exists (regardless of command-line arguments)
// 2. Uses specified config file only if config.json doesn't exist
// 3. When using default config.json: no modifications are applied (used as-is)
//
// Configuration Processing Order (for non-default configs only):
// 1. Proxy Configuration: Replaces existing "proxy" outbound with proxy.json content (if exists)
// 2. Fragment Configuration: Adds fragment outbound and modifies supported protocols (if fragment.json exists)
//
// Supported Files:
// - config.json: Main configuration file (prioritized if exists, used as-is without modifications)
// - proxy.json: Proxy outbound replacement configuration (optional, ignored when using default config.json)
// - fragment.json: Fragment outbound configuration (optional, ignored when using default config.json)
// - sockopt.json: Custom socket options for supported protocols (optional, ignored when using default config.json)
//
// Supported Protocols for Fragment Processing:
// - vmess, vless, trojan
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/xtls/xray-core/main/commands/base"
	_ "github.com/xtls/xray-core/main/distro/all"
)

// main is the entry point of the application.
// It processes configuration files with priority for default config.json:
// 1. Detects and validates the main configuration file (prioritizes config.json if exists)
// 2. Updates command-line arguments to ensure Xray uses the correct config file
// 3. If using default config.json: skips all processing and uses it as-is
// 4. If using custom config: processes proxy.json for proxy outbound replacement
// 5. If using custom config: processes fragment.json for fragment configuration
// 6. Executes the main Xray application
func main() {
	// Ensure backward compatibility with v4 command-line arguments
	os.Args = getArgsV4Compatible()

	// Step 1: Detect and validate the configuration file from command-line arguments
	configFilePath := getConfigFilePathToEdit()
	if configFilePath == "" {
		fmt.Println("Error: No configuration file specified and default 'config.json' not found.")
		return
	}
	fmt.Printf("ConfigFile: %s\n", configFilePath)

	// Step 2: Update os.Args to ensure Xray uses the correct config file
	os.Args = updateArgsWithConfigFile(os.Args, configFilePath)

	// Step 3: Check if we're using default config.json (skip processing if true)
	usingDefaultConfig := isUsingDefaultConfig(configFilePath)
	if usingDefaultConfig {
		fmt.Println("Using default config.json - skipping proxy and fragment processing")
	} else {
		// Step 4: Process proxy configuration (only for non-default configs)
		// This will replace any existing outbound with tag "proxy" if proxy.json exists
		err := processProxyConfig(configFilePath)
		if err != nil {
			fmt.Println("Error processing proxy config:", err)
			return
		}

		// Step 5: Process fragment configuration (only for non-default configs)
		// This will add fragment outbound and modify supported protocols if fragment.json exists
		err = processFragmentConfig(configFilePath)
		if err != nil {
			fmt.Println("Error processing fragment config:", err)
			return
		}
	}

	// Step 6: Initialize and execute the main Xray application
	base.RootCommand.Long = "Xray is a platform for building proxies."
	base.RootCommand.Commands = append(
		[]*base.Command{
			cmdRun,
			cmdVersion,
		},
		base.RootCommand.Commands...,
	)

	base.Execute()
}

// getConfigFilePathToEdit extracts the configuration file path with priority for default config.json.
// Priority order:
// 1. Always use "config.json" if it exists (regardless of command-line arguments)
// 2. Use specified config file from command-line arguments if config.json doesn't exist
// 3. Return empty string if no valid configuration file is found
//
// Supported command-line formats:
// - "-c <path>" or "--config <path>": Configuration file path as separate argument
// - "--config=<path>": Configuration file path with equals sign
//
// Returns:
// - string: Path to the configuration file
// - empty string: If no configuration file is found or specified
func getConfigFilePathToEdit() string {
	// Default configuration file name
	defaultConfigFile := "config.json"

	// Priority 1: Always use config.json if it exists
	if _, err := os.Stat(defaultConfigFile); err == nil {
		return defaultConfigFile
	}

	// Priority 2: Parse command-line arguments to find configuration file path
	for i, arg := range os.Args {
		// Handle "-c" or "--config" followed by path
		if arg == "-c" || arg == "--config" {
			// Ensure there's a next argument containing the file path
			if i+1 < len(os.Args) {
				return os.Args[i+1]
			}
		} else if strings.HasPrefix(arg, "--config=") {
			// Handle "--config=<path>" format
			return strings.TrimPrefix(arg, "--config=")
		}
	}

	// No configuration file found
	return ""
}

// updateArgsWithConfigFile updates the os.Args to ensure Xray uses the correct config file.
// This function modifies the command-line arguments to replace any existing config file
// specification with the determined config file path.
//
// Parameters:
// - args: Original command-line arguments
// - configFilePath: The config file path to use
//
// Returns:
// - []string: Updated command-line arguments with correct config file
func updateArgsWithConfigFile(args []string, configFilePath string) []string {
	newArgs := []string{args[0]} // Keep the program name

	// Skip any existing config arguments and add our determined config file
	for i := 1; i < len(args); i++ {
		arg := args[i]

		// Skip -c, --config arguments and their values
		if arg == "-c" || arg == "--config" {
			// Skip this argument and the next one (the config file path)
			if i+1 < len(args) {
				i++ // Skip the config file path argument too
			}
			continue
		} else if strings.HasPrefix(arg, "--config=") {
			// Skip --config=<path> format
			continue
		} else {
			// Keep other arguments
			newArgs = append(newArgs, arg)
		}
	}

	// Add our determined config file
	newArgs = append(newArgs, "-c", configFilePath)

	return newArgs
}

// isUsingDefaultConfig checks if we're using the default config.json file.
// When using the default config.json, we should not modify it with proxy or fragment processing.
//
// Returns:
// - bool: true if using default config.json, false otherwise
func isUsingDefaultConfig(configFilePath string) bool {
	defaultConfigFile := "config.json"

	// Check if the config file path is the default config.json and it exists
	if configFilePath == defaultConfigFile {
		if _, err := os.Stat(defaultConfigFile); err == nil {
			return true
		}
	}

	return false
}

// processProxyConfig handles proxy.json replacement if it exists.
// This function performs the following operations:
// 1. Checks if proxy.json exists in the current directory
// 2. If found, reads and parses both the main config and proxy.json
// 3. Finds the outbound with tag "proxy" in the main config
// 4. Completely replaces it with the content from proxy.json
// 5. Saves the updated configuration back to the main config file
//
// Parameters:
// - configFilePath: Path to the main configuration file
//
// Returns:
// - error: nil if successful, error details if any step fails
func processProxyConfig(configFilePath string) error {
	// Define proxy configuration file path
	proxyFilePath := "proxy.json"

	// Check if proxy.json exists - if not, skip processing
	if _, err := os.Stat(proxyFilePath); os.IsNotExist(err) {
		// proxy.json doesn't exist, nothing to do
		return nil
	}

	// Step 1: Read and parse the main configuration file
	configData, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(configData, &config); err != nil {
		return fmt.Errorf("failed to parse config JSON: %w", err)
	}

	// Step 2: Validate and extract the outbounds array from main config
	outbounds, ok := config["outbounds"].([]interface{})
	if !ok {
		return fmt.Errorf("missing or invalid 'outbounds' array in config")
	}

	// Step 3: Read and parse proxy.json
	proxyData, err := ioutil.ReadFile(proxyFilePath)
	if err != nil {
		return fmt.Errorf("failed to read proxy file: %w", err)
	}

	var proxyConfig map[string]interface{}
	if err := json.Unmarshal(proxyData, &proxyConfig); err != nil {
		return fmt.Errorf("failed to parse proxy JSON: %w", err)
	}

	// Step 4: Replace the existing proxy outbound with new configuration
	outbounds = replaceProxyOutbound(outbounds, proxyConfig)
	config["outbounds"] = outbounds
	fmt.Printf("Proxy outbound replaced with configuration from '%s'\n", proxyFilePath)

	// Step 5: Write the updated configuration back to file
	updatedConfigData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal updated config JSON: %w", err)
	}

	if err := ioutil.WriteFile(configFilePath, updatedConfigData, 0644); err != nil {
		return fmt.Errorf("failed to write updated config file: %w", err)
	}

	return nil
}

// processFragmentConfig modifies the specified config file based on fragment configuration.
// This function performs the following operations:
// 1. Checks if fragment.json exists in the current directory
// 2. If found, reads and parses the main config, fragment.json, and optionally sockopt.json
// 3. Modifies supported protocol outbounds (vmess, vless, trojan) to add sockopt settings
// 4. Adds the fragment outbound if it doesn't already exist
// 5. Saves the updated configuration back to the main config file
//
// Supported protocols for modification: vmess, vless, trojan
// Optional sockopt.json will be merged into streamSettings.sockopt for supported protocols
//
// Parameters:
// - configFilePath: Path to the main configuration file
//
// Returns:
// - error: nil if successful, error details if any step fails
func processFragmentConfig(configFilePath string) error {
	// Define fragment configuration file path
	fragmentFilePath := "fragment.json"

	// Check if fragment.json exists - if not, skip processing
	if _, err := os.Stat(fragmentFilePath); os.IsNotExist(err) {
		// fragment.json doesn't exist, nothing to do
		return nil
	}

	// Step 1: Read and parse the main configuration file
	configData, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(configData, &config); err != nil {
		return fmt.Errorf("failed to parse config JSON: %w", err)
	}

	// Step 2: Validate and extract the outbounds array from main config
	outbounds, ok := config["outbounds"].([]interface{})
	if !ok {
		return fmt.Errorf("missing or invalid 'outbounds' array in config")
	}

	// Step 3: Read and parse fragment.json
	fragmentData, err := ioutil.ReadFile(fragmentFilePath)
	if err != nil {
		return fmt.Errorf("failed to read fragment file: %w", err)
	}

	var fragment map[string]interface{}
	if err := json.Unmarshal(fragmentData, &fragment); err != nil {
		return fmt.Errorf("failed to parse fragment JSON: %w", err)
	}

	// Step 4: Optionally read and parse sockopt.json for custom socket options
	var sockoptConfig map[string]interface{}
	sockoptFilePath := "sockopt.json"
	if sockoptData, err := ioutil.ReadFile(sockoptFilePath); err == nil {
		if err := json.Unmarshal(sockoptData, &sockoptConfig); err != nil {
			fmt.Printf("Warning: Failed to parse sockopt JSON: %v\n", err)
			sockoptConfig = nil
		}
	}

	// Step 5: Modify existing outbounds and add the fragment
	outbounds = modifyOutbounds(outbounds, fragment, sockoptConfig)
	config["outbounds"] = outbounds

	// Step 6: Write the updated configuration back to file
	updatedConfigData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal updated config JSON: %w", err)
	}

	if err := ioutil.WriteFile(configFilePath, updatedConfigData, 0644); err != nil {
		return fmt.Errorf("failed to write updated config file: %w", err)
	}

	return nil
}

// replaceProxyOutbound finds and replaces the outbound with tag "proxy" with the new proxy configuration.
// This function searches through the outbounds array for an outbound with tag "proxy"
// and completely replaces it with the provided proxy configuration.
//
// Parameters:
// - outbounds: Array of outbound configurations from the main config
// - proxyConfig: New proxy configuration from proxy.json
//
// Returns:
// - []interface{}: Updated outbounds array with proxy outbound replaced
func replaceProxyOutbound(outbounds []interface{}, proxyConfig map[string]interface{}) []interface{} {
	// Iterate through all outbounds to find the one with tag "proxy"
	for i, item := range outbounds {
		outbound, ok := item.(map[string]interface{})
		if !ok {
			// Skip invalid outbound entries
			continue
		}

		// Check if this outbound has the "proxy" tag
		if tag, ok := outbound["tag"].(string); ok && tag == "proxy" {
			// Replace the entire proxy outbound with the new configuration
			outbounds[i] = proxyConfig
			break // Stop after finding and replacing the first proxy outbound
		}
	}
	return outbounds
}

// applySockoptToOutbound applies sockopt configuration to a specific outbound.
// This function modifies the streamSettings.sockopt of the given outbound by merging
// the provided sockopt configuration. It creates the necessary nested structures
// if they don't exist. Only processes if sockoptConfig is not nil.
//
// Parameters:
// - outbound: The outbound configuration to modify
// - sockoptConfig: Socket options configuration to apply (can be nil)
func applySockoptToOutbound(outbound map[string]interface{}, sockoptConfig map[string]interface{}) {
	// Only process if sockoptConfig exists (sockopt.json was found and parsed)
	if sockoptConfig == nil {
		return
	}

	// Get or create streamSettings
	streamSettings, ok := outbound["streamSettings"].(map[string]interface{})
	if !ok {
		streamSettings = map[string]interface{}{}
	}

	// Get or create sockopt within streamSettings
	sockopt, ok := streamSettings["sockopt"].(map[string]interface{})
	if !ok {
		sockopt = map[string]interface{}{}
	}

	// Merge sockopt.json content into existing sockopt
	for key, value := range sockoptConfig {
		sockopt[key] = value
	}

	// Update the outbound with modified settings
	streamSettings["sockopt"] = sockopt
	outbound["streamSettings"] = streamSettings
}

// modifyOutbounds updates supported protocol outbounds and appends the fragment if it doesn't exist.
// This function performs the following operations:
// 1. Checks if a fragment outbound already exists (tag "fragment")
// 2. For each supported protocol outbound (vmess, vless, trojan):
//   - Applies sockopt settings using separate function (only if sockopt.json exists)
//
// 3. Appends the fragment outbound if it doesn't already exist
//
// Parameters:
// - outbounds: Array of outbound configurations from the main config
// - fragment: Fragment configuration from fragment.json
// - sockoptConfig: Socket options from sockopt.json (can be nil)
//
// Returns:
// - []interface{}: Updated outbounds array with modified protocols and fragment added
func modifyOutbounds(outbounds []interface{}, fragment map[string]interface{}, sockoptConfig map[string]interface{}) []interface{} {
	fragmentExists := false
	// Define protocols that support fragment configuration
	supportedProtocols := []string{"vmess", "vless", "trojan"}

	// Process each existing outbound
	for _, item := range outbounds {
		outbound, ok := item.(map[string]interface{})
		if !ok {
			// Skip invalid outbound entries
			continue
		}

		// Check if the "fragment" outbound already exists
		if tag, ok := outbound["tag"].(string); ok && tag == "fragment" {
			fragmentExists = true
		}

		// Update supported protocol outbounds with sockopt settings
		if protocol, ok := outbound["protocol"].(string); ok && contains(supportedProtocols, protocol) {
			// Apply sockopt settings from sockopt.json (function handles nil check internally)
			applySockoptToOutbound(outbound, sockoptConfig)
		}
	}

	// Add the fragment outbound if it doesn't already exist
	if !fragmentExists {
		outbounds = append(outbounds, fragment)
	}

	return outbounds
}

// contains checks if a slice contains a specific string.
// This is a utility function used to check if a protocol is supported.
//
// Parameters:
// - slice: Array of strings to search in
// - item: String to search for
//
// Returns:
// - bool: true if item is found in slice, false otherwise
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// getArgsV4Compatible ensures backward compatibility with command-line arguments.
// This function handles the transition from v4 to v5 command-line interface by:
// 1. Adding "run" command if no arguments provided
// 2. Converting legacy flags (-version, -h) to new command format
// 3. Preserving existing command structure for new format
//
// Returns:
// - []string: Modified command-line arguments compatible with v5 format
func getArgsV4Compatible() []string {
	// If no arguments provided, default to "run" command
	if len(os.Args) == 1 {
		return []string{os.Args[0], "run"}
	}

	// If first argument is not a flag, assume new format
	if os.Args[1][0] != '-' {
		return os.Args
	}

	// Handle legacy flags for backward compatibility
	version := false
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	fs.BoolVar(&version, "version", false, "")

	// Parse silently without usage or error output
	fs.Usage = func() {}
	fs.SetOutput(&null{})
	err := fs.Parse(os.Args[1:])

	// Convert legacy help flag to new help command
	if err == flag.ErrHelp {
		// Legacy: -h (deprecated in v5)
		return []string{os.Args[0], "help"}
	}

	// Convert legacy version flag to new version command
	if version {
		// Legacy: -version (deprecated in v5)
		return []string{os.Args[0], "version"}
	}

	// Default: convert legacy format to "run" command with original arguments
	return append([]string{os.Args[0], "run"}, os.Args[1:]...)
}

// null is a helper type that implements io.Writer but discards all writes.
// Used to suppress output from flag parsing in compatibility mode.
type null struct{}

// Write implements io.Writer interface by discarding all input.
// Always returns the length of input to indicate successful "write".
func (n *null) Write(p []byte) (int, error) {
	return len(p), nil
}
