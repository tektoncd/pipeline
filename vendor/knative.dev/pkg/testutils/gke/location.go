/*
Copyright 2019 The Knative Authors

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

package gke

import (
	"fmt"
	"strings"
)

// GetClusterLocation returns the location used in GKE operations, given the region and zone.
func GetClusterLocation(region, zone string) string {
	if zone != "" {
		region = fmt.Sprintf("%s-%s", region, zone)
	}
	return region
}

// RegionZoneFromLoc returns the region and the zone, given the location.
func RegionZoneFromLoc(location string) (string, string) {
	parts := strings.Split(location, "-")
	// zonal location is the form of us-central1-a, and this pattern is
	// consistent in all available GCP locations so far, so we are looking for
	// location with more than 2 "-"
	if len(parts) > 2 {
		zone := parts[len(parts)-1]
		region := strings.TrimRight(location, "-"+zone)
		return region, zone
	}
	return location, ""
}
