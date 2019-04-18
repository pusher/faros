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

package main

import (
	"fmt"
	"log"
	"runtime"
	"time"

	goflag "flag"

	"github.com/pusher/faros/pkg/apis"
	"github.com/pusher/faros/pkg/controller"
	farosflags "github.com/pusher/faros/pkg/flags"
	"github.com/pusher/faros/pkg/utils"
	flag "github.com/spf13/pflag"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
)

var (
	leaderElection           = flag.Bool("leader-election", false, "Should the controller use leader election")
	leaderElectionID         = flag.String("leader-election-id", "", "Name of the configmap used by the leader election system")
	leaederElectionNamespace = flag.String("leader-election-namespace", "", "Namespace for the configmap used by the leader election system")
	metricsBindAddress       = flag.String("metrics-bind-address", ":8080", "Specify which address to bind to for serving prometheus metrics")
	syncPeriod               = flag.Duration("sync-period", 5*time.Minute, "Reconcile sync period")
	showVersion              = flag.Bool("version", false, "Show version and exit")
)

func main() {
	// Setup flags
	goflag.Lookup("logtostderr").Value.Set("true")
	flag.CommandLine.AddFlagSet(farosflags.FlagSet)
	flag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	flag.Parse()

	// Handle version flag
	if *showVersion {
		fmt.Printf("faros-gittrack-controller %s (built with %s)\n", VERSION, runtime.Version())
		return
	}

	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		log.Fatal(err)
	}

	// Create a new Cmd to provide shared dependencies and start components
	mgr, err := manager.New(cfg, manager.Options{
		LeaderElection:          *leaderElection,
		LeaderElectionID:        *leaderElectionID,
		LeaderElectionNamespace: *leaederElectionNamespace,
		MetricsBindAddress:      *metricsBindAddress,
		SyncPeriod:              syncPeriod,
		Namespace:               farosflags.Namespace,
		MapperProvider:          utils.NewRestMapper,
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Registering Components.")

	// Setup Scheme for all resources
	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		log.Fatal(err)
	}

	// Setup all Controllers
	if err := controller.AddToManager(mgr); err != nil {
		log.Fatal(err)
	}

	log.Printf("Starting the Cmd.")

	// Start the Cmd
	log.Fatal(mgr.Start(signals.SetupSignalHandler()))
}
