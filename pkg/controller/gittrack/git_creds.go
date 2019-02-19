package gittrack

import (
	"fmt"

	farosv1alpha1 "github.com/pusher/faros/pkg/apis/faros/v1alpha1"
	gitstore "github.com/pusher/git-store"
)

type gitCredentials struct {
	secret     []byte
	deployType farosv1alpha1.GitTrackDeployKeyType
}

// createRepoRef creates a git repo ref configured depending on the credentialType
func createRepoRefFromCreds(url string, creds *gitCredentials) (*gitstore.RepoRef, error) {
	if creds == nil {
		creds = &gitCredentials{}
	}
	switch creds.deployType {
	// default to SSH
	case "":
		fallthrough
	case farosv1alpha1.DeployKeyTypeSSH:
		return &gitstore.RepoRef{URL: url, PrivateKey: creds.secret}, nil
	case farosv1alpha1.DeployKeyTypeOAuthToken:
		return &gitstore.RepoRef{URL: url, User: "x-oauth-token", Pass: string(creds.secret)}, nil
	default:
		return nil, fmt.Errorf("Unable to create repo ref: invalid type \"%s\"", creds.deployType)
	}
}
