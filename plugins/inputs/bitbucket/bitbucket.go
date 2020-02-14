package bitbucket

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/internal"
	"github.com/influxdata/telegraf/plugins/inputs"
	"golang.org/x/oauth2/bitbucket"
	"golang.org/x/oauth2/clientcredentials"
)

// Bitbucket - plugin main structure
type Bitbucket struct {
	Owner               string            `toml:"owner"`
	GatherType          string            `toml:"gather_type"`
	OAuthKey            string            `toml:"oauth_key"`
	OAuthSecret         string            `toml:"oauth_secret"`
	BitbucketAPIBaseURL string            `toml:"bitbucket_api_base_url"`
	HTTPTimeout         internal.Duration `toml:"http_timeout"`
	client              oAuthClient
}

type oAuthClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type accumulator interface {
	AddFields(string, map[string]interface{}, map[string]string, ...time.Time)
	AddError(error)
}

const sampleConfig = `
  ## Owner account name
  ## Can be either team name or username
  owner = ""

  ## Gather type
  ## Can be either "team" to get PRs authored by all team members, "user" to get PRs
  ## authored by an individual user, or "repos" to get PRs on all repos owned by "owner".
  ## Note: due to the rate limit on Bitbucket API repository endpoints, if a large number of
  ## repositories are owned by a team or user, the "repos" option may fail.
  gather_type = "team"

  ## Bitbucket OAuth consumer key and secret
  ## Enable the "private consumer" option to enable the client_credentials grant type
  oauth_key = ""
  oauth_secret = ""

  ## Timeout for HTTP requests.
  # http_timeout = "5s"

  ## Bitbucket API base URL
  bitbucket_api_base_url = "https://api.bitbucket.org/2.0"
`

// SampleConfig returns sample configuration for this plugin.
func (bb *Bitbucket) SampleConfig() string {
	return sampleConfig
}

// Description returns the plugin description.
func (bb *Bitbucket) Description() string {
	return "Gather information on Bitbucket repositories for a user or team"
}

// Gather Bitbucket Metrics
func (bb *Bitbucket) Gather(acc telegraf.Accumulator) error {
	ctx := context.Background()

	if bb.client == nil {
		bb.client = newClient(ctx, bb.OAuthKey, bb.OAuthSecret)
	}

	if bb.GatherType == "team" {
		members, err := bb.getTeamMembers(bb.Owner)
		if err != nil {
			acc.AddError(err)
			return err
		}
		prs := bb.getUserPRs(members, acc)
		accumulatePRs(prs, acc)
	} else if bb.GatherType == "user" {
		users := []user{
			user{
				ID: bb.Owner,
			},
		}
		prs := bb.getUserPRs(users, acc)
		accumulatePRs(prs, acc)
	} else if bb.GatherType == "repos" {
		repos, err := bb.getRepos(bb.Owner)
		if err != nil {
			acc.AddError(err)
			return nil
		}

		prs := bb.getReposPRs(bb.Owner, repos, acc)
		accumulatePRs(prs, acc)
	} else {
		err := fmt.Errorf("invalid gather_type, must be either `team`, `user`, or `repos`")
		acc.AddError(err)
		return err
	}
	return nil
}

func (bb *Bitbucket) getUserPRs(members []user, acc accumulator) []pullRequest {
	var prs []pullRequest
	var wg sync.WaitGroup
	wg.Add(len(members))
	mtx := sync.Mutex{}
	for _, m := range members {
		prURL := fmt.Sprintf("%s/pullrequests/%s", bb.BitbucketAPIBaseURL, url.PathEscape(m.ID))
		go bb.getPRs(prURL, &mtx, &wg, acc, &prs)
	}
	wg.Wait()

	return prs
}

func (bb *Bitbucket) getReposPRs(user string, repos []repository, acc accumulator) []pullRequest {
	var prs []pullRequest
	var wg sync.WaitGroup
	wg.Add(len(repos))
	mtx := sync.Mutex{}
	for _, r := range repos {
		prURL := fmt.Sprintf("%s/repositories/%s/%s/pullrequests", bb.BitbucketAPIBaseURL, user, r.Slug)
		go bb.getPRs(prURL, &mtx, &wg, acc, &prs)
	}
	wg.Wait()

	return prs
}

func (bb *Bitbucket) getTeamMembers(team string) ([]user, error) {
	memberURL := fmt.Sprintf("%s/users/%s/members", bb.BitbucketAPIBaseURL, team)
	fields := "values.uuid"
	rawMembers, err := bb.paginatedGet(memberURL, fields, "100")
	if err != nil {
		return nil, err
	}

	parsedMembers := make([]user, 0)
	for _, m := range rawMembers {
		var currMember user
		err = json.Unmarshal(m, &currMember)
		if err != nil {
			return nil, err
		}
		parsedMembers = append(parsedMembers, currMember)
	}
	return parsedMembers, nil
}

func (bb *Bitbucket) getRepos(owner string) ([]repository, error) {
	repoURL := fmt.Sprintf("%s/repositories/%s", bb.BitbucketAPIBaseURL, owner)
	fields := "values.name,values.full_name,values.slug"
	// pagelen of 100 is maximum page length
	rawRepos, err := bb.paginatedGet(repoURL, fields, "100")
	if err != nil {
		return nil, err
	}

	parsedRepos := make([]repository, 0)
	for _, r := range rawRepos {
		var currRepo repository
		err = json.Unmarshal(r, &currRepo)
		if err != nil {
			return nil, err
		}
		parsedRepos = append(parsedRepos, currRepo)
	}

	return parsedRepos, nil
}

func (bb *Bitbucket) getPRs(prURL string, mtx *sync.Mutex,
	wg *sync.WaitGroup, acc accumulator, out *[]pullRequest) {
	defer wg.Done()

	fields := "values.id,values.title,values.description,values.state,values.comment_count," +
		"values.author.display_name, values.author.nickname,values.created_on," +
		"values.updated_on,values.source.repository.name,values.source.repository.full_name," +
		"values.source.repository.slug,values.source.branch,values.destination.repository.name," +
		"values.destination.repository.full_name,values.destination.repository.slug," +
		"values.destination.branch,values.participants.role,values.participants.user.display_name," +
		"values.participants.approved,values.links.html,values.task_count"
	// pagelen of 25 because the api doesn't like pagelen 100 on the pullrequests endpoint
	rawPRs, err := bb.paginatedGet(prURL, fields, "25")
	if err != nil {
		acc.AddError(err)
		return
	}

	parsedPRs := make([]pullRequest, 0)
	for _, p := range rawPRs {
		var currPR pullRequest
		err = json.Unmarshal(p, &currPR)
		if err != nil {
			acc.AddError(err)
			return
		}
		parsedPRs = append(parsedPRs, currPR)
	}

	mtx.Lock()
	*out = append(*out, parsedPRs...)
	mtx.Unlock()
}

func accumulatePRs(prs []pullRequest, acc accumulator) {
	now := time.Now()
	for _, p := range prs {
		acc.AddFields("bitbucket", getPRFields(p), getPRTags(p), now)
	}
}

func newClient(ctx context.Context, key, secret string) *http.Client {
	conf := clientcredentials.Config{
		ClientID:     key,
		ClientSecret: secret,
		TokenURL:     bitbucket.Endpoint.TokenURL,
	}
	client := conf.Client(ctx)

	return client
}

func (bb *Bitbucket) paginatedGet(reqURL, fields, pagelen string) ([]json.RawMessage, error) {
	currURL := reqURL
	values := make([]json.RawMessage, 0)

	for {
		req, err := http.NewRequest("GET", currURL, nil)
		if err != nil {
			return nil, err
		}

		q := req.URL.Query()
		if q.Get("pagelen") == "" {
			q.Add("pagelen", pagelen)
		}
		if q.Get("fields") == "" {
			q.Add("fields", fields)
		}
		req.URL.RawQuery = q.Encode()

		resp, err := bb.client.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			return nil, fmt.Errorf("Response from Bitbucket API: %s", resp.Status)
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		var currPage page
		err = json.Unmarshal(body, &currPage)
		if err != nil {
			return nil, err
		}

		values = append(values, currPage.Values...)

		if currPage.Next != "" {
			currURL = currPage.Next
		} else {
			return values, nil
		}
	}
}

func getPRFields(p pullRequest) map[string]interface{} {
	reviewers := ""
	approved := ""
	approvals := 0
	for _, r := range p.Participants {
		if r.Role == "REVIEWER" {
			if reviewers != "" {
				reviewers += ", "
			}

			if r.Approved {
				reviewers += "\u2705"
				if approved != "" {
					approved += ", "
				}
				approved += r.User.DisplayName
				approvals++
			}
			reviewers += r.User.DisplayName
		}
	}

	return map[string]interface{}{
		"id":             p.ID,
		"title":          p.Title,
		"pr_state":       p.State,
		"comment_count":  p.CommentCount,
		"task_count":     p.TaskCount,
		"author":         p.Author.DisplayName,
		"created_on":     p.CreatedOn.Unix(),
		"updated_on":     p.UpdatedOn.Unix(),
		"src_repo":       p.Source.Repository.Name,
		"src_branch":     p.Source.Branch.Name,
		"dest_repo":      p.Destination.Repository.Name,
		"dest_branch":    p.Destination.Branch.Name,
		"reviewers":      reviewers,
		"approved":       approved,
		"approval_count": approvals,
		"link":           p.Links.HTML.HREF,
	}
}

func getPRTags(p pullRequest) map[string]string {
	return map[string]string{
		"state":       p.State,
		"source_repo": p.Source.Repository.Slug,
	}
}

func init() {
	inputs.Add("bitbucket", func() telegraf.Input {
		return &Bitbucket{
			HTTPTimeout: internal.Duration{Duration: time.Second * 5},
		}
	})
}
