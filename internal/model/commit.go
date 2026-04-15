package model

import "time"

type RepoCommit struct {
	Scope    string
	Hash     string
	Author   string
	When     time.Time
	Subject  string
	IsRoot   bool
	RepoPath string
}

type CommitFile struct {
	Scope         string
	CommitHash    string
	Path          string
	Subject       string
	When          time.Time
	Author        string
	IsRoot        bool
	RepoPath      string
	SubmodulePath string
}
