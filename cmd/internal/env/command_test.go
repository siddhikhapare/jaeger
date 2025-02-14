// Copyright (c) 2018 The Jaeger Authors.
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

package env

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"

	"github.com/jaegertracing/jaeger/pkg/testutils"
)

func TestCommand(t *testing.T) {
	cmd := Command()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.Run(cmd, nil)
	assert.True(t, strings.Contains(buf.String(), "METRICS_BACKEND"))
	assert.True(t, strings.Contains(buf.String(), "SPAN_STORAGE"))
}

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m, testutils.IgnoreGlogFlushDaemonLeak())
}
