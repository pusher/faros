---
layout: default
title: Quick Start
permalink: /quickstart
nav_order: 2
---
If you haven't yet got Faros running on your cluster, see
[Installation](installation) for details on how to get it running.

If you would like to deploy a private Git repository, we recommend doing so
using SSH.
To allow Faros access to the repository, place an authorised private key within
a `Secret`.

Create a `ClusterGitTrack` resource based the example below:

```yaml
apiVersion: faros.pusher.com/v1alpha1
kind: ClusterGitTrack
metadata:
  name: foo
spec:
  # Repository accepts any valid Git repository reference, the most common formats
  # are:
  #   https://<server>/<organisation>/<repository>
  #   <user>@<server>:<organisation>/<repository>
  repository: git@github.com:foo-org/k8s-manifests
  # Reference accepts any valid Git reference, this could be a branch name, tag
  # or commit SHA, eg:
  #   master or refs/remotes/origin/master
  #   v1.0.0 or refs/tags/v1.0.0
  #   ec32c240b7f9b440aa727c9d931751fdd0c40b49
  reference: master
  # (Optional) SubPath expects a path to a folder within the repository.
  # Note: Faros loads all .yml/.yaml/.json files recursively within the path.
  subPath: deployments/kube-system
  # (Optional) DeployKey allows you to specify credentials for repository access
  # over SSH or HTTP Basic Auth
  deployKey:
    # SecretName is the name of the secret containing the secret
    secretName: foo-k8s-manifests
    # SecretNamespace is the namespace to look for the secret in
    secretName: default
    # Key is the Secret's key containing the secret
    key: id_rsa
    # (Optional) Type is the type of credential. Accepted values are "SSH", "HTTPBasicAuth". Defaults to "SSH"
    # When set to "HTTPBasicAuth" the expected secret format is "<username>:<password>".
    type: SSH | HTTPBasicAuth
```

Deploy the `ClusterGitTrack` to your cluster and watch its status as Faros processes
it. Eventually all conditions should have status `True` and the `objectsApplied`
and `objectsInSync` fields should be equal.

```yaml
status:
  conditions:
    - lastTransitionTime: 2018-10-16T17:36:21Z
      lastUpdateTime: 2018-10-16T17:36:21Z
      reason: FileParseSuccess
      status: "True"
      type: FilesParsed
    - lastTransitionTime: 2018-10-16T17:36:21Z
      lastUpdateTime: 2018-10-16T17:36:21Z
      reason: GitFetchSuccess
      status: "True"
      type: FilesFetched
    - lastTransitionTime: 2018-10-16T17:36:21Z
      lastUpdateTime: 2018-10-16T17:36:21Z
      reason: GCSuccess
      status: "True"
      type: ChildrenGarbageCollected
    - lastTransitionTime: 2018-10-16T17:36:21Z
      lastUpdateTime: 2018-10-16T17:36:21Z
      reason: ChildUpdateSuccess
      status: "True"
      type: ChildrenUpToDate
  objectsApplied: 82
  objectsDiscovered: 83
  objectsIgnored: 1
  objectsInSync: 82
```
