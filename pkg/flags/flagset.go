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
	"fmt"
	"strings"
	"time"

	flag "github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// FlagSet contains faros flags that are needed in multiple packages
	FlagSet *flag.FlagSet

	// Namespace is the namespace for the controller to be restricted to
	Namespace string

	// ignoredResources is a list of Kubernets kinds to ignore when reconciling
	ignoredResources []string

	// ServerDryRun whether to enable Server side dry run or not
	ServerDryRun bool

	// FetchTimeout in seconds for fetching changes from repositories
	FetchTimeout time.Duration

	// GitTrack specifies if we're handling GitTracks
	GitTrack GitTrackMode

	// ClusterGitTrack specifies which mode we're handling ClusterGitTracks in
	ClusterGitTrack ClusterGitTrackMode

	// RepositoryDir is the folder on the local filesystem in which Faros should
	// check out repositories. If left empty, Faros will check out repositories
	// in memory
	RepositoryDir string
)

func init() {
	FlagSet = flag.NewFlagSet("faros", flag.PanicOnError)
	FlagSet.StringVar(&Namespace, "namespace", "", "Only manage GitTrack resources in given namespace")
	FlagSet.StringSliceVar(&ignoredResources, "ignore-resource", []string{}, "Ignore resources of these kinds found in repositories, specified in <resource>.<group>/<version> format eg jobs.batch/v1")
	FlagSet.BoolVar(&ServerDryRun, "server-dry-run", true, "Enable/Disable server side dry run before updating resources")
	FlagSet.DurationVar(&FetchTimeout, "fetch-timeout", 30*time.Second, "Timeout in seconds for fetching changes from repositories")
	FlagSet.StringVar(&RepositoryDir, "repository-dir", "", "Directory in which to clone repositories. Defaults to cloning in memory if unset.")
	FlagSet.Var(&GitTrack, "gittrack-mode", "Whether to manage GitTracks. Valid values are Disabled and Enabled")
	FlagSet.Var(&ClusterGitTrack, "clustergittrack-mode", "How to manage ClusterGitTracks. Valid values are Disabled, IncludeNamespaced and ExcludeNamespaced")
}

// ParseIgnoredResources attempts to parse the ignore-resource flag value and
// create a set of GroupVersionResources from the slice
func ParseIgnoredResources() (map[schema.GroupVersionResource]interface{}, error) {
	gvrs := make(map[schema.GroupVersionResource]interface{})
	for _, ignored := range ignoredResources {
		if !strings.Contains(ignored, ".") || !strings.Contains(ignored, "/") {
			return nil, fmt.Errorf("%s is invalid, should be of format <resource>.<group>/<version>", ignored)
		}
		split := strings.SplitN(ignored, ".", 2)
		gv, err := schema.ParseGroupVersion(split[1])
		if err != nil {
			return nil, fmt.Errorf("unable to parse group version %s: %v", split[1], err)
		}
		gvr := schema.GroupVersionResource{
			Group:    gv.Group,
			Version:  gv.Version,
			Resource: split[0],
		}
		gvrs[gvr] = nil
	}
	return gvrs, nil
}

// ValidateFlags returns an error if an invalid set of options have been set in the flags.
// It must be called after the flags have been parsed
func ValidateFlags() error {
	if !flag.Parsed() {
		return fmt.Errorf("ValidateFlags called on unparsed flags")
	}

	if GitTrack == GTMEnabled && Namespace != "" && ClusterGitTrack != CGTMDisabled {
		return fmt.Errorf("Cannot use gittrack-mode enabled with namespace %q and clustergittrack-mode enabled", Namespace)
	}
	return nil
}

// ClusterGitTrackMode specifies which mode we're running ClusterGitTracks in
type ClusterGitTrackMode int

// Enums for ClusterGitTrackMode
const (
	CGTMIncludeNamespaced ClusterGitTrackMode = iota
	CGTMDisabled
	CGTMExcludeNamespaced
)

// String implements the flag.Value interface
func (cgtm ClusterGitTrackMode) String() string {
	switch cgtm {
	case CGTMDisabled:
		return "Disabled"
	case CGTMIncludeNamespaced:
		return "IncludeNamespaced"
	case CGTMExcludeNamespaced:
		return "ExcludeNamespaced"
	}
	panic("unreachable")
}

// Set implements the flag.Value interface
func (cgtm *ClusterGitTrackMode) Set(s string) error {
	lowered := strings.ToLower(s)
	switch lowered {
	case "disabled":
		*cgtm = CGTMDisabled
		return nil
	case "includenamespaced":
		*cgtm = CGTMIncludeNamespaced
		return nil
	case "excludenamespaced":
		*cgtm = CGTMExcludeNamespaced
		return nil
	default:
		return fmt.Errorf("invalid value %q for clustergittrack-mode; valid values are Disabled, IncludeNamespaced and ExcludeNamespaced", s)
	}
}

// Type implements the flag.Value interface
func (cgtm ClusterGitTrackMode) Type() string {
	return "ClusterGitTrackMode"
}

// GitTrackMode specifies which mode we're running GitTracks in
type GitTrackMode bool

// Enums for GitTrackMode
const (
	GTMDisabled GitTrackMode = false
	GTMEnabled  GitTrackMode = true
)

// String implements the flag.Value interface
func (gtm GitTrackMode) String() string {
	if gtm {
		return "Enabled"
	}
	return "Disabled"
}

// Set implements the flag.Value interface
func (gtm *GitTrackMode) Set(s string) error {
	lowered := strings.ToLower(s)
	switch lowered {
	case "disabled":
		*gtm = false
		return nil
	case "enabled":
		*gtm = true
		return nil
	default:
		return fmt.Errorf("invalid value %q for gittrack-mode; valid values are Disabled and Enabled", s)
	}
}

// Type implements the flag.Value interface
func (gtm *GitTrackMode) Type() string {
	return "GitTrackMode"
}
