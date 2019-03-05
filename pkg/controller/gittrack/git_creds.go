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
