package github

import "time"

// PR represents a pull request from GitHub
type PR struct {
	Number        int
	Title         string
	URL           string
	SHA           string
	Merged        bool
	CIStatus      string   // "passing", "failing", "pending", or "unknown"
	RunAttempt    int      // Maximum run_attempt from workflow runs (1 = first run, 2 = one retry, etc.)
	CherryPickFor []string // Target branches extracted from cherry-pick/* labels
}

// Commit represents a commit from GitHub
type Commit struct {
	SHA     string
	Message string
	Author  string
	Date    time.Time
}

// CherryPickPR represents a cherry-pick PR created by a bot
type CherryPickPR struct {
	Number     int
	Branch     string
	OriginalPR int
	Failed     bool // True if cherry-pick attempt failed
}

// Release represents a GitHub release
type Release struct {
	TagName     string
	Name        string
	CreatedAt   time.Time
	PublishedAt time.Time
}

// Issue represents a GitHub issue
type Issue struct {
	Number int
	Title  string
	Body   string
	URL    string
	State  string
}

// Comment represents a GitHub issue comment
type Comment struct {
	ID        int64
	Body      string
	User      string
	CreatedAt time.Time
	UpdatedAt time.Time
}
