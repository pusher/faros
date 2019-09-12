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
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/pusher/faros/pkg/apis"
	"github.com/pusher/faros/test/reporters"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	"k8s.io/klog/klogr"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logr "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var cfg *rest.Config
var repositoryPath string
var repositoryURL string
var fixturesRepoPath, _ = filepath.Abs("./fixtures/repo.tgz")

func setupRepository() string {
	dir, err := ioutil.TempDir("", "gittrack")
	if err != nil {
		log.Fatal(err)
	}

	cmd := exec.Command("tar", "-zxf", fixturesRepoPath, "-C", dir, "--strip-components", "1")
	err = cmd.Run()
	if err != nil {
		log.Fatal(err)
	}

	return dir
}

func teardownRepository(dir string) {
	os.RemoveAll(dir)
}

func TestMain(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "GitTrack Suite", reporters.Reporters())
}

var t *envtest.Environment

var _ = BeforeSuite(func() {
	logr.SetLogger(klogr.New())
	logFlags := &flag.FlagSet{}
	klog.InitFlags(logFlags)
	// Set log level high for tests
	logFlags.Lookup("v").Value.Set("4")

	t = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "..", "config", "crds")},
	}
	apis.AddToScheme(scheme.Scheme)

	repositoryPath = setupRepository()
	repositoryURL = fmt.Sprintf("file://%s", repositoryPath)

	var err error
	if cfg, err = t.Start(); err != nil {
		log.Fatal(err)
	}
})

var _ = BeforeEach(func() {
	// Reset metrics registry before each test
	metrics.Registry = prometheus.NewRegistry()
})

var _ = AfterSuite(func() {
	t.Stop()
	teardownRepository(repositoryPath)
})

// SetupTestReconcile returns a reconcile.Reconcile implementation that delegates to inner and
// writes the request to requests after Reconcile is finished.
func SetupTestReconcile(inner reconcile.Reconciler) (reconcile.Reconciler, chan reconcile.Request) {
	requests := make(chan reconcile.Request)
	fn := reconcile.Func(func(req reconcile.Request) (reconcile.Result, error) {
		result, err := inner.Reconcile(req)
		if err != nil {
			log.Printf("error during reconcile: %v\n", err)
		}
		requests <- req
		return result, err
	})
	return fn, requests
}

// StartTestManager adds recFn
func StartTestManager(mgr manager.Manager) chan struct{} {
	stop := make(chan struct{})
	go func() {
		defer GinkgoRecover()
		Expect(mgr.Start(stop)).NotTo(HaveOccurred())
	}()
	return stop
}

var _ reconcile.Reconciler = &fakeReconciler{}

// fakeReconciler is a no-op reconciler to be used for tests of the watch setup
type fakeReconciler struct{}

func (f *fakeReconciler) Reconcile(_ reconcile.Request) (reconcile.Result, error) {
	return reconcile.Result{}, nil
}
