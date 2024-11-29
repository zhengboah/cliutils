// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package dialtesting

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"

	v8 "rogchap.com/v8go"
)

//go:embed "http_script.js"
var initScript string
var vm = v8.NewIsolate()

type ScriptHTTPRequestResponse struct {
	Headers    http.Header `json:"headers"`
	Body       string      `json:"body"`
	StatusCode int         `json:"statusCode"`
}

func (h *ScriptHTTPRequestResponse) String() (string, error) {
	if bytes, err := json.Marshal(h); err != nil {
		return "", fmt.Errorf("response marshal failed: %w", err)
	} else {
		return string(bytes), nil
	}
}

type ScriptAPIContent struct {
	Values       map[string]string `json:"values"`
	IsFailed     bool              `json:"is_failed"`
	ErrorMessage string            `json:"error_message"`
}

type ScriptResult struct {
	Response *ScriptHTTPRequestResponse `json:"response"`
	API      *ScriptAPIContent          `json:"api"`
}

func postScriptDo(script string, bodyBytes []byte, resp *http.Response) (*ScriptResult, error) {
	if script == "" || resp == nil {
		return nil, nil
	}

	headers, err := json.Marshal(resp.Header)
	if err != nil {
		return nil, fmt.Errorf("header marshal failed: %w", err)
	}
	body := string(bodyBytes)
	ctx := v8.NewContext(vm)
	defer ctx.Close()
	if _, err := ctx.RunScript(initScript, "init.js"); err != nil {
		return nil, fmt.Errorf("init script failed: %w", err)
	}

	setResponseScript := fmt.Sprintf(`let response = new Response(%d, '%s', '%s')`, resp.StatusCode, headers, body)

	if _, err := ctx.RunScript(setResponseScript, "setResponse.js"); err != nil {
		return nil, fmt.Errorf("setResponse script failed: %w", err)
	}

	if _, err := ctx.RunScript(fmt.Sprintf(`
	(function runScript(response, api){
		try {
			%s
		} catch(e) {
			api.fail(e.message)
		}
	})(response, api)
	`, script), "script.js"); err != nil {
		return nil, fmt.Errorf("script failed: %w", err)
	}

	if value, err := ctx.RunScript("getResult(response, api)", "result.js"); err != nil {
		return nil, fmt.Errorf("api failed: %w", err)
	} else {
		result := value.String()
		res := ScriptResult{}
		if err := json.Unmarshal([]byte(result), &res); err != nil {
			return nil, fmt.Errorf("json.Marshal failed: %w", err)
		} else {
			return &res, nil
		}

	}

}
