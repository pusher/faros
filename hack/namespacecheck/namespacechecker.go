/*
Copyright 2019 Pusher Ltd.

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

// namespacecheck checks whether a given Faros setup
// is valid within the ownership rules added in v0.7.
//
// To run the tool, grab 3 files off your kubernetes cluster
//
// # kubectl get gittracks -A -o json > gt.json
// # kubectl get gittrackobjects -A -o json > gto.json
// # kubectl get clustergittrackobjects -A -o json > cgto.json
//
// Then run
// ./namespacecheck gt.json gto.json cgto.json
//
// The tool will report back if you have any cross namespace ownership
// and write a file containing any new clustergittracks that will need to
// be created
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	faros "github.com/pusher/faros/pkg/apis/faros/v1alpha1"
)

func main() {
	flag.Parse()
	if flag.NArg() < 2 || flag.NArg() > 3 {
		fmt.Fprintf(os.Stderr, "wrong number of flags, expected 2 or 3; got %d\n", flag.NArg())
		usage()
	}
	gtfile, err := ioutil.ReadFile(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		usage()
	}
	gtofile, err := ioutil.ReadFile(flag.Arg(1))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		usage()
	}
	var gtjson faros.GitTrackList
	err = json.Unmarshal(gtfile, &gtjson)
	if err != nil {
		panic(err)
	}

	var gtojson faros.GitTrackObjectList
	err = json.Unmarshal(gtofile, &gtojson)
	if err != nil {
		panic(err)
	}

	uidToNs := make(map[string]string)
	for _, i := range gtjson.Items {
		uidToNs[string(i.GetUID())] = i.GetNamespace()
	}

	var foundOne bool
	for _, i := range gtojson.Items {
		for _, owner := range i.GetOwnerReferences() {
			ownerNs := uidToNs[string(owner.UID)]
			if i.GetNamespace() != ownerNs {
				foundOne = true
				fmt.Println("gittrackobject", i.GetUID(), "has owner in namespace ", ownerNs)
			}
		}
	}
	if !foundOne {
		fmt.Println("no cross-namespace owner references detected!")
	}

	if flag.NArg() == 3 {
		cgtfile, err := ioutil.ReadFile(flag.Arg(2))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			usage()
		}
		output := makeCGTs(gtjson, cgtfile)
		err = ioutil.WriteFile("apply.json", output, 0644)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		fmt.Println("wrote ClusterGitTrack file to apply.json")
	}
}

func makeCGTs(gtjson faros.GitTrackList, cgtofile []byte) []byte {
	// parse the cgtfile to figure out which GitTracks owns it
	var cgtojson faros.ClusterGitTrackObjectList
	err := json.Unmarshal(cgtofile, &cgtojson)
	if err != nil {
		panic(err)
	}

	sourceUIDSet := make(map[string]bool)
	for _, i := range cgtojson.Items {
		for _, ref := range i.GetOwnerReferences() {
			sourceUIDSet[string(ref.UID)] = true
		}
	}
	var toCreate []faros.GitTrack
	for _, i := range gtjson.Items {
		if sourceUIDSet[string(i.GetUID())] {
			toCreate = append(toCreate, i)
		}
	}

	var cgtjson faros.ClusterGitTrackList
	cgtjson.APIVersion = "v1"
	cgtjson.Kind = "List"
	for _, v := range toCreate {

		cgt := faros.ClusterGitTrack{}
		cgt.TypeMeta = v.TypeMeta
		cgt.Kind = "ClusterGitTrack"
		cgt.SetName(fmt.Sprintf("%s-%s", v.GetNamespace(), v.GetName()))

		spec := v.Spec
		// if the gittrack has a secret for its deploykey, but it uses
		// implicit namespacing rules, then we need to copy the namespace
		// for the clustergittrack which doesn't have this implicit link
		if spec.DeployKey.SecretName != "" && spec.DeployKey.SecretNamespace == "" {
			spec.DeployKey.SecretNamespace = v.GetNamespace()
		}
		cgt.SetSpec(spec)
		cgtjson.Items = append(cgtjson.Items, cgt)
	}
	output, err := json.MarshalIndent(cgtjson, "", "\t")
	if err != nil {
		panic(err)
	}
	return output
}

func usage() {
	fmt.Fprintln(os.Stderr, "\tusage: ./namespacechecker <GitTrack.json> <GitTrackObjects.json> <ClusterGitTrackObjects.json>")
	os.Exit(2)
}
