# GitLab Provider for Elephant

Searches GitLab for **projects** and **merge requests** assigned to, authored by, or under review by the current user.

Results are cached in a local SQLite database for fast, offline-capable search.

## Configuration

Create `~/.config/elephant/gitlab.toml`:

```toml
# Base URL of your GitLab instance
gitlab_url = "https://gitlab.com"

# Path to a file containing your GitLab Personal Access Token
pat_file = "~/.config/elephant/.gitlab_pat"

# Minutes between background API refreshes
refresh_interval = 15

# Maximum number of projects to fetch
max_projects = 1000

# Only fetch projects you are a member of
membership_only = true

# Enable history-based scoring
history = true

# Command used to open URLs
command = "xdg-open"
```

## Authentication

Create a GitLab Personal Access Token with `read_api` scope and save it to the file referenced by `pat_file`:

```sh
echo "glpat-xxxxxxxxxxxxxxxxxxxx" > ~/.config/elephant/.gitlab_pat
chmod 600 ~/.config/elephant/.gitlab_pat
```

## Actions

| Action | Description |
|--------|-------------|
| `open` | Open the project or MR in your browser |
| `copy_url` | Copy the URL to clipboard |
| `refresh` | Trigger an immediate API sync (via State action) |
| `erase_history` | Remove an item from history |

## Build

```sh
make dev      # Build and copy to /tmp/elephant/providers/
make install  # Build and copy to ~/.config/elephant/
make clean    # Remove built plugin
```
