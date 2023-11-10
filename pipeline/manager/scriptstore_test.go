// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package manager

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/point"
	"github.com/stretchr/testify/assert"
)

func whichStore(c *Manager, cat point.Category) *ScriptStore {
	v, _ := c.whichStore(cat)
	return v
}

func TestScriptLoadFunc(t *testing.T) {
	center := NewManager(ManagerCfg{})
	case1 := map[point.Category]map[string]string{
		point.Logging: {
			"abcd": "if true {}",
		},
		point.Metric: {
			"abc": "if true {}",
			"def": "if true {}",
		},
	}

	LoadScripts(center, DefaultScriptNS, nil, nil)
	LoadScripts(center, GitRepoScriptNS, nil, nil)
	LoadScripts(center, RemoteScriptNS, nil, nil)

	LoadScripts(center, DefaultScriptNS, case1, nil)
	for category, v := range case1 {
		for name := range v {
			if y, ok := center.QueryScript(category, name); !ok {
				t.Error(category, " ", name, y)
				if y, ok := center.QueryScript(category, name); !ok {
					t.Error(y)
				}
			}
		}
	}

	LoadScripts(center, DefaultScriptNS, nil, nil)
	LoadScripts(center, GitRepoScriptNS, nil, nil)
	LoadScripts(center, RemoteScriptNS, nil, nil)
	for k, v := range case1 {
		LoadScript(center, k, DefaultScriptNS, v, nil)
	}
	for category, v := range case1 {
		for name := range v {
			if _, ok := center.QueryScript(category, name); !ok {
				t.Error(category, " ", name)
			}
		}
	}

	LoadScripts(center, DefaultScriptNS, nil, nil)
	LoadScripts(center, GitRepoScriptNS, nil, nil)
	LoadScripts(center, RemoteScriptNS, nil, nil)
	for category, v := range case1 {
		for name := range v {
			if _, ok := center.QueryScript(category, name); ok {
				t.Error(category, " ", name)
			}
		}
	}

	LoadScripts(center, DefaultScriptNS, nil, nil)
	LoadScripts(center, GitRepoScriptNS, nil, nil)
	LoadScripts(center, RemoteScriptNS, nil, nil)

	for k, v := range case1 {
		LoadScript(center, k, "DefaultScriptNS", v, nil)
		whichStore(center, k).UpdateScriptsWithNS(RemoteScriptNS, v, nil)
	}
	for category, v := range case1 {
		for name := range v {
			if s, ok := center.QueryScript(category, name); !ok || s.NS() != RemoteScriptNS {
				t.Error(category, " ", name)
			}
		}
	}

	LoadScripts(center, DefaultScriptNS, nil, nil)
	LoadScripts(center, GitRepoScriptNS, nil, nil)
	LoadScripts(center, RemoteScriptNS, nil, nil)

	_ = os.WriteFile("/tmp/nginx-time123.p", []byte(`
		json(_, time)
		set_tag(bb, "aa0")
		default_time(time)
		`), os.FileMode(0o755))
	whichStore(center, point.Logging).LoadDotPScript2Store(DefaultScriptNS, "", []string{"/tmp/nginx-time.p123"})
	_ = os.Remove("/tmp/nginx-time123.p")
}

func TestCmpCategory(t *testing.T) {
	cats := map[point.Category]struct{}{}
	for _, k := range point.AllCategories() {
		if k == point.DynamicDWCategory {
			continue
		}
		cats[k] = struct{}{}
	}

	assert.Equal(t, cats, func() map[point.Category]struct{} {
		ret := map[point.Category]struct{}{}
		for k := range CategoryDirName() {
			ret[k] = struct{}{}
		}
		return ret
	}())
}

func BenchmarkIndexMap(b *testing.B) {
	b.Run("sync.Map", func(b *testing.B) {
		type cachemap struct {
			m sync.Map
		}

		m := cachemap{}
		m.m.Store("abc.p", &PlScript{})
		m.m.Store("def.p", &PlScript{})

		var x1, x2, x3 *PlScript
		for i := 0; i < b.N; i++ {
			if v, ok := m.m.Load("abc.p"); ok {
				x1 = v.(*PlScript)
			}
			if v, ok := m.m.Load("def.p"); ok {
				x2 = v.(*PlScript)
			}
			if v, ok := m.m.Load("ddd"); ok {
				x3 = v.(*PlScript)
			}
		}
		b.Log(x1, x2, x3, false)
	})

	b.Run("map", func(b *testing.B) {
		type cachemap struct {
			m     map[string]*PlScript
			mlock sync.RWMutex
		}

		m := cachemap{
			m: map[string]*PlScript{
				"abc.p": {},
				"def.p": {},
			},
		}

		var x1, x2, x3 *PlScript
		var ok bool
		for i := 0; i < b.N; i++ {
			m.mlock.RLock()
			x1, ok = m.m["abc.p"]
			if !ok {
				b.Log()
			}
			m.mlock.RUnlock()

			m.mlock.RLock()
			x2, ok = m.m["def.p"]
			if !ok {
				b.Log()
			}
			m.mlock.RUnlock()

			m.mlock.RLock()
			x3, ok = m.m["ddd"]
			if ok {
				b.Log()
			}
			m.mlock.RUnlock()
		}
		b.Log(x1, x2, x3, ok)
	})
}

func TestPlScriptStore(t *testing.T) {
	store := NewScriptStore(point.Logging, ManagerCfg{})

	store.indexUpdate(nil)

	err := store.UpdateScriptsWithNS(DefaultScriptNS, map[string]string{"abc.p": "default_time(time) ;set_tag(a, \"1\")"}, nil)
	if err != nil {
		t.Error(err)
	}

	err = store.UpdateScriptsWithNS(DefaultScriptNS, map[string]string{"abc.p": "default_time(time)"}, nil)
	if err != nil {
		t.Error(err)
	}

	err = store.UpdateScriptsWithNS(DefaultScriptNS, map[string]string{"abc.p": "default_time(time); set_tag(a, 1)"}, nil)
	if err == nil {
		t.Error("should not be nil")
	}

	err = store.UpdateScriptsWithNS(DefaultScriptNS, map[string]string{"abc.p": "default_time(time)"}, nil)
	if err != nil {
		t.Error(err)
	}

	err = store.UpdateScriptsWithNS(GitRepoScriptNS, map[string]string{"abc.p": "default_time(time)"}, nil)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, store.Count(), 2)

	err = store.UpdateScriptsWithNS(ConfdScriptNS, map[string]string{"abc.p": "default_time(time)"}, nil)
	if err != nil {
		t.Error(err)
	}

	err = store.UpdateScriptsWithNS(RemoteScriptNS, map[string]string{"abc.p": "default_time(time)"}, nil)
	if err != nil {
		t.Error(err)
	}

	for i, ns := range plScriptNSSearchOrder {
		store.UpdateScriptsWithNS(ns, nil, nil)
		if i < len(plScriptNSSearchOrder)-1 {
			sInfo, ok := store.IndexGet("abc.p")
			if !ok {
				t.Error(fmt.Errorf("!ok"))
				return
			}
			if sInfo.ns != plScriptNSSearchOrder[i+1] {
				t.Error(sInfo.ns, plScriptNSSearchOrder[i+1])
			}
		} else {
			_, ok := store.IndexGet("abc.p")
			if ok {
				t.Error(fmt.Errorf("shoud not be ok"))
				return
			}
		}
	}
}

func TestPlDirStruct(t *testing.T) {
	bPath := fmt.Sprintf("/tmp/%d/pipeline/", time.Now().UnixNano())
	_ = os.MkdirAll(bPath, os.FileMode(0o755))

	expt := map[point.Category]map[string]string{}
	for category, dirName := range CategoryDirName() {
		if _, ok := expt[category]; !ok {
			expt[category] = map[string]string{}
		}
		expt[category][dirName+"-xx.p"] = filepath.Join(bPath, dirName, dirName+"-xx.p")
	}

	_ = os.WriteFile(filepath.Join(bPath, "nginx-xx.p"), []byte(`
	json(_, time)
	set_tag(bb, "aa0")
	default_time(time)
	`), os.FileMode(0o755))

	expt[point.Logging]["nginx-xx.p"] = filepath.Join(bPath, "nginx-xx.p")

	for _, dirName := range CategoryDirName() {
		_ = os.MkdirAll(filepath.Join(bPath, dirName), os.FileMode(0o755))
		_ = os.WriteFile(filepath.Join(bPath, dirName, dirName+"-xx.p"), []byte(`
		json(_, time)
		set_tag(bb, "aa0")
		default_time(time)
		`), os.FileMode(0o755))
	}
	act := SearchPlFilePathFromPlStructPath(bPath)

	assert.Equal(t, expt, act)
}