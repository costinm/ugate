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

package stackdriver

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"google.golang.org/api/monitoring/v3"
)

var (
	// https://istio.io/latest/docs/reference/config/metrics/
	// https://cloud.google.com/monitoring/api/metrics_istio

	// service/client
	IstioPrefix = "istio.io/service/client/"
	IstioPrefixServer = "istio.io/service/server/"

	// Resource:
	// - k8s_pod
	// - gce_instance

	// - istio_canonical_service - for request_count, roundtrip_latencies

	// "Sampled every 60 sec, not visible up to 180 sec" - so ~4 min window
	// for a test to validate metric after request is made, or for autoscale to adjust
	IstioMetrics = []string{
		// Autoscaling for short lived
		"request_count", // DELTA, INT64, 1

		// May be used for auto-scaling if it gets too high, only for
		// short-lived only.
		"roundtrip_latencies", // DELTA, DISTRIBUTION

		// Autoscaling for WS/long lived.
		"connection_close_count", // CUMULATIVE, INT64, 1
		"connection_open_count", // CUMULATIVE, INT64, 1

		// Useful for bandwidth limits - if the rate is close to line speed.
		// Very unlikely in this form.
		"received_bytes_count", // CUMULATIVE, INT64, 1
		"sent_bytes_count", // CUMULATIVE, INT64, 1

		// Useful for stats on payload size, not for scaling or health.
		"request_bytes", // DELTA, DISTRIBUTION
		"response_bytes", // DELTA, DISTRIBUTION
	}

	IstioLabels = []string {
		"request_protocol", // Protocol of the request or connection (e.g. HTTP, gRPC, TCP).
		"service_authentication_policy", // : Determines if Istio was used to secure communications between services and how. Currently supported values: "NONE", "MUTUAL_TLS".
		"mesh_uid", // : Unique identifier for the mesh that is being monitored.
		"destination_service_name", //: Name of destination service.
		"destination_service_namespace", //: Namespace of destination service.
		"destination_port", //: (INT64) Port of the destination service.
		"source_principal", //: Principal of the source workload instance.
		"source_workload_name", //: Name of the source workload.
		"source_workload_namespace", //: Namespace of the source workload.
		"source_owner", //: Owner of the source workload instance (e.g. k8s Deployment).
		"destination_principal", //: Principal of the destination workload instance.
		"destination_workload_name", //: Name of the destination workload.
		"destination_workload_namespace", //: Namespace of the destination workload.
		"destination_owner", //: Owner of the destination workload instance (e.g. k8s Deployment).
		"source_canonical_service_name", //: The canonical service name of the source workload instance.
		"destination_canonical_service_name", //: The canonical service name of the destination workload instance.
		"source_canonical_service_namespace", //: The canonical service namespace of the source workload instance.
		"destination_canonical_service_namespace", //: The canonical service namespace of the destination workload instance.
		"source_canonical_revision", //: The canonical service revision of the source workload instance.
		"destination_canonical_revision", //: The canonical service revision of the destination workload instance.

		// For *_bytes, request_count,
		// Not for connection*, *_bytes_count
		"request_operation", // : Unique string used to identify the API method (if available) or HTTP Method.
		"api_version", // : Version of the API.
		"response_code", // : (INT64) Response code of the request according to protocol.
		"api_name", // : Name of the API.
	}

	VMResourceLabels = []string{
		"project_id",
		"instance_id", // for gce_instance
		"zone",
	}
  PodResourceLabels = []string{
		"project_id",
		"location", // for pod
		"cluster_name",
		"namespace_name",
		"pod_name",
	}

	// This is aggregated from pod metrics.
	SvcResourceLabels = []string {
		"mesh_uid",
		"project_id",
		"location",
		"namespace_name",
		"canonical_service_name",
		"revision",
	}

	// Typical metric:
	// destination_canonical_revision:latest
	// destination_canonical_service_name:fortio-cr
	// destination_canonical_service_namespace:fortio
	// destination_owner:unknown
	// destination_port:15442
	// destination_principal:spiffe://wlhe-cr.svc.id.goog/ns/fortio/sa/default
	// destination_service_name:fortio-cr-icq63pqnqq-uc
	// destination_service_namespace:fortio
	// destination_workload_name:fortio-cr-sni
	// destination_workload_namespace:fortio
	// mesh_uid:proj-601426346923
	// request_operation:GET
	// request_protocol:http
	// response_code:200
	// service_authentication_policy:unknown
	// source_canonical_revision:v1
	// source_canonical_service_name:fortio
	// source_canonical_service_namespace:fortio
	// source_owner:kubernetes://apis/apps/v1/namespaces/fortio/deployments/fortio
	// source_principal:spiffe://wlhe-cr.svc.id.goog/ns/fortio/sa/default
	// source_workload_name:fortio
	// source_workload_namespace:fortio


)

// WIP.
//
// Integration and testing with stackdriver for 'proxyless' modes.
//
// With Envoy, this is implemented using WASM or native filters.
//
// For proxyless (gRPC or generic hbone / uProxy) we need to:
// - decode and generate the Istio header containing client info
// - generate the expected istio metrics.
//

// Request can also use the REST API:
// monitoring.googleapis.com/v3/projects/NAME/timeSeries
//   ?aggregation.alignmentPeriod=60s
//   &aggregation.crossSeriesReducer=REDUCE_NONE
//   &aggregation.perSeriesAligner=ALIGN_RATE
//   &alt=json
//   &filter=metric.type+%3D+%22istio.io%2Fservice%2Fclient%2Frequest_count%22+AND+resource.type+%3D+%22istio_canonical_service%22+AND+resource.labels.namespace_name+%3D+%22fortio%22
//  &interval.endTime=2021-09-30T14%3A32%3A51-07%3A00
//  &interval.startTime=2021-09-30T14%3A27%3A51-07%3A00
//  &prettyPrint=false


type Stackdriver struct {
	projectID         string
	monitoringService *monitoring.Service
}

var (
	queryInterval = -5 * time.Minute
)

func NewStackdriver(projectID string) (*Stackdriver, error){
	monitoringService, err := monitoring.NewService(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get monitoring service: %v", err)
	}

	return &Stackdriver{projectID: projectID, monitoringService: monitoringService}, nil
}

// Get metrics.
//
// Filter options:
// project, group.id, resource.type, resource.labels.[KEY], metric.type,
// metric.labels.[KEY]
func (s *Stackdriver) ListTimeSeries(ctx context.Context, namespace, resourceType, metricName, extra string) ([]*monitoring.TimeSeries, error) {
	endTime := time.Now()
	startTime := endTime.Add(queryInterval)

	f := fmt.Sprintf("metric.type = %q ",
		metricName)
	if resourceType != "" {
		f = f + fmt.Sprintf(" AND resource.type = %q", resourceType)
	}
	if namespace != "" {
		f = f + fmt.Sprintf(" AND resource.labels.namespace_name = %q", namespace)
	}
	if extra != "" {
		f = f + extra
	}

	lr := s.monitoringService.Projects.TimeSeries.List(fmt.Sprintf("projects/%v", s.projectID)).
		IntervalStartTime(startTime.Format(time.RFC3339)).
		IntervalEndTime(endTime.Format(time.RFC3339)).
		AggregationCrossSeriesReducer("REDUCE_NONE").
		AggregationAlignmentPeriod("60s").
		AggregationPerSeriesAligner("ALIGN_RATE").
		Filter(f).//, destCanonical
		Context(ctx)
	resp, err := lr.Do()
	if err != nil {
		return nil, err
	}
	if resp.HTTPStatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get expected status code from monitoring service, got: %d", resp.HTTPStatusCode)
	}

	return resp.TimeSeries, nil
}

// For a metric, list resource types that generated the metric and the names.
func (s *Stackdriver) ListResources(ctx context.Context, namespace, metricName, extra string) ([]*monitoring.TimeSeries, error) {
	endTime := time.Now()
	startTime := endTime.Add(queryInterval)

	f := fmt.Sprintf("metric.type = %q ", metricName)
	if namespace != "" {
		f = f + fmt.Sprintf(" AND resource.labels.namespace_name = %q", namespace)
	}
	if extra != "" {
		f = f + extra
	}

	lr := s.monitoringService.Projects.TimeSeries.List(fmt.Sprintf("projects/%v", s.projectID)).
		IntervalStartTime(startTime.Format(time.RFC3339)).
		IntervalEndTime(endTime.Format(time.RFC3339)).
		AggregationCrossSeriesReducer("REDUCE_NONE").
		AggregationAlignmentPeriod("60s").
		AggregationPerSeriesAligner("ALIGN_RATE").
		Filter(f).//, destCanonical
		Context(ctx)
	resp, err := lr.Do()
	if err != nil {
		return nil, err
	}
	if resp.HTTPStatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get expected status code from monitoring service, got: %d", resp.HTTPStatusCode)
	}

	return resp.TimeSeries, nil
}

