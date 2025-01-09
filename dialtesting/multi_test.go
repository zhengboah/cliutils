// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package dialtesting

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMulti(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	}))

	step := HTTPTask{
		URL: server.URL + "?token={{config_var_token}}-{{config_var_global}}",
		PostScript: `
			result["is_failed"] = false	
			vars["token"] = "token"
		`,
	}

	step1Bytes, _ := json.Marshal(step)

	step = HTTPTask{
		URL: fmt.Sprintf("%s?token={{token}}-{{config_var_token}}-{{config_var_global}}", server.URL),
		PostScript: `
			result["is_failed"] = true
			result["error_message"]	= "error"
		`,
	}

	step2Bytes, _ := json.Marshal(step)

	multiTask := &MultiTask{
		Task: &Task{
			ConfigVars: []*ConfigVar{
				{
					Name:  "config_var_token",
					Value: "config_var_token",
				},
				{
					Name: "config_var_global",
					ID:   "global_var_id",
					Type: TypeVariableGlobal,
				},
			},
		},
		Steps: []*MultiStep{
			{
				Type:       "http",
				TaskString: string(step1Bytes),
				ExtractedVars: []MultiExtractedVar{
					{
						Name:  "token",
						Field: "token",
					},
				},
			},
			{
				Type:  "wait",
				Value: 1,
			},

			{
				Type:       "http",
				TaskString: string(step2Bytes),
			},
		},
	}

	taskString, _ := json.Marshal(multiTask)

	task, err := NewTask(string(taskString), multiTask)

	assert.NoError(t, err)

	globalVars := map[string]Variable{
		"global_var_id": {
			Value: "global_var_value",
		},
	}
	assert.NoError(t, task.RenderTemplateAndInit(globalVars))

	assert.NoError(t, task.Run())

	tags, fields := task.GetResults()

	assert.True(t, tags != nil)
	assert.True(t, fields != nil)

	assert.Equal(t, "FAIL", tags["status"])
	assert.EqualValues(t, -1, fields["success"])
	assert.Equal(t, 3, len(multiTask.Steps))
	assert.Equal(t, fmt.Sprintf("%s?token=config_var_token-global_var_value", server.URL), multiTask.Steps[0].result["url"])
	assert.Equal(t, fmt.Sprintf("%s?token=token-config_var_token-global_var_value", server.URL), multiTask.Steps[2].result["url"])
}
