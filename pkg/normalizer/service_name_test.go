// Copyright (c) 2019 The Jaeger Authors.
// Copyright (c) 2017 Uber Technologies, Inc.
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

package normalizer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
)

func TestServiceNameReplacer(t *testing.T) {
	assert.Equal(t, "abc", ServiceName("ABC"), "lower case conversion")
	assert.Equal(t, "a_b_c__", ServiceName("a&b%c/:"), "disallowed runes to underscore")
	assert.Equal(t, "a_z_0123456789.", ServiceName("A_Z_0123456789."), "allowed runes")
}

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
