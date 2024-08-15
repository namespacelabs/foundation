package cluster

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

type gitScheme string

const (
	SSH   gitScheme = "ssh"
	HTTPS gitScheme = "https"
)

type giturl struct {
	scheme                      gitScheme
	hostname, owner, repository string
}

func (g giturl) URL() string {
	if g.scheme == SSH {
		return fmt.Sprintf("git@%s:%s/%s.git", g.hostname, g.owner, g.repository)
	} else {
		return fmt.Sprintf("https://%s/%s/%s.git", g.hostname, g.owner, g.repository)
	}
}

func parseGitUrl(url string) (giturl, error) {
	switch {
	case strings.HasPrefix(url, "https://"):
		urlSegments := strings.Split(url, "/")
		return giturl{
			scheme:     HTTPS,
			hostname:   urlSegments[1],
			owner:      path.Join(urlSegments[2 : len(urlSegments)-1]...),
			repository: strings.TrimSuffix(urlSegments[len(urlSegments)-1], ".git"),
		}, nil
	case strings.HasPrefix(url, "git@"):
		urlSegments := strings.Split(url, ":")
		ownerAndRepoName := strings.Split(urlSegments[1], "/")
		return giturl{
			scheme:     SSH,
			hostname:   strings.TrimPrefix(urlSegments[0], "git@"),
			owner:      path.Join(ownerAndRepoName[:len(ownerAndRepoName)-1]...),
			repository: strings.TrimSuffix(ownerAndRepoName[len(ownerAndRepoName)-1], ".git"),
		}, nil
	}

	return giturl{}, fmt.Errorf("url %q is not git url", url)
}

func isRelativeUrl(url string) bool {
	return strings.HasPrefix(url, "./") || strings.HasPrefix(url, "../")
}

func resolveRelativeRemoteUrl(remoteUrl, originRemoteUrl string) (giturl, error) {
	origin, err := parseGitUrl(originRemoteUrl)
	if err != nil {
		return giturl{}, err
	}

	combined := filepath.Join(origin.owner, origin.repository, remoteUrl)
	resolved := filepath.Clean(combined)
	resolvedOwnerRepoName := strings.Split(resolved, "/")
	return giturl{
		scheme:     origin.scheme,
		hostname:   origin.hostname,
		owner:      filepath.Join(resolvedOwnerRepoName[:len(resolvedOwnerRepoName)-1]...),
		repository: strings.TrimSuffix(resolvedOwnerRepoName[len(resolvedOwnerRepoName)-1], ".git"),
	}, nil
}
