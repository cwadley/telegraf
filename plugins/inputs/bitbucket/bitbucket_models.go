package bitbucket

import (
	"encoding/json"
	"time"
)

type page struct {
	Next   string            `json:"next"`
	Values []json.RawMessage `json:"values"`
}

type repository struct {
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	Slug     string `json:"slug"`
}

type pullRequest struct {
	ID           int           `json:"id"`
	Title        string        `json:"title"`
	State        string        `json:"state"`
	CommentCount int           `json:"comment_count"`
	TaskCount    int           `json:"task_count"`
	Author       user          `json:"author"`
	CreatedOn    time.Time     `json:"created_on"`
	UpdatedOn    time.Time     `json:"updated_on"`
	Source       merge         `json:"source"`
	Destination  merge         `json:"destination"`
	Participants []participant `json:"participants"`
	Links        links         `json:"links"`
}

type participant struct {
	User     user   `json:"user"`
	Role     string `json:"role"`
	Approved bool   `json:"approved"`
}

type user struct {
	DisplayName string `json:"display_name"`
	ID          string `json:"UUID"`
}

type merge struct {
	Repository repository `json:"repository"`
	Branch     branch     `json:"branch"`
}

type branch struct {
	Name string `json:"name"`
}

type links struct {
	HTML link `json:"html"`
}

type link struct {
	HREF string `json:"href"`
}
