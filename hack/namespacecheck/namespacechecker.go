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

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	faros "github.com/pusher/faros/pkg/apis/faros/v1alpha1"
)

var (
	gtFlag     = flag.String("gt-file", "", "gittrack file to read")
	gtoFlag    = flag.String("gto-file", "", "gittrackobject file to read")
	cgtoFlag   = flag.String("cgto-file", "", "clustergittrackobject file to read, optional")
	outputFlag = flag.String("output", "apply.json", "output file for clustergittracks to create")
)

func main() {
	flag.Parse()
	if *gtFlag == "" || *gtoFlag == "" {
		fmt.Fprintf(os.Stderr, "Must give --gt-file and --gto-file flags\n")
		usage()
	}
	gtfile, err := ioutil.ReadFile(*gtFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		usage()
	}
	gtofile, err := ioutil.ReadFile(*gtoFlag)
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

	if *cgtoFlag != "" {
		cgtfile, err := ioutil.ReadFile(*cgtoFlag)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			usage()
		}
		output := makeCGTs(gtjson, cgtfile)
		err = ioutil.WriteFile(*outputFlag, output, 0644)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		fmt.Println("wrote ClusterGitTrack file to", *outputFlag)
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

	var cgtList faros.ClusterGitTrackList
	cgtList.APIVersion = "v1"
	cgtList.Kind = "List"
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
		cgtList.Items = append(cgtList.Items, cgt)
	}
	output, err := json.MarshalIndent(cgtList, "", "\t")
	if err != nil {
		panic(err)
	}
	return output
}

func usage() {
	flag.PrintDefaults()
	os.Exit(2)
}
