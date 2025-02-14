// Copyright (c) 2021 The Jaeger Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package apiv3

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"testing"

	gogojsonpb "github.com/gogo/protobuf/jsonpb"
	gogoproto "github.com/gogo/protobuf/proto"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jaegertracing/jaeger/model"
	_ "github.com/jaegertracing/jaeger/pkg/gogocodec" // force gogo codec registration
	"github.com/jaegertracing/jaeger/pkg/tenancy"
	"github.com/jaegertracing/jaeger/proto-gen/api_v3"
	"github.com/jaegertracing/jaeger/storage/spanstore"
	spanstoremocks "github.com/jaegertracing/jaeger/storage/spanstore/mocks"
)

// Utility functions used from http_gateway_test.go.

const (
	snapshotLocation = "./snapshots/"
)

// Snapshots can be regenerated via:
//
//	REGENERATE_SNAPSHOTS=true go test -v ./cmd/query/app/apiv3/...
var regenerateSnapshots = os.Getenv("REGENERATE_SNAPSHOTS") == "true"

type testGateway struct {
	reader *spanstoremocks.Reader
	url    string
	router *mux.Router
	// used to set a tenancy header when executing requests
	setupRequest func(*http.Request)
}

func (gw *testGateway) execRequest(t *testing.T, url string) ([]byte, int) {
	req, err := http.NewRequest(http.MethodGet, gw.url+url, nil)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	gw.setupRequest(req)
	response, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	return body, response.StatusCode
}

func (gw *testGateway) verifySnapshot(t *testing.T, body []byte) []byte {
	// reformat JSON body with indentation, to make diffing easier
	var data interface{}
	require.NoError(t, json.Unmarshal(body, &data), "response: %s", string(body))
	body, err := json.MarshalIndent(data, "", "  ")
	require.NoError(t, err)

	testName := path.Base(t.Name())
	snapshotFile := filepath.Join(snapshotLocation, testName+".json")
	if regenerateSnapshots {
		os.WriteFile(snapshotFile, body, 0o644)
	}
	snapshot, err := os.ReadFile(snapshotFile)
	require.NoError(t, err)
	assert.Equal(t, string(snapshot), string(body), "comparing against stored snapshot. Use REGENERATE_SNAPSHOTS=true to rebuild snapshots.")
	return body
}

func parseResponse(t *testing.T, body []byte, obj gogoproto.Message) {
	require.NoError(t, gogojsonpb.Unmarshal(bytes.NewBuffer(body), obj))
}

func makeTestTrace() (*model.Trace, model.TraceID) {
	traceID := model.NewTraceID(150, 160)
	return &model.Trace{
		Spans: []*model.Span{
			{
				TraceID:       traceID,
				SpanID:        model.NewSpanID(180),
				OperationName: "foobar",
				Tags: []model.KeyValue{
					model.String("span.kind", "server"),
					model.Bool("error", true),
				},
			},
		},
	}, traceID
}

func runGatewayTests(
	t *testing.T,
	basePath string,
	tenancyOptions tenancy.Options,
	setupRequest func(*http.Request),
) {
	gw := setupHTTPGateway(t, basePath, tenancyOptions)
	gw.setupRequest = setupRequest
	t.Run("GetServices", gw.runGatewayGetServices)
	t.Run("GetOperations", gw.runGatewayGetOperations)
	t.Run("GetTrace", gw.runGatewayGetTrace)
	t.Run("FindTraces", gw.runGatewayFindTraces)
}

func (gw *testGateway) runGatewayGetServices(t *testing.T) {
	gw.reader.On("GetServices", matchContext).Return([]string{"foo"}, nil).Once()

	body, statusCode := gw.execRequest(t, "/api/v3/services")
	require.Equal(t, http.StatusOK, statusCode)
	body = gw.verifySnapshot(t, body)

	var response api_v3.GetServicesResponse
	parseResponse(t, body, &response)
	assert.Equal(t, []string{"foo"}, response.Services)
}

func (gw *testGateway) runGatewayGetOperations(t *testing.T) {
	qp := spanstore.OperationQueryParameters{ServiceName: "foo", SpanKind: "server"}
	gw.reader.
		On("GetOperations", matchContext, qp).
		Return([]spanstore.Operation{{Name: "get_users", SpanKind: "server"}}, nil).Once()

	body, statusCode := gw.execRequest(t, "/api/v3/operations?service=foo&span_kind=server")
	require.Equal(t, http.StatusOK, statusCode)
	body = gw.verifySnapshot(t, body)

	var response api_v3.GetOperationsResponse
	parseResponse(t, body, &response)
	require.Len(t, response.Operations, 1)
	assert.Equal(t, "get_users", response.Operations[0].Name)
	assert.Equal(t, "server", response.Operations[0].SpanKind)
}

func (gw *testGateway) runGatewayGetTrace(t *testing.T) {
	trace, traceID := makeTestTrace()
	gw.reader.On("GetTrace", matchContext, traceID).Return(trace, nil).Once()

	body, statusCode := gw.execRequest(t, "/api/v3/traces/"+traceID.String())
	require.Equal(t, http.StatusOK, statusCode, "response=%s", string(body))
	body = gw.verifySnapshot(t, body)

	var response api_v3.GRPCGatewayWrapper
	parseResponse(t, body, &response)

	assert.Len(t, response.Result.ResourceSpans, 1)
	assert.EqualValues(t,
		bytesOfTraceID(t, traceID.High, traceID.Low),
		response.Result.ResourceSpans[0].ScopeSpans[0].Spans[0].TraceID)
}

func (gw *testGateway) runGatewayFindTraces(t *testing.T) {
	trace, traceID := makeTestTrace()
	q, qp := mockFindQueries()
	gw.reader.
		On("FindTraces", matchContext, qp).
		Return([]*model.Trace{trace}, nil).Once()
	body, statusCode := gw.execRequest(t, "/api/v3/traces?"+q.Encode())
	require.Equal(t, http.StatusOK, statusCode, "response=%s", string(body))
	body = gw.verifySnapshot(t, body)

	var response api_v3.GRPCGatewayWrapper
	parseResponse(t, body, &response)

	assert.Len(t, response.Result.ResourceSpans, 1)
	assert.EqualValues(t,
		bytesOfTraceID(t, traceID.High, traceID.Low),
		response.Result.ResourceSpans[0].ScopeSpans[0].Spans[0].TraceID)
}

func bytesOfTraceID(t *testing.T, high, low uint64) []byte {
	traceID := model.NewTraceID(high, low)
	buf := make([]byte, 16)
	_, err := traceID.MarshalTo(buf)
	require.NoError(t, err)
	return buf
}
