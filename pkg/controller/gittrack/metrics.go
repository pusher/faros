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

package gittrack

import (
	"fmt"
	"time"

	farosv1alpha1 "github.com/pusher/faros/pkg/apis/faros/v1alpha1"
	"github.com/pusher/faros/pkg/controller/gittrack/metrics"
)

type metricsOpts struct {
	status       *statusOpts
	timeToDeploy []time.Duration
	repository   string
}

func newMetricOpts(status *statusOpts) *metricsOpts {
	return &metricsOpts{
		status:       status,
		timeToDeploy: []time.Duration{},
	}
}

func (r *ReconcileGitTrack) updateMetrics(gt farosv1alpha1.GitTrackInterface, opts *metricsOpts) error {
	if gt == nil {
		return nil
	}
	err := updateChildStatusMetric(gt.GetName(), gt.GetNamespace(),
		map[string]int64{
			"discovered": opts.status.discovered,
			"applied":    opts.status.applied,
			"ignored":    opts.status.ignored,
			"inSync":     opts.status.inSync,
		},
	)
	if err != nil {
		return fmt.Errorf("error updating Child Status metric: %v", err)
	}

	err = updateTimeToDeployMetric(gt.GetName(), gt.GetNamespace(), opts.repository, opts.timeToDeploy)
	if err != nil {
		return fmt.Errorf("error updating Time To Deploy metric: %v", err)
	}
	return nil
}

func updateChildStatusMetric(gtName, gtNamespace string, values map[string]int64) error {
	for status, value := range values {
		labels := map[string]string{
			"name":      gtName,
			"namespace": gtNamespace,
			"status":    status,
		}
		metric, err := metrics.ChildStatus.GetMetricWith(labels)
		if err != nil {
			return fmt.Errorf("unable to get metric with labels %+v: %v", labels, err)
		}
		metric.Set(float64(value))
	}
	return nil
}

func updateTimeToDeployMetric(gtName, gtNamespace, repository string, durations []time.Duration) error {
	labels := map[string]string{
		"name":       gtName,
		"namespace":  gtNamespace,
		"repository": repository,
	}
	metric, err := metrics.TimeToDeploy.GetMetricWith(labels)
	if err != nil {
		return fmt.Errorf("unable to get metric with labels %+v: %v", labels, err)
	}
	for _, duration := range durations {
		if duration != 0 {
			metric.Observe(duration.Seconds())
		}
	}

	return nil
}
