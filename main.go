package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	// Define command-line flags
	registry := flag.String("registry", "", "Registry URL")
	username := flag.String("username", "", "Username")
	password := flag.String("password", "", "Password")
	npmrcPath := flag.String("npmrc", filepath.Join(os.Getenv("HOME"), ".npmrc"), "Path to .npmrc file or directory")
	useRegistry := flag.Bool("use", false, "Set the registry=... key in the .npmrc file")
	flag.Parse()

	// Validate required flags
	if *registry == "" || *username == "" || *password == "" {
		fmt.Println("All flags --registry, --username, and --password are required.")
		os.Exit(1)
	}

	// Construct the URL and request body
	requestURL := fmt.Sprintf("%s/-/user/org.couchdb.user:%s", *registry, *username)
	body := map[string]string{
		"name":     *username,
		"password": *password,
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		fmt.Printf("Error encoding JSON: %v\n", err)
		os.Exit(1)
	}

	// Send the PUT request
	req, err := http.NewRequest("PUT", requestURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		fmt.Printf("Error creating request: %v\n", err)
		os.Exit(1)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error sending request: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	// Read and parse the response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading response: %v\n", err)
		os.Exit(1)
	}

	var respData map[string]interface{}
	if err := json.Unmarshal(respBody, &respData); err != nil {
		fmt.Printf("Error parsing response JSON: %v\n", err)
		os.Exit(1)
	}

	// Check for success and extract the token
	if ok, exists := respData["ok"].(string); !exists || ok != "true" {
		fmt.Println("Login failed.")
		fmt.Printf("Response: %v\n", respData["ok"])
		os.Exit(1)
	}

	token, exists := respData["token"].(string)
	if !exists {
		fmt.Println("Token not found in response.")
		os.Exit(1)
	}

	// Parse the registry URL to strip the scheme
	parsedURL, err := url.Parse(*registry)
	if err != nil {
		fmt.Printf("Error parsing registry URL: %v\n", err)
		os.Exit(1)
	}

	// Use the host part of the URL for the .npmrc entry
	registryHost := parsedURL.Host

	// Update the .npmrc file
	npmrcEntry := fmt.Sprintf("//%s/:_authToken=%s", registryHost, token)

	// Determine the actual path to the .npmrc file
	npmrcFilePath := *npmrcPath
	if stat, err := os.Stat(*npmrcPath); err == nil && stat.IsDir() {
		npmrcFilePath = filepath.Join(*npmrcPath, ".npmrc")
	}

	// Read the existing .npmrc file
	npmrcContent, err := os.ReadFile(npmrcFilePath)
	if err != nil && !os.IsNotExist(err) {
		fmt.Printf("Error reading .npmrc file: %v\n", err)
		os.Exit(1)
	}

	// Convert the content to a string and split by lines
	lines := strings.Split(string(npmrcContent), "\n")
	entryExists := false
	registryEntry := fmt.Sprintf("registry=%s", *registry)

	// Check if the entry already exists and update it
	for i, line := range lines {
		if strings.HasPrefix(line, fmt.Sprintf("//%s/:_authToken=", registryHost)) {
			lines[i] = npmrcEntry
			entryExists = true
		}
		if *useRegistry && strings.HasPrefix(line, "registry=") {
			lines[i] = registryEntry
		}
	}

	// If the auth entry does not exist, append it
	if !entryExists {
		lines = append(lines, npmrcEntry)
	}

	// If the --use flag is set and no registry entry exists, append it
	if *useRegistry && !strings.Contains(strings.Join(lines, "\n"), registryEntry) {
		lines = append(lines, registryEntry)
	}

	// Write the updated content back to the .npmrc file
	err = os.WriteFile(npmrcFilePath, []byte(strings.Join(lines, "\n")), 0644)
	if err != nil {
		fmt.Printf("Error writing to .npmrc file: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Login successful. Token added to .npmrc.")
}
