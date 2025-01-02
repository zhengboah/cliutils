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
		URL: server.URL,
		PostScript: `
			result["is_failed"] = false	
			vars["token"] = "token"
		`,
	}

	step1Bytes, _ := json.Marshal(step)

	step = HTTPTask{
		URL: fmt.Sprintf("%s/{{token}}", server.URL),
		PostScript: `
			result["is_failed"] = true
			result["error_message"]	= "error"
		`,
	}

	step2Bytes, _ := json.Marshal(step)

	multiTask := &MultiTask{
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

	task, err := NewTask(multiTask)

	assert.NoError(t, err)

	assert.NoError(t, task.RenderTemplateAndInit(nil))

	assert.NoError(t, task.Run())

	tags, fields := task.GetResults()

	assert.True(t, tags != nil)
	assert.True(t, fields != nil)

	assert.Equal(t, "FAIL", tags["status"])
	assert.EqualValues(t, -1, fields["success"])
}
