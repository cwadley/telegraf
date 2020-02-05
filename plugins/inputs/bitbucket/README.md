# Bitbucket Input Plugin
For collecting pull request information from Bitbucket.

The plugin will collect pull requests from either a team or an individual user.
When set to a team, all pull requests authored by team members will be collected.
When set to a user, all pull requests in repos owned by the user will be collected.

### Configuration
```toml
[[inputs.bitbucket]]
  ## Owner account name
  ## Will gather all pull requests authored by team members
  ## Will gather all pull requests on all repositories owned by individual user
  owner = ""

  ## Owner type: can be either "team" for a team, or "user" for an individual user
  owner_type = "team"

  ## Bitbucket OAuth consumer key and secret
  ## Enable the "private consumer" option to enable the client_credentials grant type
  oauth_key = ""
  oauth_secret = ""

  ## Timeout for HTTP requests.
  # http_timeout = "5s"

  ## Bitbucket API base URL
  bitbucket_api_base_url = "https://api.bitbucket.org/2.0"
```

### Metrics
- pull request
  - tags
    - state
		- source_repo
  - fields
    - id
		- title
		- pr_state
		- comment_count
		- author
		- created_on
		- updated_on
		- src_repo
		- src_branch
		- dest_repo
		- dest_branch
		- reviewers
		- not_approved