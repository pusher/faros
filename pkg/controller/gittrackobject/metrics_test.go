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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	farosv1alpha1 "github.com/pusher/faros/pkg/apis/faros/v1alpha1"
	"github.com/pusher/faros/pkg/controller/gittrackobject/metrics"
	farosflags "github.com/pusher/faros/pkg/flags"
	testutils "github.com/pusher/faros/test/utils"
	"k8s.io/client-go/util/flowcontrol"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var _ = Describe("Metrics Suite", func() {
	var r *ReconcileGitTrackObject
	var mgr manager.Manager

	BeforeEach(func() {
		// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
		// channel when it is finished.
		var err error
		cfg.RateLimiter = flowcontrol.NewFakeAlwaysRateLimiter()
		mgr, err = manager.New(cfg, manager.Options{
			Namespace:          farosflags.Namespace,
			MetricsBindAddress: "0", // Disable serving metrics while testing
		})
		Expect(err).NotTo(HaveOccurred())

		recFn := newReconciler(mgr)
		r = recFn.(*ReconcileGitTrackObject)

		// Reset all metrics before each test
		metrics.InSync.Reset()
	})

	Context("updateMetrics", func() {
		var opts *metricsOpts

		BeforeEach(func() {
			opts = &metricsOpts{}
		})

		Context("with a GitTrackObject", func() {
			var gto *farosv1alpha1.GitTrackObject

			BeforeEach(func() {
				gto = testutils.ExampleGitTrackObject.DeepCopy()

				r = r.withValues(
					"Namespace", gto.GetNamespace(),
					"ChildName", gto.GetSpec().Name,
					"ChildKind", gto.GetSpec().Kind,
				)
			})

			Context("with inSync true", func() {
				BeforeEach(func() {
					opts.inSync = true
					Expect(r.updateMetrics(gto, opts)).To(Succeed())
				})

				It("sets the in-sync metric to 1.0", func() {
					gauge, err := GetGauge(metrics.InSync, gto)
					Expect(err).NotTo(HaveOccurred())
					Expect(gauge.GetValue()).To(Equal(1.0))
				})
			})

			Context("with inSync false", func() {
				BeforeEach(func() {
					opts.inSync = false
					Expect(r.updateMetrics(gto, opts)).To(Succeed())
				})

				It("sets the in-sync metric to 0.0", func() {
					gauge, err := GetGauge(metrics.InSync, gto)
					Expect(err).NotTo(HaveOccurred())
					Expect(gauge.GetValue()).To(Equal(0.0))
				})
			})
		})

		Context("with a ClusterGitTrackObject", func() {
			var gto *farosv1alpha1.ClusterGitTrackObject

			BeforeEach(func() {
				gto = testutils.ExampleClusterGitTrackObject.DeepCopy()

				r = r.withValues(
					"Namespace", gto.GetNamespace(),
					"ChildName", gto.GetSpec().Name,
					"ChildKind", gto.GetSpec().Kind,
				)
			})

			Context("with inSync true", func() {
				BeforeEach(func() {
					opts.inSync = true
					Expect(r.updateMetrics(gto, opts)).To(Succeed())
				})

				It("sets the in-sync metric to 1.0", func() {
					gauge, err := GetGauge(metrics.InSync, gto)
					Expect(err).NotTo(HaveOccurred())
					Expect(gauge.GetValue()).To(Equal(1.0))
				})
			})

			Context("with inSync false", func() {
				BeforeEach(func() {
					opts.inSync = false
					Expect(r.updateMetrics(gto, opts)).To(Succeed())
				})

				It("sets the in-sync metric to 0.0", func() {
					gauge, err := GetGauge(metrics.InSync, gto)
					Expect(err).NotTo(HaveOccurred())
					Expect(gauge.GetValue()).To(Equal(0.0))
				})
			})
		})
	})
})

func GetGauge(gv *prometheus.GaugeVec, obj farosv1alpha1.GitTrackObjectInterface) (*dto.Gauge, error) {
	gauge, err := gv.GetMetricWith(map[string]string{
		"kind":      obj.GetSpec().Kind,
		"name":      obj.GetSpec().Name,
		"namespace": obj.GetNamespace(),
	})
	if err != nil {
		return nil, err
	}

	var metric dto.Metric
	err = gauge.Write(&metric)
	if err != nil {
		return nil, err
	}

	return metric.GetGauge(), nil
}
