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
	"sync"
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
	Name      string    `json:"name"`
	FullName  string    `json:"full_name"`
	CreatedOn time.Time `json:"created_on"`
	UpdatedOn time.Time `json:"updated_on"`
	Owner     struct {
		DisplayName string `json:"display_name"`
		Username    string `json:"username"`
	} `json:"owner"`
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

type Commit struct {
	Hash   string    `json:"hash"`
	Date   time.Time `json:"date"`
	Author struct {
		User struct {
			DisplayName string `json:"display_name"`
		} `json:"user"`
	} `json:"author"`
	Message string `json:"message"`
}

type BitbucketClient struct {
	username    string
	appPassword string
	workspace   string
	baseURL     string
	httpClient  *http.Client
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
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *BitbucketClient) makeRequest(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(c.username, c.appPassword)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
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
	var allRepos []Repository
	url := fmt.Sprintf("%s/repositories/%s?pagelen=100", c.baseURL, c.workspace)

	for url != "" {
		data, err := c.makeRequest(url)
		if err != nil {
			return nil, err
		}

		var response struct {
			Values []Repository `json:"values"`
			Next   string       `json:"next"`
		}

		err = json.Unmarshal(data, &response)
		if err != nil {
			return nil, err
		}

		allRepos = append(allRepos, response.Values...)
		url = response.Next
	}

	return allRepos, nil
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
	var allBranches []Branch
	url := fmt.Sprintf("%s/repositories/%s/refs/branches?pagelen=100", c.baseURL, repoFullName)

	for url != "" {
		data, err := c.makeRequest(url)
		if err != nil {
			return nil, err
		}

		var response struct {
			Values []Branch `json:"values"`
			Next   string   `json:"next"`
		}

		err = json.Unmarshal(data, &response)
		if err != nil {
			return nil, err
		}

		allBranches = append(allBranches, response.Values...)
		url = response.Next
	}

	return allBranches, nil
}

func (c *BitbucketClient) getFirstCommit(repoFullName string) (*Commit, error) {
	// Get repository info to know when it was created
	parts := strings.Split(repoFullName, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repository name format")
	}

	repo, err := c.getRepository(parts[1])
	if err != nil {
		return nil, err
	}
	// Look for commits around the creation date (subtract 1 day to catch earliest commits, then 30 days after)
	startDate := repo.CreatedOn.AddDate(0, 0, -1) // 1 day before creation
	endDate := repo.CreatedOn.AddDate(0, 0, 30)   // 30 days after creation

	// Format dates for API (ISO 8601 format)
	since := startDate.Format("2006-01-02T15:04:05Z")
	until := endDate.Format("2006-01-02T15:04:05Z")

	// Use date filtering in the API call
	url := fmt.Sprintf("%s/repositories/%s/commits?pagelen=100&since=%s&until=%s",
		c.baseURL, repoFullName, since, until)

	data, err := c.makeRequest(url)
	if err != nil {
		return nil, err
	}

	var response struct {
		Values []Commit `json:"values"`
		Next   string   `json:"next"`
	}

	err = json.Unmarshal(data, &response)
	if err != nil {
		return nil, err
	}

	if len(response.Values) == 0 {
		return nil, fmt.Errorf("no commits found near creation date")
	}

	// Return the oldest commit from the filtered results (last in the list)
	return &response.Values[len(response.Values)-1], nil
}

func loadConfigFromFile() (*Config, error) {
	configPaths := []string{
		"bhunter.local.yaml", // Local override (highest priority)
		"bhunter.local.yml",
		"bhunter.yaml", // Standard config
		"bhunter.yml",
		".bhunter.local.yaml", // Hidden local override
		".bhunter.local.yml",
		".bhunter.yaml", // Hidden config
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
	fmt.Println("  -o, --output       Output old branch names (>6 months) for piping to bkiller")
	fmt.Println("  --csv              Output repository information in CSV format")
	fmt.Println("  --summary          Show summary statistics (repos, branches, old branches)")
	fmt.Println("  -c, --config       Create sample config file")
	fmt.Println("  -h, --help         Show this help message")
	fmt.Println("\nExamples:")
	fmt.Println("  bhunter                                    # Analyze all repositories with branches")
	fmt.Println("  bhunter --repo-only                        # Show only repository information")
	fmt.Println("  bhunter --summary                          # Show summary statistics only")
	fmt.Println("  bhunter -r BidvestDirect                   # Analyze only BidvestDirect repo")
	fmt.Println("  bhunter -r BidvestDirect --repo-only       # Show only BidvestDirect repo info")
	fmt.Println("  bhunter --output | bkiller                 # Find old branches and pipe to bkiller")
	fmt.Println("  bhunter -r MyRepo -o | bkiller             # Find old branches in specific repo")
	fmt.Println("\nConfiguration File:")
	fmt.Println("  The program will automatically look for config files in this order:")
	fmt.Println("  1. ./bhunter.local.yaml or ./bhunter.local.yml (local overrides)")
	fmt.Println("  2. ./bhunter.yaml or ./bhunter.yml (standard config)")
	fmt.Println("  3. ./.bhunter.local.yaml or ./.bhunter.local.yml (hidden local)")
	fmt.Println("  4. ./.bhunter.yaml or ./.bhunter.yml (hidden config)")
	fmt.Println("  5. ~/bhunter.local.yaml or ~/bhunter.local.yml (user local)")
	fmt.Println("  6. ~/bhunter.yaml or ~/bhunter.yml (user config)")
	fmt.Println("  7. ~/.bhunter.local.yaml or ~/.bhunter.local.yml (hidden user local)")
	fmt.Println("  8. ~/.bhunter.yaml or ~/.bhunter.yml (hidden user config)")
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

func outputOldBranches(repo Repository, client *BitbucketClient) {
	branches, err := client.getBranches(repo.FullName)
	if err != nil {
		// Don't output errors when in pipe mode
		return
	}

	for _, branch := range branches {
		// Skip main/master branches
		if branch.Name == "main" || branch.Name == "master" || branch.Name == "develop" {
			continue
		}

		if isOlderThan(branch.Target.Date, 6) {
			fmt.Printf("%s:%s\n", repo.FullName, branch.Name)
		}
	}
}

func displayRepositoryInfo(repo Repository, creator string, client *BitbucketClient, yellow, red, bold, green, cyan func(a ...interface{}) string, repoOnly bool) {
	fmt.Printf("\n%s\n", green("Repository: "+repo.Name))
	fmt.Printf("  Name: %s\n", repo.Name)
	fmt.Printf("  Owner: %s (%s)\n", repo.Owner.DisplayName, repo.Owner.Username)
	fmt.Printf("  Creator: %s\n", creator)

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
		fmt.Printf("    %s\n", cyan("Branch: "+branch.Name))
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

// RepositoryResult holds a repository and its processing result
type RepositoryResult struct {
	Repository Repository
	Creator    string
	Error      error
}

// processRepositoryConcurrently processes a single repository with creator lookup
func processRepositoryConcurrently(repo Repository, client *BitbucketClient, results chan<- RepositoryResult) {
	creator := "(unable to determine)"

	// Try to get the actual creator from the first commit
	firstCommit, err := client.getFirstCommit(repo.FullName)
	if err == nil && firstCommit.Author.User.DisplayName != "" {
		creator = firstCommit.Author.User.DisplayName
	}

	results <- RepositoryResult{
		Repository: repo,
		Creator:    creator,
		Error:      err,
	}
}

// processRepositoriesConcurrently processes repositories with controlled concurrency
func processRepositoriesConcurrently(repos []Repository, client *BitbucketClient, maxConcurrency int) []RepositoryResult {
	results := make(chan RepositoryResult, len(repos))
	semaphore := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup

	// Start workers
	for _, repo := range repos {
		wg.Add(1)
		go func(r Repository) {
			defer wg.Done()
			semaphore <- struct{}{} // Acquire semaphore
			processRepositoryConcurrently(r, client, results)
			<-semaphore // Release semaphore
		}(repo)
	}

	// Close results channel when all workers are done
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var repoResults []RepositoryResult
	for result := range results {
		repoResults = append(repoResults, result)
	}

	return repoResults
}

// outputCSVHeader prints the CSV header
func outputCSVHeader() {
	fmt.Println("Repository Name,Owner,Creator,Date Created,Date Last Accessed,Main Branch,Repo Age (months),Last Access (months),Branch Name,Branch Date Created,Branch Last Pushed,Branch Last Pushed By,Branch Age (months)")
}

// outputRepositoryCSV outputs repository information in CSV format
func outputRepositoryCSV(repo Repository, creator string, client *BitbucketClient, repoOnly bool) {
	now := time.Now()
	repoAge := calculateMonthsDifference(repo.CreatedOn, now)
	lastAccessAge := calculateMonthsDifference(repo.UpdatedOn, now)

	// Escape commas and quotes in text fields
	name := escapeCSV(repo.Name)
	ownerDisplay := escapeCSV(repo.Owner.DisplayName)
	creatorDisplay := escapeCSV(creator)
	mainBranch := escapeCSV(repo.MainBranch.Name)

	if repoOnly {
		// Repository-only mode: output single row without branch details
		fmt.Printf("%s,%s,%s,%s,%s,%s,%d,%d,,,,,\n",
			name,
			ownerDisplay,
			creatorDisplay,
			repo.CreatedOn.Format("2006-01-02"),
			repo.UpdatedOn.Format("2006-01-02"),
			mainBranch,
			repoAge,
			lastAccessAge)
	} else {
		// Include branch information
		branches, err := client.getBranches(repo.FullName)
		if err != nil {
			// Output repository row with error indication
			fmt.Printf("%s,%s,%s,%s,%s,%s,%d,%d,ERROR: %s,,,\n",
				name,
				ownerDisplay,
				creatorDisplay,
				repo.CreatedOn.Format("2006-01-02"),
				repo.UpdatedOn.Format("2006-01-02"),
				mainBranch,
				repoAge,
				lastAccessAge,
				escapeCSV(err.Error()))
			return
		}

		for _, branch := range branches {
			branchAge := calculateMonthsDifference(branch.Target.Date, now)
			branchName := escapeCSV(branch.Name)
			lastPushedBy := escapeCSV(branch.Target.Author.User.DisplayName)

			fmt.Printf("%s,%s,%s,%s,%s,%s,%d,%d,%s,%s,%s,%s,%d\n",
				name,
				ownerDisplay,
				creatorDisplay,
				repo.CreatedOn.Format("2006-01-02"),
				repo.UpdatedOn.Format("2006-01-02"),
				mainBranch,
				repoAge,
				lastAccessAge,
				branchName,
				branch.Target.Date.Format("2006-01-02"),
				branch.Target.Date.Format("2006-01-02"),
				lastPushedBy,
				branchAge)
		}
	}
}

// escapeCSV escapes commas and quotes in CSV fields
func escapeCSV(field string) string {
	if strings.Contains(field, ",") || strings.Contains(field, "\"") || strings.Contains(field, "\n") {
		// Replace quotes with double quotes and wrap in quotes
		field = strings.ReplaceAll(field, "\"", "\"\"")
		return "\"" + field + "\""
	}
	return field
}

// SummaryStats holds summary statistics
type SummaryStats struct {
	TotalRepos     int
	TotalBranches  int
	OldBranches    int
	OldRepos       int
	RecentRepos    int
	RecentBranches int
}

// calculateSummaryStats calculates summary statistics for repositories and branches
func calculateSummaryStats(repos []Repository, client *BitbucketClient) (*SummaryStats, error) {
	stats := &SummaryStats{
		TotalRepos: len(repos),
	}

	for _, repo := range repos {
		// Check if repo is old (>12 months since last access)
		if isOlderThan(repo.UpdatedOn, 12) {
			stats.OldRepos++
		} else {
			stats.RecentRepos++
		}

		// Get branches for each repository
		branches, err := client.getBranches(repo.FullName)
		if err != nil {
			// Skip repos with branch fetch errors but continue processing
			continue
		}

		stats.TotalBranches += len(branches)

		for _, branch := range branches {
			if isOlderThan(branch.Target.Date, 6) {
				stats.OldBranches++
			} else {
				stats.RecentBranches++
			}
		}
	}

	return stats, nil
}

// displaySummaryStats displays the summary statistics
func displaySummaryStats(stats *SummaryStats, yellow, red, green, cyan func(a ...interface{}) string) {
	fmt.Printf("\n%s\n", green("=== BITBUCKET WORKSPACE SUMMARY ==="))
	fmt.Printf("\n%s\n", cyan("Repository Statistics:"))
	fmt.Printf("  Total Repositories: %d\n", stats.TotalRepos)

	recentReposDisplay := fmt.Sprintf("%d", stats.RecentRepos)
	oldReposDisplay := fmt.Sprintf("%d", stats.OldRepos)
	if stats.OldRepos > 0 {
		oldReposDisplay = yellow(oldReposDisplay)
	}

	fmt.Printf("  Recent Repositories (accessed within 12 months): %s\n", recentReposDisplay)
	fmt.Printf("  Old Repositories (no access for >12 months): %s\n", oldReposDisplay)

	if stats.TotalRepos > 0 {
		oldRepoPercent := float64(stats.OldRepos) / float64(stats.TotalRepos) * 100
		fmt.Printf("  Old Repository Percentage: %.1f%%\n", oldRepoPercent)
	}

	fmt.Printf("\n%s\n", cyan("Branch Statistics:"))
	fmt.Printf("  Total Branches: %d\n", stats.TotalBranches)

	recentBranchesDisplay := fmt.Sprintf("%d", stats.RecentBranches)
	oldBranchesDisplay := fmt.Sprintf("%d", stats.OldBranches)
	if stats.OldBranches > 0 {
		oldBranchesDisplay = red(oldBranchesDisplay)
	}

	fmt.Printf("  Recent Branches (updated within 6 months): %s\n", recentBranchesDisplay)
	fmt.Printf("  Old Branches (no updates for >6 months): %s\n", oldBranchesDisplay)

	if stats.TotalBranches > 0 {
		oldBranchPercent := float64(stats.OldBranches) / float64(stats.TotalBranches) * 100
		fmt.Printf("  Old Branch Percentage: %.1f%%\n", oldBranchPercent)
		avgBranchesPerRepo := float64(stats.TotalBranches) / float64(stats.TotalRepos)
		fmt.Printf("  Average Branches per Repository: %.1f\n", avgBranchesPerRepo)
	}

	fmt.Printf("\n%s\n", cyan("Cleanup Recommendations:"))
	if stats.OldBranches > 0 {
		fmt.Printf("  • Consider cleaning up %s old branches\n", red(fmt.Sprintf("%d", stats.OldBranches)))
		fmt.Printf("  • Use: bhunter --output | bkiller --dry-run\n")
	}
	if stats.OldRepos > 0 {
		fmt.Printf("  • Review %s repositories with no recent activity\n", yellow(fmt.Sprintf("%d", stats.OldRepos)))
	}
	if stats.OldBranches == 0 && stats.OldRepos == 0 {
		fmt.Printf("  • %s No cleanup needed - workspace is well maintained!\n", green("✓"))
	}
	fmt.Println()
}

// calculateMonthsDifference calculates the accurate difference in months between two dates
func calculateMonthsDifference(start, end time.Time) int {
	years := end.Year() - start.Year()
	months := int(end.Month()) - int(start.Month())
	totalMonths := years*12 + months

	// Adjust if the day hasn't been reached yet in the current month
	if end.Day() < start.Day() {
		totalMonths--
	}

	return totalMonths
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
		output          = flag.Bool("o", false, "Output old branch names (>6 months) for piping to bkiller")
		outputAlt       = flag.Bool("output", false, "Output old branch names (>6 months) for piping to bkiller")
		csv             = flag.Bool("csv", false, "Output repository information in CSV format")
		summary         = flag.Bool("summary", false, "Show summary statistics (repos, branches, old branches)")
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

	// Handle output flag
	isOutputMode := *output || *outputAlt

	var config *Config // Try to load from config file first
	if *username == "" || *appPassword == "" {
		fileConfig, err := loadConfigFromFile()
		if err == nil {
			config = fileConfig
			if !isOutputMode && !*csv && !*summary {
				fmt.Printf("Loaded configuration from file\n")
			}
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
		if !isOutputMode {
			fmt.Println("Error: Username and app password are required")
			fmt.Println("\nOptions:")
			fmt.Println("1. Use command line: bhunter -u username -p app_password")
			fmt.Println("2. Create config file: bhunter -c")
			fmt.Println("3. Use environment variables: BITBUCKET_USERNAME, BITBUCKET_APP_PASSWORD, BITBUCKET_WORKSPACE")
			fmt.Println("\nFor help: bhunter -h")
		}
		// Fallback to environment variables
		envUsername := os.Getenv("BITBUCKET_USERNAME")
		envPassword := os.Getenv("BITBUCKET_APP_PASSWORD")
		envWorkspace := os.Getenv("BITBUCKET_WORKSPACE")
		if envUsername != "" && envPassword != "" {
			config.Username = envUsername
			config.AppPassword = envPassword
			if envWorkspace != "" {
				config.Workspace = envWorkspace
			}
			if !isOutputMode && !*csv && !*summary {
				fmt.Println("\nUsing environment variables...")
			}
		} else {
			os.Exit(1)
		}
	}
	client := NewBitbucketClient(config.Username, config.AppPassword, config.Workspace)

	if !isOutputMode && !*csv && !*summary {
		fmt.Printf("Connecting to Bitbucket workspace: %s\n", client.workspace)
	}

	// Handle output mode (for piping to bkiller)
	if isOutputMode {
		if *repoName != "" {
			// Single repository
			repo, err := client.getRepository(*repoName)
			if err != nil {
				os.Exit(1)
			}
			outputOldBranches(*repo, client)
		} else {
			// All repositories
			repos, err := client.getRepositories()
			if err != nil {
				os.Exit(1)
			}
			for _, repo := range repos {
				outputOldBranches(repo, client)
			}
		}
		return
	}
	yellow := color.New(color.FgYellow).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()
	bold := color.New(color.Bold).SprintFunc()
	green := color.New(color.FgGreen, color.Bold).SprintFunc()
	cyan := color.New(color.FgCyan, color.Bold).SprintFunc()

	outputMode := "full analysis"
	if *repoOnly {
		outputMode = "repository information only"
	} else if *summary {
		outputMode = "summary statistics"
	}
	// If specific repo requested, fetch only that repo
	if *repoName != "" {
		if !*csv && !*summary {
			fmt.Printf("Fetching repository: %s (%s)\n", *repoName, outputMode)
		}
		repo, err := client.getRepository(*repoName)
		if err != nil {
			if !*csv && !*summary {
				fmt.Printf("Error fetching repository '%s': %v\n", *repoName, err)
				fmt.Println("\nTip: Repository name is case-sensitive. Try listing all repos first:")
				fmt.Println("     bhunter --repo-only")
			}
			os.Exit(1)
		}

		if !*csv && !*summary {
			fmt.Printf("\nFound repository: %s\n", repo.Name)
		}
		// Get creator for single repository
		creator := "(unable to determine)"
		firstCommit, err := client.getFirstCommit(repo.FullName)
		if err == nil && firstCommit.Author.User.DisplayName != "" {
			creator = firstCommit.Author.User.DisplayName
		}

		if *summary {
			// Create a slice with just this repository for summary calculation
			repos := []Repository{*repo}
			stats, err := calculateSummaryStats(repos, client)
			if err != nil {
				fmt.Printf("Error calculating summary statistics: %v\n", err)
				os.Exit(1)
			}
			displaySummaryStats(stats, yellow, red, green, cyan)
		} else if *csv {
			outputCSVHeader()
			outputRepositoryCSV(*repo, creator, client, *repoOnly)
		} else {
			displayRepositoryInfo(*repo, creator, client, yellow, red, bold, green, cyan, *repoOnly)
		}
		return
	}
	// Otherwise, fetch all repositories
	if !*csv && !*summary {
		fmt.Printf("Fetching repositories (%s)...\n", outputMode)
	}
	repos, err := client.getRepositories()
	if err != nil {
		if !*csv && !*summary {
			fmt.Printf("Error fetching repositories: %v\n", err)
		}
		os.Exit(1)
	}

	if !*csv && !*summary {
		fmt.Printf("\nFound %d repositories:\n", len(repos))
		// Process repositories concurrently for creator lookup
		fmt.Printf("Processing creator information concurrently...\n")
	}
	repoResults := processRepositoriesConcurrently(repos, client, 10) // Max 10 concurrent requests

	// Handle summary mode first
	if *summary {
		stats, err := calculateSummaryStats(repos, client)
		if err != nil {
			fmt.Printf("Error calculating summary statistics: %v\n", err)
			os.Exit(1)
		}
		displaySummaryStats(stats, yellow, red, green, cyan)
		return
	}

	// Handle CSV output
	if *csv {
		outputCSVHeader()
		for _, result := range repoResults {
			outputRepositoryCSV(result.Repository, result.Creator, client, *repoOnly)
		}
	} else {
		// Display results in original order
		for _, result := range repoResults {
			displayRepositoryInfo(result.Repository, result.Creator, client, yellow, red, bold, green, cyan, *repoOnly)
		}
	}
}
