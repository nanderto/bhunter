# Bitbucket Hunter (bhunter)

A command-line tool for analyzing Bitbucket Cloud repositories and branches. This tool helps you identify stale repositories and branches by displaying creation dates, last push dates, and contributor information with color-coded indicators for old content.

## Features

- **Repository Analysis:**
  - Repository name and creation date
  - Last accessed date (highlighted in yellow if older than 1 year)
  - Main branch identification

- **Branch Analysis:**
  - Branch name and creation date
  - Last push date (highlighted in red if older than 6 months)
  - Author information (who created and last pushed to the branch)

- **Color Indicators:**
  - 🟡 Yellow: Repository last accessed more than 1 year ago
  - 🔴 Red: Branch last pushed more than 6 months ago

## Installation

### Prerequisites
- Go 1.21 or later
- Bitbucket Cloud account with app password

### Build from Source
```bash
git clone https://github.com/yourusername/bhunter.git
cd bhunter
go mod init bhunter
go mod tidy
go build -o bhunter.exe
```

## Configuration

### 1. Bitbucket App Password
Create an app password at: https://bitbucket.org/account/settings/app-passwords/

**Required permissions:**
- Repositories: Read

### 2. Setup Configuration File
```bash
# Copy the example config file
cp bhunter.yaml.example bhunter.yaml

# Edit with your actual credentials
# Never commit bhunter.yaml to version control!
```

### 3. Authentication Options

#### Option A: Configuration File (Recommended)
Edit `bhunter.yaml`:
```yaml
username: your_username
app_password: your_app_password
workspace: your_workspace  # Optional, defaults to username
```

#### Option B: Command Line Arguments
```bash
bhunter -u your_username -p your_app_password
bhunter --username your_username --password your_app_password -w workspace_name
```

#### Option C: Environment Variables
```bash
set BITBUCKET_USERNAME=your_username
set BITBUCKET_APP_PASSWORD=your_app_password
```

## Usage

### Basic Usage
```bash
# Using config file (automatically detected)
bhunter

# Show only repository information (faster)
bhunter --repo-only

# Analyze specific repository
bhunter -r BidvestDirect

# Show specific repo info only
bhunter -r BidvestDirect --repo-only
```

### Command Line Options
```
  -u, --username     Bitbucket username
  -p, --password     Bitbucket app password
  -w, --workspace    Bitbucket workspace (optional, defaults to username)
  -r, --repo         Repository name (optional, analyze only this repo)
  --repo-only        Show only repository information (no branch details)
  -c, --config       Create sample config file
  -h, --help         Show help message
```

### Configuration File Search Order
The tool automatically searches for config files in this order:
1. `./bhunter.yaml` or `./bhunter.yml`
2. `./.bhunter.yaml` or `./.bhunter.yml`  
3. `~/bhunter.yaml` or `~/bhunter.yml`
4. `~/.bhunter.yaml` or `~/.bhunter.yml`

## Examples

```bash
# Analyze all repositories with full branch details
bhunter

# Quick overview of all repositories
bhunter --repo-only

# Analyze specific repository
bhunter -r MyRepository

# Using command line credentials
bhunter -u username -p app_password --repo-only

# Create sample config file
bhunter -c
```

## Sample Output

### Repository Overview (--repo-only)
```
Repository: my-web-app
  Name: my-web-app
  Date Created: 2023-01-15 10:30:00
  Date Last Accessed: 2024-12-01 14:22:00
  Main Branch: main
```

### Full Analysis
```
Repository: my-web-app
  Name: my-web-app
  Date Created: 2023-01-15 10:30:00
  Date Last Accessed: 2024-12-01 14:22:00
  Main Branch: main

  Branches:
    Branch: main
      Name: main
      Date Created: 2023-01-15 10:30:00
      Date Last Pushed: 2024-12-01 14:22:00
      Last Pushed By: John Doe
      Created By: John Doe

    Branch: feature/old-feature
      Name: feature/old-feature
      Date Created: 2023-03-10 09:15:00
      Date Last Pushed: 2023-04-01 16:45:00  [RED - older than 6 months]
      Last Pushed By: Jane Smith
      Created By: Jane Smith
```

## Dependencies

- `github.com/fatih/color` - Terminal color output
- `gopkg.in/yaml.v3` - YAML configuration file parsing

## Security Notes

⚠️ **IMPORTANT**: Never commit your `bhunter.yaml` file containing real credentials to version control!

- Use `bhunter.example.yaml` as a template
- Add `bhunter.yaml` to your `.gitignore`
- Use environment variables or config files with appropriate permissions
- App passwords have limited scope compared to account passwords

## Error Handling

The tool provides clear error messages for common issues:
- Invalid credentials
- Network connectivity problems
- Missing repositories or branches
- Configuration file errors

## Troubleshooting

### Common Issues

1. **"API request failed with status: 401"**
   - Check your username and app password
   - Verify app password has "Repositories: Read" permission

2. **"Error fetching repositories"**
   - Verify workspace name is correct
   - Check network connectivity
   - Ensure you have access to the workspace

3. **"No config file found"**
   - Create config file with `bhunter -c`
   - Use command line arguments instead
   - Check file permissions

### Debug Tips
- Use `bhunter -h` to see all available options
- Verify credentials with Bitbucket web interface first
- Check that the workspace name matches your Bitbucket workspace

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

MIT License - see LICENSE file for details