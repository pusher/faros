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
	// ChildStatus is a prometheus gauge that details the state of child objects
	// of the GitTrack
	ChildStatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "faros_gittrack_child_status",
		Help: "Shows the status of a GitTracks child objects",
	}, []string{"name", "namespace", "status"})
)

func init() {
	ctrlmetrics.Registry.MustRegister(ChildStatus)
}
