<img src="./faros-logo.svg" width=150 height=150 alt="Faros Logo"/>

# Faros

> Faros - Greek for Lighthouse

Faros is a GitOps controller that takes a Git repository reference from a
Custom Resource Definition (CRD) and applies resources within the repository to
a Kubernetes cluster.

Note: This is a proof of concept in the early alpha stage.
We are providing no guarantees and recommend that you test and thoroughly
understand the project before deploying to production.

## Table of Contents

- [Introduction](#introduction)
- [Installation](#installation)
  - [Deploying to Kubernetes](#deploying-to-kubernetes)
  - [Configuration](#configuration)
    - [Ignore Resource types](#ignore-resource-types)
    - [Namespace restriction](#namespace-restriction)
    - [Leader Election](#leader-election)
    - [Sync period](#sync-period)
- [Quick Start](#quick-start)
- [Project Concepts](#project-concepts)
- [Communication](#communication)
- [Contributing](#contributing)
- [License](#license)

## Introduction

Faros aims to make it easier for teams to ensure that the desired state of their
applications is synchronised between a Kubernetes cluster and Git.

Typically, a team running workloads on Kubernetes will use
infrastructure-as-code concepts and keep a copy of their deployment
configuration under source control just as they do with their product code.
The process of taking this desired state from Git and applying it to the
Kubernetes cluster is the problem Faros aims to solve.

By providing Faros with a reference to a Git repository (URL and Git Reference
(eg master)), credentials to access the repository and an optional path within
the repository, Faros will load all Kubernetes resource definitions from the
repository and synchronise these with the Kubernetes cluster.

Faros then watches the child resources and, if they are ever modified,
immediately reverts the change back to the state of the Git repository.
This forces users to make changes to their deployment configuration in Git,
which in turn allows for auditing, peer review and a canonical history of what
was deployed and when.

## Installation

### Deploying to Kubernetes

Faros is a [Kubebuilder](https://github.com/kubernetes-sigs/kubebuilder) based
project, as such we have auto-generated [CRDs](config/crds) and
[Kustomize](https://github.com/kubernetes-sigs/kustomize) configuration as
examples of how to install the controller in the [config](config) folder.

You **must** manually install the [CRDs](config/crds) before the controller will
be fully functional, it will not install them for you.

#### RBAC

If you are using [RBAC](https://kubernetes.io/docs/reference/access-authn-authz/rbac/)
within your cluster, you must grant the service account used by your Faros
instance a superset of all roles you expect it to deploy.

The simplest way to do this is to grant Faros `cluster-admin`, however, if you
wish to be more secure, you can concatenate all `rules` from each Role and
ClusterRole that Faros will manage.

If you do not do so, you will see errors where Faros is attempting to
escalate its privileges.

### Configuration

The following details the various configuration options that Faros provides
at the controller level.

#### Ignore Resource types

You may not want to have Faros manage all types of Kubernetes resource.
The `--ignore-resource` flag allows you to specify a particular Resource to
ignore.

Specify Resources to ignore in the format `<resource>.<api-group>/<api-version>`,
for example to ignore all Kubernetes Jobs specify the following flag:

```
--ignore-resource=jobs.batch/v1
```

It is recommended not to manage `GitTrack` resources using Faros itself,
so ignore those using the following flag:

```
--ignore-resource=gittracks.faros.pusher.com/v1alpha1
```

#### Namespace restriction

Faros can be run either as a cluster wide controller or per namespace.

To restrict Faros to watch GitTrack resources only in a single namespace,
use the following flag:

```
--namespace=<namespace>
```

This will restrict Faros to the namespace you provide.
At this point, it will only read GitTrack resources within the defined namespace.
If any GitTrack refers to any resource not in the GitTrack's namespace, the
resource will be ignored.
Therefore a GitTrack in the `default` namespace cannot manage a resource in the
`kube-system` namespace if the controller is restricted to the `default`
namespace.

When restricted to a namespace, Faros **will** still manage any non-namespaced
Kubernetes resource it finds referenced within its GitTracks. If however, the
non-namespaced resource clashes and is defined in another GitTrack within
another namespace, Faros will ignore the resource. First owner wins.

#### Leader Election

Faros can be run in an active-standby HA configuration using Kubernetes leader
election.
When leader election is enabled, each Pod will attempt to become leader and,
whichever is successful, will become the active or master controller.
The master will perform all of the reconciliation of Resources.

The standby Pods will sit and wait until the leader goes away and then one
standby will be promoted to master.

To enable leader election, set the following flags:

```
--leader-elect=true
--leader-election-id=<name-of-leader-election-configmap>
--leader-election-namespace=<namespace-controller-runs-in>
```

#### Sync period

The controller uses Kubernetes informers to cache resources and reduce load on
the Kubernetes API server. Every informer has a sync period, after which it will
refresh all resources within its cache. At this point, every item in the cache
is queued for reconciliation by the controller.

Therefore, by setting the following flag;

```
--sync-period=5m
```

You can ensure that every resource will be reconciled at least every 5 minutes.

## Quick Start

## Project Concepts

## Communication

- Found a bug? Please open an issue.
- Have a feature request. Please open an issue.
- If you want to contribute, please submit a pull request

## Contributing

Please see our [Contributing](CONTRIBUTING.md) guidelines.

## License

This project is licensed under Apache 2.0 and a copy of the license is available [here](LICENSE).
