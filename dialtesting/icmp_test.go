// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package dialtesting

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"text/template"
	"time"

	"github.com/stretchr/testify/assert"
)

var icmpCases = []struct {
	t         *ICMPTask
	fail      bool
	reasonCnt int
}{
	{
		fail:      false,
		reasonCnt: 0,
		t: &ICMPTask{
			Host:        "localhost",
			PacketCount: 5,
			SuccessWhen: []*ICMPSuccess{
				{
					ResponseTime: []*ResponseTimeSucess{
						{
							Func:   "avg",
							Op:     "lt",
							Target: "10ms",
						},
					},
				},
			},
			Task: &Task{
				ExternalID: "xxxx", Frequency: "10s", Name: "success-ipv4",
			},
		},
	},
	{
		fail:      false,
		reasonCnt: 0,
		t: &ICMPTask{
			Host:        "::1",
			PacketCount: 5,
			SuccessWhen: []*ICMPSuccess{
				{
					ResponseTime: []*ResponseTimeSucess{
						{
							Func:   "avg",
							Op:     "lt",
							Target: "10ms",
						},
					},
				},
			},
			Task: &Task{
				ExternalID: "xxxx", Frequency: "10s", Name: "success-ipv6",
			},
		},
	},
}

func TestIcmp(t *testing.T) {
	for _, c := range icmpCases {
		c.t.SetChild(c.t)
		if err := c.t.Check(); err != nil {
			if c.fail == false {
				t.Errorf("case: %s, failed: %s", c.t.Name, err)
			} else {
				t.Logf("expected: %s", err.Error())
			}
			continue
		}

		err := c.t.Run()
		if err != nil {
			if c.fail == false {
				t.Errorf("case %s failed: %s", c.t.Name, err)
			} else {
				t.Logf("expected: %s", err.Error())
			}
			continue
		}

		tags, fields := c.t.GetResults()

		t.Logf("ts: %+#v \n fs: %+#v \n ", tags, fields)

		reasons, _ := c.t.CheckResult()
		if len(reasons) != c.reasonCnt {
			t.Errorf("case %s expect %d reasons, but got %d reasons:\n\t%s",
				c.t.Name, c.reasonCnt, len(reasons), strings.Join(reasons, "\n\t"))
		} else if len(reasons) > 0 {
			t.Logf("case %s reasons:\n\t%s",
				c.t.Name, strings.Join(reasons, "\n\t"))
		}
	}
}

func TestICMPRenderTemplate(t *testing.T) {
	ct := &ICMPTask{
		Host: "{{host}}",
	}

	fm := template.FuncMap{
		"host": func() string {
			return "localhost"
		},
	}

	task, err := NewTask("", ct)
	assert.NoError(t, err)

	ct, ok := task.(*ICMPTask)
	assert.True(t, ok)

	assert.NoError(t, ct.renderTemplate(fm))
	assert.Equal(t, "localhost", ct.Host)
}

func TestDoPing(t *testing.T) {
	MaxICMPConcurrent = 1000 // max icmp concurrent, to avoid too many icmp packets at the same time
	concurrency := 5000
	var lossCnt atomic.Int32
	ipsChan := make(chan string, concurrency)
	var wg sync.WaitGroup
	go func() {
		for _, ip := range ips {
			ipsChan <- ip
		}
		close(ipsChan)
	}()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for ip := range ipsChan {
				start := time.Now()
				rtt, err := doPing(30*time.Second, ip)
				if rtt == 0 {
					lossCnt.Add(1)
				}
				if err != nil {
					fmt.Printf("ping %s failed: %s\n", ip, err)
				}
				fmt.Printf("ping %s, rtt: %v, ip: %s, cost: %v\n", ip, rtt, ip, time.Since(start))
			}
		}(i)
	}

	wg.Wait()

	fmt.Printf("loss cnt: %d/%d\n", lossCnt.Load(), len(ips))
}

func TestDoPingOnce(t *testing.T) {
	ip := "127.0.0.1"
	// ip := "37.152.148.44"
	start := time.Now()
	rtt, err := doPing(30*time.Second, ip)
	if err != nil {
		fmt.Printf("ping %s failed: %s\n", ip, err)
	}
	fmt.Printf("ping %s, rtt: %v, ip: %s, cost: %v\n", ip, rtt, ip, time.Since(start))
}

func TestProbeICMPOnce(t *testing.T) {
	// ip := "127.0.0.1"
	// ip := "37.152.148.44"
	ip := "37.152.148.45"
	start := time.Now()
	rtt, success := ProbeICMP(30*time.Second, ip)
	if !success {
		fmt.Printf("ping %s failed: %s\n", ip, "icmp failed")
		return
	}
	fmt.Printf("ping %s, rtt: %v, ip: %s, cost: %v\n", ip, rtt, ip, time.Since(start))
}

func TestProbeICMP(t *testing.T) {
	concurrency := 5000
	var lossCnt atomic.Int32
	ipsChan := make(chan string, concurrency)
	var wg sync.WaitGroup
	go func() {
		for {
			time.Sleep(time.Second)
			for _, ip := range ips {
				ipsChan <- ip
			}
		}
	}()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for ip := range ipsChan {
				start := time.Now()
				rtt, success := ProbeICMP(30*time.Second, ip)
				if rtt == 0 {
					lossCnt.Add(1)
				}
				if !success {
					fmt.Printf("ping %s failed: %s\n", ip, "icmp failed")
					continue
				}
				fmt.Printf("ping %s, rtt: %v, ip: %s, cost: %v\n", ip, rtt, ip, time.Since(start))
			}
		}(i)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			time.Sleep(time.Second)
			ProbeTest()
		}
	}()

	wg.Wait()

	fmt.Printf("loss cnt: %d/%d\n", lossCnt.Load(), len(ips))
}

func TestICMPProbeTest(t *testing.T) {
	for {
		time.Sleep(1 * time.Second)
		ProbeTest()
	}
}
