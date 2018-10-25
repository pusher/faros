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
	"time"

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

	// TimeToDeploy is a prometheus histogram that holds the time between a new
	// commit being added to the head of the git tree and the changes being
	// reflected within the GitTrackObjects
	TimeToDeploy = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "faros_gittrack_time_to_deploy_seconds",
		Help: "Counts the time from commit to deploy of a child resource",
		Buckets: []float64{
			// Divide histogram into sensible buckets
			1 * time.Minute.Seconds(), // Per minute up to 5 minutes
			2 * time.Minute.Seconds(),
			3 * time.Minute.Seconds(),
			4 * time.Minute.Seconds(),
			5 * time.Minute.Seconds(), // Per 5 minutes up to 30 minutes
			10 * time.Minute.Seconds(),
			15 * time.Minute.Seconds(),
			20 * time.Minute.Seconds(),
			25 * time.Minute.Seconds(),
			30 * time.Minute.Seconds(), // Per 10 minutes up to an hour
			40 * time.Minute.Seconds(),
			50 * time.Minute.Seconds(),
			1 * time.Hour.Seconds(), // +Inf after an hour
		},
	}, []string{"name", "namespace", "repository"})
)

func init() {
	ctrlmetrics.Registry.MustRegister(ChildStatus)
	ctrlmetrics.Registry.MustRegister(TimeToDeploy)
}
