package model

import "time"

type RepoCommit struct {
	Scope    string
	Hash     string
	Author   string
	Email    string
	When     time.Time
	Subject  string
	IsRoot   bool
	RepoPath string
}

type CommitSummary struct {
	Hash      string
	Author    string
	Email     string
	When      time.Time
	Subject   string
	Body      string
	Parents   []string
	ShortStat string
	Files     []CommitFile
}

type CommitFile struct {
	Scope         string
	CommitHash    string
	Status        string
	Path          string
	Subject       string
	When          time.Time
	Author        string
	IsRoot        bool
	RepoPath      string
	SubmodulePath string
}
