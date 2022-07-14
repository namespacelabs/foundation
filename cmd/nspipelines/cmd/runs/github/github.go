// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package github

type PushEvent struct {
	Commits    []Commit   `json:"commits,omitempty"`
	HeadCommit Commit     `json:"head_commit,omitempty"`
	Repository Repository `json:"repository,omitempty"`
	Compare    string     `json:"compare,omitempty"`
	Pusher     Pusher     `json:"pusher,omitempty"`
	Ref        string     `json:"ref"`
	Sender     User       `json:"sender,omitempty"`
}

type Commit struct {
	ID        string `json:"id"`
	Author    Author `json:"author,omitempty"`
	Message   string `json:"message,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
	URL       string `json:"url,omitempty"`
}

type Repository struct {
	FullName string `json:"full_name"`
}

type Author struct {
	Email    string `json:"email,omitempty"`
	Name     string `json:"name,omitempty"`
	Username string `json:"username,omitempty"`
}

type Pusher struct {
	Email    string `json:"email"`
	Username string `json:"name"`
}

type User struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
}
