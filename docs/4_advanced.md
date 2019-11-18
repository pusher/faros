---
layout: default
title: Advanced concepts
permalink: /advanced
nav_order: 4
---

## Owner References and Garbage Collection

Faros uses Kubernetes `OwnerReferences` to leverage built-in
[Garbage Collection](https://kubernetes.io/docs/concepts/workloads/controllers/garbage-collection/).

When Faros manages a resource, it will create a `GitTrackObject`
/`ClusterGitTrackObject` CRD owned by the source `GitTrack` and the `GTO`/`CGTO`
will in turn own the managed resource.

By default, when deleting a `GitTrack`, the Garbage Collector will in turn
delete all dependents (`GTOs`/`CGTOs`) and then delete the dependent's dependents
(managed resources). Therefore, by deleting a `GitTrack`, every resource it was
managing will be deleted.

If you wish to leave objects in place when deleting a `GitTrack`, you can use
the `--cascade=false` flag with `kubectl`.

```
kubectl delete gittrack --cascade=false <gittrack-name>
```

If you wish to remove Faros entirely, we recommend deleting all `GitTrack` and
then `GitTrackObject` and `ClusterGitTrackObject` resources using the
`--cascade=false` flag.

## Three Way Merge

Faros uses a three-way merging strategy to determine the patch to apply when
updating resources. In a similar way to `kubectl apply`, Faros applies a
`last-applied` annotation to resources that it manages.

It uses this annotation, along with the current specification of the Resource
and the existing Resource retrieved from the Kubernetes API to calculate the
desired change to any Resource.

This strategy means that Faros will not interfere with any changes made by
other controllers running on the cluster or changes made outside of Git that
do not conflict with the desired state of the Resource in Git.

For example, when using a Horizontal Pod Autoscaler (HPA) with a Deployment,
if the Deployment explicitly sets the number of desired replicas
(`.spec.replicas`), each time the HPA scales the Deployment, Faros would undo
this action replacing it with the value in the Deployment's spec from Git.
However, if the field is unspecified in the Deployment's spec in Git, Faros
would ignore the update since it does not cause a clash with the defined
desired state.

## Update Strategies

Some Kubernetes resources have fields that are immutable, for example the
spec of a Job is immutable once created. If you were ever to update the spec
of a Resource like this, Faros would encounter an error when attempting to apply
the updated Resource.

To change the update behaviour of individual Resources managed by Faros you can
set an annotation on the individual Resource within your Git repository and have
Faros read this before it attempts to apply the Resource.

Add the annotation `faros.pusher.com/update-strategy` to your resource with one
of the following values:

- `update`: This is the default value. Faros will update the Resource in place.
  This is equivalent to a `kubectl apply`.
- `never`: Faros will ensure that the Resource exists within Kubernetes but will
  never update it if the Resource is modified.
- `recreate`: Faros will first attempt to patch the resource, if this fails it
  will delete the existing Resource and create a new copy.
  This is equivalent to a `kubectl apply --force`.

For example:

```
apiVersion: batch/v1
kind: Job
metadata:
  annotations:
    faros.pusher.com/update-strategy: never
...
```

This annotation is designed to be used in special cases where individual
Resources need special handling by Faros. If you wish to ignore a particular
type of Resource altogether (eg. ignoring all Jobs), see
[Ignore Resource types](#ignore-resource-types).
