package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/GFW-knocker/Xray-core/main/commands/base"
	_ "github.com/GFW-knocker/Xray-core/main/distro/all"
)

func main() {
	os.Args = getArgsV4Compatible()

	// Detect the configuration file from arguments
	configFilePath := getConfigFilePathToEdit()
	if configFilePath == "" {
		fmt.Println("Error: No configuration file specified and default 'config.json' not found.")
		return
	}
	fmt.Printf("ConfigFile: %s\n", configFilePath)

	// Load, modify, and save the configuration file
	err := processConfigFile(configFilePath, "fragment.json")
	if err != nil {
		fmt.Println("Error processing config file:", err)
		return
	}

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

// getConfigFilePath extracts the configuration file path from command-line arguments.
func getConfigFilePathToEdit() string {
	// Default file name
	defaultConfigFile := "config.json"

	// Iterate over command-line arguments
	for i, arg := range os.Args {
		if arg == "-c" || arg == "--config" {
			// If the flag is present, get the next argument as the file path
			if i+1 < len(os.Args) {
				return os.Args[i+1]
			}
		} else if strings.HasPrefix(arg, "--config=") {
			// Handle the --config=<path> format
			return strings.TrimPrefix(arg, "--config=")
		}
	}

	// Check if default config.json exists
	if _, err := os.Stat(defaultConfigFile); err == nil {
		return defaultConfigFile
	}

	return ""
}

// processConfigFile modifies the specified config file based on the logic for "vmess" and adds the fragment from fragment.json.
func processConfigFile(configFilePath string, fragmentFilePath string) error {
	// Read config file
	configData, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse config file
	var config map[string]interface{}
	if err := json.Unmarshal(configData, &config); err != nil {
		return fmt.Errorf("failed to parse config JSON: %w", err)
	}

	// Read fragment file
	fragmentData, err := ioutil.ReadFile(fragmentFilePath)
	if err != nil {
		return fmt.Errorf("failed to read fragment file: %w", err)
	}

	// Parse fragment file
	var fragment map[string]interface{}
	if err := json.Unmarshal(fragmentData, &fragment); err != nil {
		return fmt.Errorf("failed to parse fragment JSON: %w", err)
	}

	// Check and modify the "outbounds" array
	outbounds, ok := config["outbounds"].([]interface{})
	if !ok {
		return fmt.Errorf("missing or invalid 'outbounds' array in config")
	}

	// Modify existing outbounds and add the fragment
	outbounds = modifyOutbounds(outbounds, fragment)
	config["outbounds"] = outbounds

	// Write back the updated config file
	updatedConfigData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal updated config JSON: %w", err)
	}

	if err := ioutil.WriteFile(configFilePath, updatedConfigData, 0644); err != nil {
		return fmt.Errorf("failed to write updated config file: %w", err)
	}

	return nil
}

// modifyOutbounds updates "vmess" outbounds and appends the fragment if it doesn't exist.
func modifyOutbounds(outbounds []interface{}, fragment map[string]interface{}) []interface{} {
	fragmentExists := false

	for _, item := range outbounds {
		outbound, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		// Check if the "fragment" outbound already exists
		if tag, ok := outbound["tag"].(string); ok && tag == "fragment" {
			fragmentExists = true
		}

		// Update "vmess" outbound
		if protocol, ok := outbound["protocol"].(string); ok && protocol == "vmess" {
			streamSettings, ok := outbound["streamSettings"].(map[string]interface{})
			if !ok {
				streamSettings = map[string]interface{}{}
			}

			sockopt, ok := streamSettings["sockopt"].(map[string]interface{})
			if !ok {
				sockopt = map[string]interface{}{}
			}

			// Add "dialerProxy": "fragment" if not already present
			if _, exists := sockopt["dialerProxy"]; !exists {
				sockopt["dialerProxy"] = "fragment"
			}

			streamSettings["sockopt"] = sockopt
			outbound["streamSettings"] = streamSettings
		}
	}

	// Add the fragment outbound if it doesn't exist
	if !fragmentExists {
		outbounds = append(outbounds, fragment)
	}

	return outbounds
}

// getArgsV4Compatible ensures backward compatibility with command-line arguments.
func getArgsV4Compatible() []string {
	if len(os.Args) == 1 {
		return []string{os.Args[0], "run"}
	}
	if os.Args[1][0] != '-' {
		return os.Args
	}
	version := false
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	fs.BoolVar(&version, "version", false, "")
	// parse silently, no usage, no error output
	fs.Usage = func() {}
	fs.SetOutput(&null{})
	err := fs.Parse(os.Args[1:])
	if err == flag.ErrHelp {
		// fmt.Println("DEPRECATED: -h, WILL BE REMOVED IN V5.")
		// fmt.Println("PLEASE USE: xray help")
		// fmt.Println()
		return []string{os.Args[0], "help"}
	}
	if version {
		// fmt.Println("DEPRECATED: -version, WILL BE REMOVED IN V5.")
		// fmt.Println("PLEASE USE: xray version")
		// fmt.Println()
		return []string{os.Args[0], "version"}
	}
	// fmt.Println("COMPATIBLE MODE, DEPRECATED.")
	// fmt.Println("PLEASE USE: xray run [arguments] INSTEAD.")
	// fmt.Println()
	return append([]string{os.Args[0], "run"}, os.Args[1:]...)
}

type null struct{}

func (n *null) Write(p []byte) (int, error) {
	return len(p), nil
}
