---
layout: default
title: Migration to v0.7
permalink: /migrate
nav_order: 5
---

This document is only applicable if you are upgrading from a version before
v0.6.1 to a later version. You can safely ignore this document if this is
not the case for you.


## ClusterGitTrack migration plan

Kubernetes disallows having owner relationships that go from one namespace
into another or from a namespaced object to a cluster-scoped one. Previously,
Faros has been able to create these, and because of limitations in the
Kubernetes garbage collector, it hasn’t been immediately obvious that this
shouldn’t have worked.

This document sets out how to check whether you might be vulnerable to this
issue and how to mitigate it when upgrading to Faros version 0.7.0

## Checking if you’re impacted

If you have any ClusterGitTrackObjects in your cluster, you are impacted. You
can check this by running `kubectl get clustergittrackobjects`

If you don’t have any ClusterGitTrackObjects,
you might still be impacted. Run the tool
[here](https://github.com/pusher/faros/blob/master/hack/namespacecheck/namespacechecker.go)
to check if you have any GitTrackObjects owned by GitTracks in a different
namespace

If neither of these are applicable to your setup, you are not impacted. The
only change required is to add the `--gittrack-mode=Enabled` flag to your
Faros deployment when upgrading to version 0.7.0 or greater.

The rest of this document sets out how to migrate in the case that you
are impacted.

## Migrating ClusterGitTrackObjects

If you have ClusterGitTrackObjects in your setup, then you will have to
migrate those to being managed by ClusterGitTracks

1. Scale down your Faros deployments so that there are no active Faros pods
 running
2. Remove all `ownerReferences` from ClusterGitTrackObjects
3. For every `GitTrack` which previously owned a `ClusterGitTrackObject`,
create a `ClusterGitTrack` that matches its target.
4. Create a new deployment of Faros with the flags `--gittrack-mode=Disabled`
and `--clustergittrack-mode=ExcludeNamespaced`
5. Check that ClusterGitTrackObjects are now owned by ClusterGitTracks

A Faros deployment must handle ClusterGitTracks. This means you can either
leave the Faros from this process in place, or you can choose one of your
previous Faros deployments to add `--clustergittrack-mode=ExcludeNamespaced`
to.

## Migrating GitTrackObjects

If the tool for checking GitTrackObjects didn’t find any cross-namespace
references, you are good to go, just add the `--gittrack-mode=Enabled`
to all your existing Faros deployments

If you did find objects owned across namespaces, you’ll have to take steps
to make sure that they are owned by a parent within its own namespace

1. Scale down your Faros deployments so that there are no active Faros pods
running
2. Remove all ownerReferences from GitTrackObjects

If you have a setup where all your GitTracks live in one namespace, follow
these steps

1. Create ClusterGitTracks matching each of your current GitTracks
2. Remove the existing GitTracks
3. Start a Faros with `--clustergittrack-mode=IncludeNamespaced` and
`--gittrack-mode=Disabled`

If you have a setup where GitTracks are distributed amongst multiple
namespaces, follow these steps

1. Create a new deployment of Faros with the flags `--gittrack-mode=Enabled`
and `--clustergittrack-mode=Disabled`, with no namespace (meaning it’ll
handle all namespaces)
2. For each GitTrack, inspect them for `status.ignoredFiles` saying `namespace
<NAMESPACE> is not managed by this GitTrack`
3. Move your resources in git and update your GitTracks so that each GitTrack
manages a single namespace that the GitTrack lives in (you can have multiple
GitTrack per namespace, but only one namespace to a GitTrack)
4. Turn off the new deployment of Faros and scale up your old Faros deployments
with `--gittrack-mode=Enabled`

