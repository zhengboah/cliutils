// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

// Package dialtesting defined dialtesting tasks and task implements.
package dialtesting

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"text/template"
	"time"

	"github.com/GuanceCloud/cliutils"
)

const (
	StatusStop = "stop"

	ClassHTTP      = "HTTP"
	ClassTCP       = "TCP"
	ClassWebsocket = "WEBSOCKET"
	ClassICMP      = "ICMP"
	ClassDNS       = "DNS"
	ClassHeadless  = "BROWSER"
	ClassOther     = "OTHER"

	MaxMsgSize = 15 * 1024 * 1024
)

type ConfigVar struct {
	ID      string `json:"id,omitempty"`
	Type    string `json:"type,omitempty"`
	Name    string `json:"name"`
	Value   string `json:"value,omitempty"`
	Example string `json:"example,omitempty"`
	Secure  bool   `json:"secure"`
}

var TypeVariableGlobal = "global"

type Variable struct {
	Id          int    `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	UUID        string `json:"uuid,omitempty"`
	TaskID      string `json:"task_id,omitempty"`
	TaskVarName string `json:"task_var_name,omitempty"`
	Value       string `json:"value,omitempty"`
	Secure      bool   `json:"secure,omitempty"`
	PostScript  string `json:"post_script,omitempty"`

	UpdatedAt       int64  `json:"updated_at,omitempty"`
	OwnerExternalID string `json:"owner_external_id,omitempty"`
	CreatedAt       int64  `json:"-"`
	DeletedAt       int64  `json:"-"`
}
type TaskChild interface {
	beforeFirstRender()
	run() error
	init() error
	checkResult() ([]string, bool)
	getResults() (map[string]string, map[string]interface{})
	stop() error
	check() error
	clear()
	getVariableValue(variable Variable) (string, error)
	class() string
	metricName() string
	getHostName() (string, error)
}

func getHostName(host string) (string, error) {
	reqURL, err := url.Parse(host)
	if err != nil {
		return "", fmt.Errorf("parse host error: %w", err)
	}

	return reqURL.Hostname(), nil
}

type ITask interface {
}

type Task struct {
	ExternalID        string             `json:"external_id"`
	Name              string             `json:"name"`
	AK                string             `json:"access_key"`
	Method            string             `json:"method"`
	PostURL           string             `json:"post_url"`
	CurStatus         string             `json:"status"`
	Frequency         string             `json:"frequency"`
	Region            string             `json:"region"`
	OwnerExternalID   string             `json:"owner_external_id"`
	Tags              map[string]string  `json:"tags,omitempty"`
	Labels            []string           `json:"labels,omitempty"`
	WorkspaceLanguage string             `json:"workspace_language,omitempty"`
	TagsInfo          string             `json:"tags_info,omitempty"` // deprecated
	DFLabel           string             `json:"df_label,omitempty"`
	AdvanceOptions    *HTTPAdvanceOption `json:"advance_options,omitempty"`
	UpdateTime        int64              `json:"update_time,omitempty"`
	ConfigVars        []*ConfigVar       `json:"config_vars,omitempty"`

	ticker               *time.Ticker
	taskJSONString       string
	parsedTaskJSONString string
	child                TaskChild

	inited bool
}

func (t *Task) SetChild(child TaskChild) {
	t.child = child
}

func (t *Task) UpdateTimeUs() int64 {
	return t.UpdateTime
}

func (t *Task) Clear() {
	t.child.clear()
}

func (t *Task) ID() string {
	if t.ExternalID == `` {
		return cliutils.XID("dtst_")
	}
	return fmt.Sprintf("%s_%s", t.AK, t.ExternalID)
}

func (t *Task) GetOwnerExternalID() string {
	return t.OwnerExternalID
}

func (t *Task) GetExternalID() string {
	return t.ExternalID
}

func (t *Task) SetOwnerExternalID(exid string) {
	t.OwnerExternalID = exid
}

func (t *Task) SetRegionID(regionID string) {
	t.Region = regionID
}

func (t *Task) SetAk(ak string) {
	t.AK = ak
}

func (t *Task) SetStatus(status string) {
	t.CurStatus = status
}

func (t *Task) SetUpdateTime(ts int64) {
	t.UpdateTime = ts
}

func (t *Task) Stop() error {
	return t.child.stop()
}

func (t *Task) Status() string {
	return t.CurStatus
}

func (t *Task) Ticker() *time.Ticker {
	return t.ticker
}

func (t *Task) Class() string {
	return t.child.class()
}

func (t *Task) MetricName() string {
	return t.child.metricName()
}

func (t *Task) PostURLStr() string {
	return t.PostURL
}

func (t *Task) GetFrequency() string {
	return t.Frequency
}

func (t *Task) GetLineData() string {
	return ""
}

func (t *Task) GetResults() (tags map[string]string, fields map[string]interface{}) {
	return t.child.getResults()
}

func (t *Task) RegionName() string {
	return t.Region
}

func (t *Task) AccessKey() string {
	return t.AK
}

func (t *Task) Check() error {
	// TODO: check task validity
	if t.ExternalID == "" {
		return fmt.Errorf("external ID missing")
	}

	if err := t.child.check(); err != nil {
		return err
	}

	return t.init(false)
}

func (t *Task) Run() error {
	t.Clear()
	return t.child.run()
}

func (t *Task) InitDebug() error {
	return t.init(true)
}

func (t *Task) init(debug bool) error {
	defer func() {
		t.inited = true
	}()
	if !debug {
		du, err := time.ParseDuration(t.Frequency)
		if err != nil {
			return err
		}
		if t.ticker != nil {
			t.ticker.Stop()
		}
		t.ticker = time.NewTicker(du)
	}

	if strings.EqualFold(t.CurStatus, StatusStop) {
		return nil
	}

	return t.child.init()

}

func (t *Task) Init() error {
	return t.init(false)
}

func (t *Task) GetHostName() (string, error) {
	return t.child.getHostName()
}

func (t *Task) GetWorkspaceLanguage() string {
	if t.WorkspaceLanguage == "en" {
		return "en"
	}
	return "zh"
}

func (t *Task) GetDFLabel() string {
	if t.DFLabel != "" {
		return t.DFLabel
	}
	return t.TagsInfo
}

func (t *Task) SetTaskJSONString(s string) {
	t.taskJSONString = s
}

func (t *Task) GetTaskJSONString() string {
	return t.taskJSONString
}

func (t *Task) GetGlobalVars() []string {
	vars := []string{}
	for _, v := range t.ConfigVars {
		if v.Type == TypeVariableGlobal {
			vars = append(vars, v.ID)
		}
	}
	return vars
}

// RenderTempate render template and init task.
func (t *Task) RenderTemplate(globalVariables map[string]Variable) error {
	defer func() {
		if !t.inited {
			t.init(false)
		}
	}()

	if !t.inited {
		t.child.beforeFirstRender()
	}

	if globalVariables == nil {
		globalVariables = make(map[string]Variable)
	}

	if len(t.ConfigVars) == 0 {
		return nil
	}

	fm := template.FuncMap{}

	for _, v := range t.ConfigVars {
		value := v.Value
		if v.Type == TypeVariableGlobal && v.ID != "" { // global variables
			if gv, ok := globalVariables[v.ID]; ok {
				value = gv.Value
				v.Secure = gv.Secure
			}
		}

		fm[v.Name] = func() string {
			return value
		}

		v.Value = value
	}

	tmpl, err := template.New("task").Funcs(fm).Option("missingkey=zero").Parse(t.taskJSONString)
	if err != nil {
		return fmt.Errorf("parse template error: %w", err)
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, nil); err != nil {
		return fmt.Errorf("execute template error: %w", err)
	}

	parsedString := buf.String()

	// no need to re-parse
	if parsedString == t.parsedTaskJSONString {
		return nil
	}

	t.parsedTaskJSONString = parsedString

	if err := json.Unmarshal([]byte(parsedString), t.child); err != nil {
		return fmt.Errorf("unmarshal parsed template error: %w", err)
	}

	t.init(t.inited)

	return nil
}

func (t *Task) GetVariableValue(variable Variable) (string, error) {
	return t.child.getVariableValue(variable)
}
