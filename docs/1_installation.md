---
layout: default
title: Installation
permalink: /installation
nav_order: 1
---

## Building

To build faros locally, run

```
./configure
make build
```

In order to build all binaries for all supported architectures, you may

```
./configure
make release
```

## Deploying to Kubernetes

Faros is a [Kubebuilder](https://github.com/kubernetes-sigs/kubebuilder) based
project, as such we have auto-generated [CRDs](config/crds) and
[Kustomize](https://github.com/kubernetes-sigs/kustomize) configuration as
examples of how to install the controller in the [config](config) folder.
To quickly install the controller and CRDs on a cluster you can run `make deploy`.

You **must** manually install the [CRDs](config/crds) before the controller will
be fully functional, it will not install them for you.

A public docker image is available on [Quay](https://quay.io/repository/pusher/faros).

[![Docker Repository on Quay](https://quay.io/repository/pusher/faros/status "Docker Repository on Quay")](https://quay.io/repository/pusher/faros)

```
quay.io/pusher/faros
```

### RBAC

If you are using [RBAC](https://kubernetes.io/docs/reference/access-authn-authz/rbac/)
within your cluster, you must grant the service account used by your Faros
instance a superset of all roles you expect it to deploy.

The simplest way to do this is to grant Faros `cluster-admin`, however, if you
wish to be more secure, you can concatenate all `rules` from each Role and
ClusterRole that Faros will manage.

If you do not do so, you will see errors where Faros is attempting to
escalate its privileges.
