package bitbucket

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/stretchr/testify/require"
)

func TestGatherTeam(t *testing.T) {
	bb := Bitbucket{client: &clientStub{}}
	bb.GatherType = "team"
	acc := accumulatorStub{}

	bb.Gather(&acc)
	require.Nil(t, acc.err)
	require.Equal(t, "bitbucket", acc.metric)
	require.Equal(t, "example-pr", acc.fields["title"].(string))
	require.Equal(t, "example-repo", acc.tags["source_repo"])
}

func TestGatherUser(t *testing.T) {
	bb := Bitbucket{client: &clientStub{}}
	bb.GatherType = "user"
	acc := accumulatorStub{}

	bb.Gather(&acc)
	require.Nil(t, acc.err)
	require.Equal(t, "bitbucket", acc.metric)
	require.Equal(t, "example-pr", acc.fields["title"].(string))
	require.Equal(t, "example-repo", acc.tags["source_repo"])
}

func TestGatherRepos(t *testing.T) {
	bb := Bitbucket{client: &clientStub{}}
	bb.GatherType = "repos"
	acc := accumulatorStub{}

	bb.Gather(&acc)
	require.Nil(t, acc.err)
	require.Equal(t, "bitbucket", acc.metric)
	require.Equal(t, "example-pr", acc.fields["title"].(string))
	require.Equal(t, "example-repo", acc.tags["source_repo"])
}

func TestGatherError(t *testing.T) {
	bb := Bitbucket{client: &clientStub{}}
	bb.GatherType = "pepperoni"
	acc := accumulatorStub{}

	bb.Gather(&acc)
	require.NotNil(t, acc.err)
}

func TestGetTeamMembers(t *testing.T) {
	bb := Bitbucket{client: &clientStub{}}

	users, err := bb.getTeamMembers("testteam")
	require.Nil(t, err)
	require.Equal(t, "Goldie Locks", users[0].DisplayName)
	require.Equal(t, "{6ccb2745-fe26-4fcf-9641-fc780c35f944}", users[0].ID)
	require.Equal(t, "Hunky Dunky", users[1].DisplayName)
	require.Equal(t, "{db48b438-e516-4c0e-9b19-43a8d7895faf}", users[1].ID)
}

func TestGetRepos(t *testing.T) {
	bb := Bitbucket{client: &clientStub{}}

	repos, err := bb.getRepos("testowner")
	require.Nil(t, err)
	require.Equal(t, "example-repo1", repos[0].Name)
	require.Equal(t, "example-repo1", repos[0].Slug)
	require.Equal(t, "example-repo2", repos[1].Name)
	require.Equal(t, "example-team/example-repo2", repos[1].FullName)
}

func TestGetReposPRs(t *testing.T) {
	bb := Bitbucket{client: &clientStub{}}

	repos := []repository{
		repository{
			Name: "test",
			Slug: "test",
		},
	}
	acc := accumulatorStub{}
	prs := bb.getReposPRs("testowner", repos, &acc)

	require.Equal(t, 1, prs[0].ID)
	require.Equal(t, "example-pr", prs[0].Title)
	require.Equal(t, "OPEN", prs[0].State)
	require.Equal(t, 2, prs[0].CommentCount)
	require.Equal(t, 1, prs[0].TaskCount)
	require.Equal(t, "Example Dude", prs[0].Author.DisplayName)
	require.Equal(t, "example_branch", prs[0].Source.Branch.Name)
	require.Equal(t, "example-team/example-repo", prs[0].Source.Repository.FullName)
	require.Equal(t, "master", prs[0].Destination.Branch.Name)
	require.Equal(t, "example-repo", prs[0].Destination.Repository.Name)
	require.Equal(t, "https://example.com/html", prs[0].Links.HTML.HREF)
}

func TestGetUserPRs(t *testing.T) {
	bb := Bitbucket{client: &clientStub{}}

	users := []user{}
	users = append(users, user{
		ID: "{e7ad8bc7-e3a5-4b0c-b9c3-5ba855155eb9}",
	})
	users = append(users, user{
		ID: "{ffaa712e-418a-42b6-a66b-9bb325691064}",
	})
	acc := accumulatorStub{}
	prs := bb.getUserPRs(users, &acc)

	require.Equal(t, 1, prs[0].ID)
	require.Equal(t, "example-pr", prs[0].Title)
	require.Equal(t, "OPEN", prs[0].State)
	require.Equal(t, 2, prs[0].CommentCount)
	require.Equal(t, 1, prs[0].TaskCount)
	require.Equal(t, "Example Dude", prs[0].Author.DisplayName)
	require.Equal(t, "example_branch", prs[0].Source.Branch.Name)
	require.Equal(t, "example-team/example-repo", prs[0].Source.Repository.FullName)
	require.Equal(t, "master", prs[0].Destination.Branch.Name)
	require.Equal(t, "example-repo", prs[0].Destination.Repository.Name)
	require.Equal(t, "https://example.com/html", prs[0].Links.HTML.HREF)
}

func TestAccumulatePRs(t *testing.T) {
	prs := []pullRequest{
		pullRequest{
			Title: "pr",
			State: "OPEN",
		},
	}
	acc := accumulatorStub{}
	accumulatePRs(prs, &acc)

	require.Equal(t, prs[0].Title, acc.fields["title"])
	require.Equal(t, prs[0].State, acc.tags["state"])
}

func TestNewClient(t *testing.T) {
	ctx := context.Background()
	key := "testkey"
	secret := "testsecret"

	client := newClient(ctx, key, secret)
	require.IsType(t, &http.Client{}, client)
}

func TestPaginatedGet(t *testing.T) {
	bb := Bitbucket{client: &paginatedGetClientStub{}}

	ret, err := bb.paginatedGet("https://example.com", "mahfields", "100")
	require.Nil(t, err)
	require.IsType(t, []json.RawMessage{}, ret)

	p1 := string([]byte(ret[0]))
	p2 := string([]byte(ret[1]))
	require.Equal(t, "{\"page\": 1}", p1)
	require.Equal(t, "{\"page\": 2}", p2)
}

func TestGetPRFields(t *testing.T) {
	p := getTestPRData()

	expectedReviewers := fmt.Sprintf("%s, %s, \u2705%s",
		p.Participants[0].User.DisplayName,
		p.Participants[1].User.DisplayName,
		p.Participants[2].User.DisplayName)
	expectedApproved := fmt.Sprintf("%s",
		p.Participants[2].User.DisplayName)

	test := getPRFields(p)

	require.Equal(t, p.ID, test["id"])
	require.Equal(t, p.Title, test["title"])
	require.Equal(t, p.State, test["pr_state"])
	require.Equal(t, p.CommentCount, test["comment_count"])
	require.Equal(t, p.Author.DisplayName, test["author"])
	require.Equal(t, p.CreatedOn.Unix(), test["created_on"])
	require.Equal(t, p.UpdatedOn.Unix(), test["updated_on"])
	require.Equal(t, p.Source.Repository.Name, test["src_repo"])
	require.Equal(t, p.Source.Branch.Name, test["src_branch"])
	require.Equal(t, p.Destination.Repository.Name, test["dest_repo"])
	require.Equal(t, p.Destination.Branch.Name, test["dest_branch"])
	require.Equal(t, expectedReviewers, test["reviewers"])
	require.Equal(t, expectedApproved, test["approved"])
	require.Equal(t, 1, test["approval_count"])
	require.Equal(t, p.Links.HTML.HREF, test["link"])
}

func TestGetPRTags(t *testing.T) {
	p := getTestPRData()
	test := getPRTags(p)

	require.Equal(t, p.State, test["state"])
	require.Equal(t, p.Source.Repository.Slug, test["source_repo"])
}

func getTestPRData() pullRequest {
	reviewers := []participant{
		participant{
			User: user{
				DisplayName: "Barry Bluejeans",
			},
			Role:     "REVIEWER",
			Approved: false,
		},
		participant{
			User: user{
				DisplayName: "Angus McDonald",
			},
			Role:     "REVIEWER",
			Approved: false,
		},
		participant{
			User: user{
				DisplayName: "Lucas Miller",
			},
			Role:     "REVIEWER",
			Approved: true,
		},
	}

	pullRequest := pullRequest{
		ID:           1,
		Title:        "test PR",
		State:        "OPEN",
		CommentCount: 0,
		Author: user{
			DisplayName: "Mr. Upsy",
		},
		CreatedOn: time.Unix(1581007908, 0),
		UpdatedOn: time.Unix(1581007960, 0),
		Source: merge{
			Repository: repository{
				Name: "testRepo",
			},
			Branch: branch{
				Name: "BAL-365",
			},
		},
		Destination: merge{
			Repository: repository{
				Name: "testRepo",
			},
			Branch: branch{
				Name: "master",
			},
		},
		Participants: reviewers,
		Links: links{
			HTML: link{
				HREF: "https://example.com/testPR",
			},
		},
	}
	return pullRequest
}

// Stubs
type clientStub struct{}

func (*clientStub) Do(req *http.Request) (*http.Response, error) {
	if strings.Contains(req.URL.Path, "members") {
		return getHTTPResponse("./test_data/members.json")
	}
	if strings.Contains(req.URL.Path, "pullrequests") {
		return getHTTPResponse("./test_data/pr.json")
	}
	return getHTTPResponse("./test_data/repos.json")
}

type paginatedGetClientStub struct{}

func (*paginatedGetClientStub) Do(req *http.Request) (*http.Response, error) {
	if req.URL.Query().Get("page") != "" {
		return getHTTPResponse("./test_data/paginated_get2.json")
	}
	return getHTTPResponse("./test_data/paginated_get1.json")
}

func getHTTPResponse(file string) (*http.Response, error) {
	f, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("Error reading test data file")
	}

	r := ioutil.NopCloser(strings.NewReader(string(f)))
	return &http.Response{
		Body:       r,
		StatusCode: 200,
	}, nil
}

type accumulatorStub struct {
	metric string
	fields map[string]interface{}
	tags   map[string]string
	err    error
}

func (a *accumulatorStub) AddFields(m string, fields map[string]interface{},
	tags map[string]string, t ...time.Time) {
	a.metric = m
	a.fields = fields
	a.tags = tags
}

func (a *accumulatorStub) AddError(err error) {
	a.err = err
}

func (a *accumulatorStub) AddGauge(measurement string, fields map[string]interface{},
	tags map[string]string, t ...time.Time) {
}

func (a *accumulatorStub) AddCounter(measurement string, fields map[string]interface{},
	tags map[string]string, t ...time.Time) {
}

func (a *accumulatorStub) AddSummary(measurement string, fields map[string]interface{},
	tags map[string]string, t ...time.Time) {
}

func (a *accumulatorStub) AddHistogram(measurement string, fields map[string]interface{},
	tags map[string]string, t ...time.Time) {
}

func (*accumulatorStub) AddMetric(metric telegraf.Metric) {
}

func (a *accumulatorStub) SetPrecision(precision time.Duration) {
}

func (a *accumulatorStub) WithTracking(maxTracked int) telegraf.TrackingAccumulator {
	return nil
}
