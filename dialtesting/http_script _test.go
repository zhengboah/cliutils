// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package dialtesting

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPostScriptDo(t *testing.T) {
	cases := []struct {
		Name     string
		Script   string
		Expected *ScriptResult
		Body     string
		Response *http.Response
		IsFailed bool
	}{
		{
			Name:   "response object",
			Script: "1",
			Expected: &ScriptResult{
				Response: &ScriptHTTPRequestResponse{
					Body: "1",
					Headers: http.Header{
						"custom-header": {"value1", "value2"},
					},
					StatusCode: 301,
				},
				API: &ScriptAPIContent{
					Values: map[string]string{},
				},
			},
			Body: "1",
			Response: &http.Response{
				StatusCode: 301,
				Header: http.Header{
					"custom-header": {"value1", "value2"},
				},
			},
		},
		{
			Name:   "invalid script",
			Script: "response.xxxxxx = xxxxxx",
			Expected: &ScriptResult{
				Response: &ScriptHTTPRequestResponse{
					Body: "1",
					Headers: http.Header{
						"custom-header": {"value1", "value2"},
					},
					StatusCode: 301,
				},
				API: &ScriptAPIContent{
					Values: map[string]string{},
					IsFailed: true,
					ErrorMessage: "",
				},
			},
			Body: "1",
			Response: &http.Response{
				StatusCode: 301,
				Header: http.Header{
					"custom-header": {"value1", "value2"},
				},
			},
		},
		{
			Name: "response.getHeaders",
			Script: `
			let headers = response.getHeaders();
			let headerEqual = headers["header1"] && headers["header1"].length == 1 && headers["header1"][0] == "value1" &&
			headers["header2"] && headers["header2"].length == 1 && headers["header2"][0] == "value2"
			if (!headerEqual) {
				api.fail("header not equal")	
				return
			}

			let header1 = headers.get("header1")
			if (!header1 || header1.length != 1 || header1[0] != "value1") {
				api.fail("header1 not equal")
				return
			}
			`,
			Expected: &ScriptResult{
				Response: &ScriptHTTPRequestResponse{
					Body: "1",
					Headers: http.Header{
						"header1": {"value1"},
						"header2": {"value2"},
					},
					StatusCode: 301,
				},
				API: &ScriptAPIContent{
					Values: map[string]string{},
				},
			},
			Body: "1",
			Response: &http.Response{
				StatusCode: 301,
				Header: http.Header{
					"header1": {"value1"},
					"header2": {"value2"},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			result, err := postScriptDo(tc.Script, []byte(tc.Body), tc.Response)
			if tc.IsFailed {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.True(t, reflect.DeepEqual(tc.Expected, result))
		})
	}

}
