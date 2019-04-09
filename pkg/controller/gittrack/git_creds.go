/*
Copyright 2018 Pusher Ltd.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package gittrack

import (
	"fmt"
	"strings"

	farosv1alpha1 "github.com/pusher/faros/pkg/apis/faros/v1alpha1"
	gitstore "github.com/slentzen-auth0/git-store"
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
		credStringSplit := strings.SplitN(string(creds.secret), ":", 2)
		if len(credStringSplit) == 2 {
			return &gitstore.RepoRef{URL: url, User: credStringSplit[0], Pass: credStringSplit[1]}, nil
		}
		return nil, fmt.Errorf("You must specify the secret as <username>:<password> for credential type %s", creds.credentialType)
	default:
		return nil, fmt.Errorf("Unable to create repo ref: invalid type \"%s\"", creds.credentialType)
	}
}
