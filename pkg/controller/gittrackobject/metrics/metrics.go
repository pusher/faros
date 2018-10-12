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

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// InSync is a prometheus gauge for whether (Cluster)GitTrackObjects are in
	// sync with their child objects or not
	//
	// Value should be 0 if not in sync and 1 if in sync
	InSync = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "faros_gittrackobject_in_sync",
		Help: "Shows whether a (Cluster)GitTrackObject is In Sync (boolean)",
	}, []string{"kind", "name", "namespace"})
)

func init() {
	ctrlmetrics.Registry.MustRegister(InSync)
}
