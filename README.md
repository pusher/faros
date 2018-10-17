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
