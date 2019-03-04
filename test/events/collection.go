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

package events

import v1 "k8s.io/api/core/v1"

// Any returns true if any call to `f` evaluates to `true`
func Any(events []v1.Event, f func(v1.Event) bool) bool {
	for _, e := range events {
		if f(e) {
			return true
		}
	}
	return false
}

// None returns true if any call to `f` evaluates to `false`
func None(events []v1.Event, f func(v1.Event) bool) bool {
	return !Any(events, f)
}

// Select filters the given events according to the given function `f`
func Select(events []v1.Event, f func(v1.Event) bool) []v1.Event {
	filtered := []v1.Event{}
	for _, e := range events {
		if f(e) {
			filtered = append(filtered, e)
		}
	}
	return filtered
}
