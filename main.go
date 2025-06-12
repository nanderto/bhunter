package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Username    string `yaml:"username"`
	AppPassword string `yaml:"app_password"`
	Workspace   string `yaml:"workspace,omitempty"`
}

type Repository struct {
	Name       string    `json:"name"`
	FullName   string    `json:"full_name"`
	CreatedOn  time.Time `json:"created_on"`
	UpdatedOn  time.Time `json:"updated_on"`
	MainBranch struct {
		Name string `json:"name"`
	} `json:"mainbranch"`
}

type Branch struct {
	Name   string `json:"name"`
	Target struct {
		Date   time.Time `json:"date"`
		Author struct {
			User struct {
				DisplayName string `json:"display_name"`
			} `json:"user"`
		} `json:"author"`
	} `json:"target"`
}

type BitbucketClient struct {
	username    string
	appPassword string
	workspace   string
	baseURL     string
}

func NewBitbucketClient(username, appPassword, workspace string) *BitbucketClient {
	if workspace == "" {
		workspace = username
	}
	return &BitbucketClient{
		username:    username,
		appPassword: appPassword,
		workspace:   workspace,
		baseURL:     "https://api.bitbucket.org/2.0",
	}
}

func (c *BitbucketClient) makeRequest(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(c.username, c.appPassword)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func (c *BitbucketClient) getRepositories() ([]Repository, error) {
	url := fmt.Sprintf("%s/repositories/%s", c.baseURL, c.workspace)
	data, err := c.makeRequest(url)
	if err != nil {
		return nil, err
	}

	var response struct {
		Values []Repository `json:"values"`
	}

	err = json.Unmarshal(data, &response)
	if err != nil {
		return nil, err
	}

	return response.Values, nil
}

func (c *BitbucketClient) getRepository(repoName string) (*Repository, error) {
	url := fmt.Sprintf("%s/repositories/%s/%s", c.baseURL, c.workspace, repoName)
	data, err := c.makeRequest(url)
	if err != nil {
		return nil, err
	}

	var repo Repository
	err = json.Unmarshal(data, &repo)
	if err != nil {
		return nil, err
	}

	return &repo, nil
}

func (c *BitbucketClient) getBranches(repoFullName string) ([]Branch, error) {
	url := fmt.Sprintf("%s/repositories/%s/refs/branches", c.baseURL, repoFullName)
	data, err := c.makeRequest(url)
	if err != nil {
		return nil, err
	}

	var response struct {
		Values []Branch `json:"values"`
	}

	err = json.Unmarshal(data, &response)
	if err != nil {
		return nil, err
	}

	return response.Values, nil
}

func loadConfigFromFile() (*Config, error) {
	configPaths := []string{
		"bhunter.yaml",
		"bhunter.yml",
		".bhunter.yaml",
		".bhunter.yml",
	}

	// Try current directory first
	for _, configPath := range configPaths {
		if _, err := os.Stat(configPath); err == nil {
			return readConfigFile(configPath)
		}
	}

	// Try home directory
	homeDir, err := os.UserHomeDir()
	if err == nil {
		for _, configPath := range configPaths {
			fullPath := filepath.Join(homeDir, configPath)
			if _, err := os.Stat(fullPath); err == nil {
				return readConfigFile(fullPath)
			}
		}
	}

	return nil, fmt.Errorf("no config file found")
}

func readConfigFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func createSampleConfigFile() {
	sampleConfig := `# Bitbucket Hunter Configuration
username: your_username
app_password: your_app_password
workspace: your_workspace  # Optional, defaults to username
`
	err := os.WriteFile("bhunter.yaml", []byte(sampleConfig), 0644)
	if err != nil {
		fmt.Printf("Error creating sample config file: %v\n", err)
	} else {
		fmt.Println("Sample config file 'bhunter.yaml' created. Please edit it with your credentials.")
	}
}

func formatDate(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

func isOlderThan(t time.Time, months int) bool {
	return time.Since(t) > time.Duration(months)*30*24*time.Hour
}

func printUsage() {
	fmt.Println("Bitbucket Hunter - Repository and Branch Analysis Tool")
	fmt.Println("\nUsage:")
	fmt.Println("  bhunter [options]")
	fmt.Println("\nOptions:")
	fmt.Println("  -u, --username     Bitbucket username")
	fmt.Println("  -p, --password     Bitbucket app password")
	fmt.Println("  -w, --workspace    Bitbucket workspace (optional, defaults to username)")
	fmt.Println("  -r, --repo         Repository name (optional, analyze only this repo)")
	fmt.Println("  --repo-only        Show only repository information (no branch details)")
	fmt.Println("  -c, --config       Create sample config file")
	fmt.Println("  -h, --help         Show this help message")
	fmt.Println("\nExamples:")
	fmt.Println("  bhunter                                    # Analyze all repositories with branches")
	fmt.Println("  bhunter --repo-only                        # Show only repository information")
	fmt.Println("  bhunter -r BidvestDirect                   # Analyze only BidvestDirect repo")
	fmt.Println("  bhunter -r BidvestDirect --repo-only       # Show only BidvestDirect repo info")
	fmt.Println("  bhunter -u user -p pass --repo-only        # Specify credentials, repos only")
	fmt.Println("\nConfiguration File:")
	fmt.Println("  The program will automatically look for config files in this order:")
	fmt.Println("  1. ./bhunter.yaml or ./bhunter.yml")
	fmt.Println("  2. ./.bhunter.yaml or ./.bhunter.yml")
	fmt.Println("  3. ~/bhunter.yaml or ~/bhunter.yml")
	fmt.Println("  4. ~/.bhunter.yaml or ~/.bhunter.yml")
	fmt.Println("\nExample config file (bhunter.yaml):")
	fmt.Println("  username: your_username")
	fmt.Println("  app_password: your_app_password")
	fmt.Println("  workspace: your_workspace")
	fmt.Println("\nGet app password at: https://bitbucket.org/account/settings/app-passwords/")
}

func matchesRepoName(repoName, searchName string) bool {
	// Case-insensitive partial match
	return strings.Contains(strings.ToLower(repoName), strings.ToLower(searchName))
}

func displayRepositoryInfo(repo Repository, client *BitbucketClient, yellow, red, bold func(a ...interface{}) string, repoOnly bool) {
	fmt.Printf("\n%s\n", bold("Repository: "+repo.Name))
	fmt.Printf("  Name: %s\n", repo.Name)
	fmt.Printf("  Date Created: %s\n", formatDate(repo.CreatedOn))

	lastAccessed := formatDate(repo.UpdatedOn)
	if isOlderThan(repo.UpdatedOn, 12) {
		lastAccessed = yellow(lastAccessed)
	}
	fmt.Printf("  Date Last Accessed: %s\n", lastAccessed)
	fmt.Printf("  Main Branch: %s\n", repo.MainBranch.Name)

	// Skip branch details if repo-only flag is set
	if repoOnly {
		return
	}

	fmt.Println("\n  Branches:")
	branches, err := client.getBranches(repo.FullName)
	if err != nil {
		fmt.Printf("    Error fetching branches: %v\n", err)
		return
	}

	for _, branch := range branches {
		fmt.Printf("    %s\n", bold("Branch: "+branch.Name))
		fmt.Printf("      Name: %s\n", branch.Name)
		fmt.Printf("      Date Created: %s\n", formatDate(branch.Target.Date))

		lastPush := formatDate(branch.Target.Date)
		if isOlderThan(branch.Target.Date, 6) {
			lastPush = red(lastPush)
		}
		fmt.Printf("      Date Last Pushed: %s\n", lastPush)
		fmt.Printf("      Last Pushed By: %s\n", branch.Target.Author.User.DisplayName)
		fmt.Printf("      Created By: %s\n", branch.Target.Author.User.DisplayName)
	}
}

func main() {
	var (
		username        = flag.String("u", "", "Bitbucket username")
		usernameAlt     = flag.String("username", "", "Bitbucket username")
		appPassword     = flag.String("p", "", "Bitbucket app password")
		appPasswordAlt  = flag.String("password", "", "Bitbucket app password")
		workspace       = flag.String("w", "", "Bitbucket workspace (optional)")
		workspaceAlt    = flag.String("workspace", "", "Bitbucket workspace (optional)")
		repoName        = flag.String("r", "", "Repository name (optional)")
		repoNameAlt     = flag.String("repo", "", "Repository name (optional)")
		repoOnly        = flag.Bool("repo-only", false, "Show only repository information (no branch details)")
		createConfig    = flag.Bool("c", false, "Create sample config file")
		createConfigAlt = flag.Bool("config", false, "Create sample config file")
		help            = flag.Bool("h", false, "Show help")
		helpAlt         = flag.Bool("help", false, "Show help")
	)

	flag.Parse()

	if *help || *helpAlt {
		printUsage()
		return
	}

	if *createConfig || *createConfigAlt {
		createSampleConfigFile()
		return
	}

	// Use the long form flags if short form is empty
	if *username == "" && *usernameAlt != "" {
		*username = *usernameAlt
	}
	if *appPassword == "" && *appPasswordAlt != "" {
		*appPassword = *appPasswordAlt
	}
	if *workspace == "" && *workspaceAlt != "" {
		*workspace = *workspaceAlt
	}
	if *repoName == "" && *repoNameAlt != "" {
		*repoName = *repoNameAlt
	}

	var config *Config

	// Try to load from config file first
	if *username == "" || *appPassword == "" {
		fileConfig, err := loadConfigFromFile()
		if err == nil {
			config = fileConfig
			fmt.Printf("Loaded configuration from file\n")
		}
	}

	// Override with command line arguments
	if config == nil {
		config = &Config{}
	}
	if *username != "" {
		config.Username = *username
	}
	if *appPassword != "" {
		config.AppPassword = *appPassword
	}
	if *workspace != "" {
		config.Workspace = *workspace
	}

	// Validate required fields
	if config.Username == "" || config.AppPassword == "" {
		fmt.Println("Error: Username and app password are required")
		fmt.Println("\nOptions:")
		fmt.Println("1. Use command line: bhunter -u username -p app_password")
		fmt.Println("2. Create config file: bhunter -c")
		fmt.Println("3. Use environment variables: BITBUCKET_USERNAME and BITBUCKET_APP_PASSWORD")
		fmt.Println("\nFor help: bhunter -h")

		// Fallback to environment variables
		envUsername := os.Getenv("BITBUCKET_USERNAME")
		envPassword := os.Getenv("BITBUCKET_APP_PASSWORD")
		if envUsername != "" && envPassword != "" {
			config.Username = envUsername
			config.AppPassword = envPassword
			fmt.Println("\nUsing environment variables...")
		} else {
			os.Exit(1)
		}
	}

	client := NewBitbucketClient(config.Username, config.AppPassword, config.Workspace)

	fmt.Printf("Connecting to Bitbucket workspace: %s\n", client.workspace)

	yellow := color.New(color.FgYellow).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()
	bold := color.New(color.Bold).SprintFunc()

	outputMode := "full analysis"
	if *repoOnly {
		outputMode = "repository information only"
	}

	// If specific repo requested, fetch only that repo
	if *repoName != "" {
		fmt.Printf("Fetching repository: %s (%s)\n", *repoName, outputMode)

		repo, err := client.getRepository(*repoName)
		if err != nil {
			fmt.Printf("Error fetching repository '%s': %v\n", *repoName, err)
			fmt.Println("\nTip: Repository name is case-sensitive. Try listing all repos first:")
			fmt.Println("     bhunter --repo-only")
			os.Exit(1)
		}

		fmt.Printf("\nFound repository: %s\n", repo.Name)
		displayRepositoryInfo(*repo, client, yellow, red, bold, *repoOnly)
		return
	}

	// Otherwise, fetch all repositories
	fmt.Printf("Fetching repositories (%s)...\n", outputMode)
	repos, err := client.getRepositories()
	if err != nil {
		fmt.Printf("Error fetching repositories: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nFound %d repositories:\n", len(repos))

	for _, repo := range repos {
		displayRepositoryInfo(repo, client, yellow, red, bold, *repoOnly)
	}
}
