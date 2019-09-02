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
	"context"
	"fmt"
	"strings"

	farosv1alpha1 "github.com/pusher/faros/pkg/apis/faros/v1alpha1"
	gitstore "github.com/pusher/git-store"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

type gitCredentials struct {
	secret         []byte
	credentialType farosv1alpha1.GitCredentialType
}

// fetchGitCredentials creates git credentials data from a given deployKey secret reference
func (r *ReconcileGitTrack) fetchGitCredentials(gt farosv1alpha1.GitTrackInterface) (*gitCredentials, error) {
	deployKey := gt.GetSpec().DeployKey

	// Check if the deployKey is empty, do nothing if it is
	emptyKey := farosv1alpha1.GitTrackDeployKey{}
	if deployKey == emptyKey {
		return nil, nil
	}
	// Check the deployKey fields are both non-empty
	if deployKey.SecretName == "" || deployKey.Key == "" {
		return nil, fmt.Errorf("if using a deploy key, both SecretName and Key must be set")
	}

	// Set the SecretNamespace to match the GitTrack Namespace
	// Users must set this for ClusterGitTracks
	switch gt.(type) {
	case *farosv1alpha1.GitTrack:
		if deployKey.SecretNamespace != "" && deployKey.SecretNamespace != gt.GetNamespace() {
			return nil, fmt.Errorf("DeployKey namespace must match GitTrack namespace or be empty")
		}
		deployKey.SecretNamespace = gt.GetNamespace()
	case *farosv1alpha1.ClusterGitTrack:
		if deployKey.SecretNamespace == "" {
			return nil, fmt.Errorf("No Secret Namespace set for DeployKey")
		}
	default:
		panic(fmt.Errorf("This code should not be reachable"))
	}

	// Fetch the secret from the API
	secret := &corev1.Secret{}
	err := r.Get(context.TODO(), types.NamespacedName{
		Namespace: deployKey.SecretNamespace,
		Name:      deployKey.SecretName,
	}, secret)
	if err != nil {
		return nil, fmt.Errorf("failed to look up secret %s: %v", deployKey.SecretName, err)
	}

	// Extract the data from the secret
	var ok bool
	var secretData []byte
	if secretData, ok = secret.Data[deployKey.Key]; !ok {
		return nil, fmt.Errorf("invalid deploy key reference. Secret %s does not have key %s", deployKey.SecretName, deployKey.Key)
	}

	return &gitCredentials{secret: secretData, credentialType: deployKey.Type}, nil
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
