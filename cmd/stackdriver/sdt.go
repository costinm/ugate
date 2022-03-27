// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/costinm/ugate/ext/stackdriver"
)

var (
	p = flag.String("p", os.Getenv("PROJECT_ID"), "Project ID")

	r = flag.String("r", "", "Resource type.")

	ns = flag.String("n", os.Getenv("WORKLOAD_NAMESPACE"), "Namespace")
	wname = flag.String("s", os.Getenv("WORKLOAD_NAME"), "Service name")
	rev = flag.String("v", "", "Version/revision")
	metric = flag.String("m", "istio.io/service/client/request_count", "Metric name")
	extra = flag.String("x", "", "Extra query parameters")

	includeZero = flag.Bool("zero", false, "Include metrics with zero value")
	jsonF = flag.Bool("json", false, "json output")
)

func main() {
	flag.Parse()
	projectID := *p
	if *p == "" {
		projectID = "wlhe-cr"
		//panic("Missing PROJECT_ID")
		//return
	}

	sd, err := stackdriver.NewStackdriver(projectID)
	if err != nil {
		panic(err)
	}

	// Verify client side metrics (in pod) reflect the CloudrRun server properties
	ts, err := sd.ListTimeSeries(context.Background(),
		*ns, *r,
		*metric, *extra)
		//" AND metric.labels.source_canonical_service_name = \"fortio\"" +
		//		" AND metric.labels.response_code = \"200\"")
	if err != nil {
		log.Fatalf("Error %v", err)
	}

	for _, tsr := range ts {
		v := tsr.Points[0].Value
		if ! *includeZero && *v.DoubleValue == 0 {
			continue
		}
		if *jsonF {
			d, _ := json.Marshal(tsr)
			fmt.Println(string(d))
		} else {
			fmt.Printf("%v %v\n", *v.DoubleValue, tsr.Metric.Labels)
		}
	}
}

