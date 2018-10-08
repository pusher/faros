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

package flags

import (
	flag "github.com/spf13/pflag"
)

var (
	// FlagSet contains faros flags that are needed in multiple packages
	FlagSet *flag.FlagSet

	// Namespace is the namespace for the controller to be restricted to
	Namespace string
)

func init() {
	FlagSet = flag.NewFlagSet("faros", flag.PanicOnError)
	FlagSet.StringVar(&Namespace, "namespace", "", "Only manage GitTrack resources in given namespace")
}
