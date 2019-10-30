---
layout: default
title: Home
permalink: /
nav_order: 0
---

# Faros

Faros is a GitOps controller that takes a Git repository reference from a Custom Resource Definition (CRD) and applies resources within the repository to a Kubernetes cluster.

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

Faros then watches the child resources (resources created from the repository)
and, if they are ever modified, reverts the change back to the state of the Git
repository.
This allows users to make changes to their deployment configuration exclusively
in Git, which in turn enables them to audit and peer review those changes as
well as providing a canonical history of what was deployed and when.


Note: This is a proof of concept in the early alpha stage. We are providing no guarantees and recommend that you test and thoroughly understand the project before deploying to production.
