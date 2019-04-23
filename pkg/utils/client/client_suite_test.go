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

package client

import (
	"log"
	"os"
	"strconv"
	"sync"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pusher/faros/test/reporters"
	"k8s.io/client-go/rest"
	"k8s.io/klog/klogr"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var cfg *rest.Config
var skipDryRun bool

func TestMain(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Client Suite", reporters.Reporters())
}

var t *envtest.Environment

var _ = BeforeSuite(func() {
	t = &envtest.Environment{}

	logf.SetLogger(klogr.New())

	var err error
	if cfg, err = t.Start(); err != nil {
		log.Fatal(err)
	}
	if skipDryRunEnv := os.Getenv("SKIP_DRY_RUN_TESTS"); skipDryRunEnv != "" {
		skipDryRun, err = strconv.ParseBool(skipDryRunEnv)
		Expect(err).NotTo(HaveOccurred())
	}
})

var _ = AfterSuite(func() {
	t.Stop()
})

// SetupTestReconcile returns a reconcile.Reconcile implementation that delegates to inner and
// writes the request to requests after Reconcile is finished.
func SetupTestReconcile(inner reconcile.Reconciler) (reconcile.Reconciler, chan reconcile.Request) {
	requests := make(chan reconcile.Request)
	fn := reconcile.Func(func(req reconcile.Request) (reconcile.Result, error) {
		result, err := inner.Reconcile(req)
		requests <- req
		return result, err
	})
	return fn, requests
}

// StartTestManager adds recFn
func StartTestManager(mgr manager.Manager) (chan struct{}, *sync.WaitGroup) {
	stop := make(chan struct{})
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer GinkgoRecover()
		Expect(mgr.Start(stop)).NotTo(HaveOccurred())
	}()
	return stop, wg
}
