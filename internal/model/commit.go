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
