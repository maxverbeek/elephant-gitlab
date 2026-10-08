# GitLab Provider for Elephant

Searches GitLab for **projects** and **merge requests** assigned to, authored by, or under review by the current user.

Results are cached in a local SQLite database for fast, offline-capable search.

## Configuration

Create `~/.config/elephant/gitlab.toml`:

```toml
# Base URL of your GitLab instance (its host selects the Secret Service item)
gitlab_url = "https://gitlab.com"

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

The token is read from the freedesktop Secret Service (default collection) from the item with exactly the attributes `service=gitlab` and `host=<host of gitlab_url>`:

```sh
secret-tool store --label="GitLab PAT" service gitlab host gitlab.com
```

The token needs `read_api` scope. The plugin only reads the item, and looks it up again on every refresh, so a rotated token is picked up without restarting elephant. If the lookup fails (missing or ambiguous item, locked and not unlocked, no secret service), the error is logged and cached data is served until the next refresh.

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
