package gittrack

import (
	"fmt"
	"strings"

	farosv1alpha1 "github.com/pusher/faros/pkg/apis/faros/v1alpha1"
	gitstore "github.com/pusher/git-store"
)

type gitCredentials struct {
	secret         []byte
	credentialType farosv1alpha1.GitCredentialType
}

// createRepoRef creates a git repo ref configured depending on the credentialType
func createRepoRefFromCreds(url string, creds *gitCredentials) (*gitstore.RepoRef, error) {
	if creds == nil {
		creds = &gitCredentials{}
	}
	switch creds.credentialType {
	// default to SSH
	case "":
		fallthrough
	case farosv1alpha1.GitCredentialTypeSSH:
		return &gitstore.RepoRef{URL: url, PrivateKey: creds.secret}, nil
	case farosv1alpha1.GitCredentialTypeHTTPBasicAuth:
		credString := string(creds.secret)
		colenIdx := strings.Index(credString, ":")
		var username, password string
		if colenIdx == -1 {
			password = credString
		} else {
			username = credString[0:colenIdx]
			password = credString[colenIdx+1:]
		}
		return &gitstore.RepoRef{URL: url, User: username, Pass: password}, nil
	default:
		return nil, fmt.Errorf("Unable to create repo ref: invalid type \"%s\"", creds.credentialType)
	}
}
