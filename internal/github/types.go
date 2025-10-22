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
