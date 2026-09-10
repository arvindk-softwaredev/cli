// Copyright © 2026 The Tekton Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package formatted

import (
	"bytes"
	"encoding/json"
	"testing"

	"gotest.tools/v3/assert"
	"sigs.k8s.io/yaml"
)

func TestIsStructured(t *testing.T) {
	assert.Equal(t, IsStructured("json"), true)
	assert.Equal(t, IsStructured("YAML"), true)
	assert.Equal(t, IsStructured("csv"), false)
	assert.Equal(t, IsStructured(""), false)
}

func TestValidateOutput(t *testing.T) {
	assert.NilError(t, ValidateOutput(""))
	assert.NilError(t, ValidateOutput("json"))
	assert.NilError(t, ValidateOutput("YAML"))

	err := ValidateOutput("csv")
	if err == nil {
		t.Fatal("expected error for invalid output format")
	}
	assert.ErrorContains(t, err, "invalid output format \"csv\"")
	assert.ErrorContains(t, err, "must be json or yaml")
}

func TestPrint_JSONArray(t *testing.T) {
	var buf bytes.Buffer
	items := []map[string]string{
		{"name": "foo"},
		{"name": "bar"},
	}
	assert.NilError(t, PrintStructuredOutput(&buf, "json", items))

	var got []map[string]string
	assert.NilError(t, json.Unmarshal(buf.Bytes(), &got))
	assert.DeepEqual(t, items, got)
}

func TestPrint_YAMLArray(t *testing.T) {
	var buf bytes.Buffer
	items := []map[string]string{
		{"name": "foo"},
		{"name": "bar"},
	}
	assert.NilError(t, PrintStructuredOutput(&buf, "yaml", items))

	var got []map[string]string
	assert.NilError(t, yaml.Unmarshal(buf.Bytes(), &got))
	assert.DeepEqual(t, items, got)
}

func TestPrint_JSONObject(t *testing.T) {
	var buf bytes.Buffer
	obj := map[string]string{"name": "foo"}
	assert.NilError(t, PrintStructuredOutput(&buf, "json", obj))

	var got map[string]string
	assert.NilError(t, json.Unmarshal(buf.Bytes(), &got))
	assert.DeepEqual(t, obj, got)
}

func TestPrint_InvalidFormat(t *testing.T) {
	var buf bytes.Buffer
	err := PrintStructuredOutput(&buf, "csv", map[string]string{"name": "foo"})
	if err == nil {
		t.Fatal("expected error for invalid output format")
	}
	assert.ErrorContains(t, err, "invalid output format \"csv\"")
}

func TestPrint_EmptySliceIsJSONArray(t *testing.T) {
	var buf bytes.Buffer
	assert.NilError(t, PrintStructuredOutput(&buf, "json", []string{}))

	var got []string
	assert.NilError(t, json.Unmarshal(buf.Bytes(), &got))
	assert.Equal(t, len(got), 0)
}
