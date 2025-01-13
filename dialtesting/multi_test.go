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
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestMulti(t *testing.T) {
	body := struct {
		Token string
	}{
		Token: fmt.Sprintf("token_%d", time.Now().UnixNano()),
	}

	engine := gin.Default()
	engine.GET("/token", func(ctx *gin.Context) {
		bodyBytes, _ := json.Marshal(body)
		ctx.Writer.Write(bodyBytes)
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		engine.ServeHTTP(w, r)
	}))
	defer server.Close()

	cases := makeCases(server.URL)

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			taskString, _ := json.Marshal(tc.Task)
			task, err := NewTask(string(taskString), tc.Task)

			assert.NoError(t, err)

			globalVars := map[string]Variable{
				"global_var_id": {
					Value: "global_var_value",
				},
			}
			assert.NoError(t, task.RenderTemplateAndInit(globalVars))

			assert.NoError(t, task.Run())

			tags, fields := task.GetResults()
			if tc.IsFailed {
				assert.Equal(t, -1, fields["success"])
			} else {
				assert.Equal(t, 1, fields["success"])
			}
			assert.NotNil(t, tags, fields)
			assert.NoError(t, tc.Check(t, tags, fields))
		})
	}
}

type cs struct {
	Name       string
	Task       *MultiTask
	IsFailed   bool
	Check      func(t assert.TestingT, tags map[string]string, fields map[string]interface{}) error
	GlobalVars map[string]Variable
}

func makeCases(serverURL string) []cs {
	return []cs{
		{
			Name:     "normal test",
			IsFailed: true,
			Task: func() *MultiTask {
				step1 := HTTPTask{
					URL: serverURL + "/token?token={{config_var_token}}-{{config_var_global}}",
					PostScript: `
			result["is_failed"] = false	
			body = load_json(response["body"])
			vars["token"] = body["Token"]
		`,
				}

				step1Bytes, _ := json.Marshal(step1)

				step3 := HTTPTask{
					URL: fmt.Sprintf("%s/token?token={{token}}", serverURL),
					PostScript: `
			result["is_failed"] = true
			result["error_message"]	= "error"
		`,
				}

				step2Bytes, _ := json.Marshal(step3)
				return &MultiTask{
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
			}(),
			GlobalVars: map[string]Variable{
				"global_var_id": {
					Value: "global_var_value",
				},
			},
			Check: func(t assert.TestingT, tags map[string]string, fields map[string]interface{}) error {
				assert.Equal(t, "FAIL", tags["status"])
				msg := map[string]interface{}{}
				message, ok := fields["message"].(string)
				assert.True(t, ok)
				assert.NoError(t, json.Unmarshal([]byte(message), &msg))
				assert.Equal(t, "error", msg["fail_reason"])
				assert.EqualValues(t, -1, fields["success"])
				return nil
			},
		},
	}
}
