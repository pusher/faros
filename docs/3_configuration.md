---
layout: default
title: Configuration
permalink: /configuration
nav_order: 3
---

The following details the various configuration options that Faros provides
at the controller level.

### Ignore Resource types

You may not want to have Faros manage all types of Kubernetes resource.
The `--ignore-resource` flag allows you to specify a particular Resource to
ignore.

Specify Resources to ignore in the format `<resource>.<api-group>/<api-version>`,
for example to ignore all Kubernetes Jobs specify the following flag:

```
--ignore-resource=jobs.batch/v1
```

It is recommended not to manage `GitTrack` or `ClusterGitTrack` resources using Faros itself,
so ignore those using the following flags:

```
--ignore-resource=gittracks.faros.pusher.com/v1alpha1
--ignore-resource=clustergittracks.faros.pusher.com/v1alpha1
```

### GitTracks and ClusterGitTracks

References to git repositories are known as GitTracks. Faros has 2
different kinds of GitTracks, regular and ClusterGitTrack. Regular
GitTracks exist in a namespace and can only handle resources in that
namespace. ClusterGitTracks live in the cluster scope and can handle
all resources.

This somewhat cumbersome distinction exists because Kubernetes doesn't
allow cross-namespace ownership. For a simple setup, it's recommended
that you use a single ClusterGitTrack to handle your entire cluster.

A Faros controller can be restricted to only handling GitTracks or ClusterGitTracks
with the `--gittrack-mode` and `--clustergittrack-mode` flags. `--gittrack-mode` takes the following options

* `Disabled`: Don't handle `gittracks`
* `Enabled`: Handle `gittracks`

When a Faros controller is handling `gittracks`, you can restrict it to only
handling `GitTracks` within one namespace with the `--namespace=$namespace`
flag. If the flag is empty, all namespaces are handled.

`--clustergittrack-mode` takes the following options

* `Disabled`: No `ClusterGitTracks` are handled
* `ExcludeNamespaced`: Only cluster-scoped resources within a `ClusterGitTrack` are handled
* `IncludeNamespaced`: all resources are handled by a `ClusterGitTrack`, including namespaced ones.

When `--clustergittrack-mode` is enabled, you cannot restrict it to a given
namespace because of kubernetes internals limitations

### Leader Election

Faros can be run in an active-standby HA configuration using Kubernetes leader
election.
When leader election is enabled, each Pod will attempt to become leader and,
whichever is successful, will become the active or master controller.
The master will perform all of the reconciliation of Resources.

The standby Pods will sit and wait until the leader goes away and then one
standby will be promoted to master.

To enable leader election, set the following flags:

```
--leader-election=true
--leader-election-id=<name-of-leader-election-configmap>
--leader-election-namespace=<namespace-controller-runs-in>
```

### Sync period

The controller uses Kubernetes informers to cache resources and reduce load on
the Kubernetes API server. Every informer has a sync period, after which it will
refresh all resources within its cache. At this point, every item in the cache
is queued for reconciliation by the controller.

Therefore, by setting the following flag;

```
--sync-period=5m // Default value of 5m (5 minutes)
```

You can ensure that every resource will be reconciled at least every 5 minutes.

### Server Dry Run

By default, the GitTrackObject controller will attempt to dry run updates to
resources before actually applying updates. This helps to prevent unnecessary
updates and allows mutating admission controller results to be observed.

Server side dry run currently sits behind a feature gate within Kubernetes,
please see the table below for compatibility.

| Kubernetes Version | Feature Status                           | Compatible |
| ------------------ | ---------------------------------------- | :--------: |
| **1.14**           | **Beta feature gate enabled (default)**  |  **True**  |
| 1.14               | Beta feature gate disabled               |   False    |
| **1.13**           | **Beta feature gate enabled (default)**  |  **True**  |
| 1.13               | Beta feature gate disabled               |   False    |
| 1.12               | Alpha feature gate enabled               |    True    |
| **1.12**           | **Beta feature gate disabled (default)** | **False**  |
| **1.11**           | **Not Implemented**                      | **False**  |

If your version of Kubernets is incompatible, please disable server dry run as
below:

```
--server-dry-run=false // Defaults to true
```

### Metrics

The controller exposes a number of metrics in a prometheus format at a
`/metrics` endpoint.
By default, the metrics serving are bound to the address port `8080` on all
interfaces.

Change this with the following flag:

```
--metrics-bind-address=127.0.0.1:8080 // Bind the metrics to localhost only
```

Serving metrics can be disabled by setting the flag to `0`:

```
--metrics-bind-address=0 // Disable serving all metrics
```

#### Available Metrics

- `faros_gittrack_child_status` - Exposes the count of GitTrack child objects
  by status (applied,discovered,ignored,inSync).
- `faros_gittrack_time_to_deploy_seconds_{bucket, count, sum}` - Measures the
  time from updating a repository to the update being propagated to the child
  object.
- `faros_gittrackobject_in_sync` - Indicates whether individual children are in
  sync with their desired state.

- `controller_runtime_reconcile_errors_total` - Counts the total number of
  errors produced by the controller.
- `controller_runtime_reconcile_queue_length` - Counts how many items are
  currently queued for reconciliation by the controller.
- `controller_runtime_reconcile_time_second_{bucket, count, sum}` - Measures how
  long each reconciliation takes within the controller.
