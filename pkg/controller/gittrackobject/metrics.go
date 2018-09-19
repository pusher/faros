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

package gittrackobject

import (
	"fmt"

	farosv1alpha1 "github.com/pusher/faros/pkg/apis/faros/v1alpha1"
	"github.com/pusher/faros/pkg/controller/gittrackobject/metrics"
)

type metricsOpts struct {
	inSync bool
}

func newMetricOpts() *metricsOpts {
	return &metricsOpts{}
}

func (r *ReconcileGitTrackObject) updateMetrics(gto farosv1alpha1.GitTrackObjectInterface, opts *metricsOpts) error {
	if gto == nil {
		return nil
	}
	labels := map[string]string{
		"kind":      gto.GetSpec().Kind,
		"name":      gto.GetSpec().Name,
		"namespace": gto.GetNamespace(),
	}
	inSync, err := metrics.InSync.GetMetricWith(labels)
	if err != nil {
		return fmt.Errorf("unable to update in-sync metric: %v", err)
	}
	if opts.inSync {
		inSync.Set(1.0)
	} else {
		inSync.Set(0.0)
	}
	return nil
}
