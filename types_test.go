package thingscloud

import (
	"encoding/json"
	"testing"
	"time"
)

func TestBoolean_UnmarshalJSON(t *testing.T) {
	testCases := []struct {
		JSON     string
		Expected bool
	}{
		{"1", true},
		{"0", false},
	}
	for _, testCase := range testCases {
		bs := []byte(testCase.JSON)
		var b Boolean
		if err := json.Unmarshal(bs, &b); err != nil {
			t.Fatal(err.Error())
		}
		if bool(b) != testCase.Expected {
			t.Fatalf("Expected %t but got %t", testCase.Expected, b)
		}
	}
}

func TestBoolean_MarshalJSON(t *testing.T) {
	testCases := []struct {
		Value    bool
		Expected string
	}{
		{true, "1"},
		{false, "0"},
	}
	for _, testCase := range testCases {
		b := Boolean(testCase.Value)
		bb := &b
		bs, err := bb.MarshalJSON()
		if err != nil {
			t.Fatal(err.Error())
		}
		if string(bs) != testCase.Expected {
			t.Fatalf("Expected %q but got %q", testCase.Expected, string(bs))
		}
	}
}

func TestTaskType_Values(t *testing.T) {
	if TaskTypeTask != 0 {
		t.Errorf("expected TaskTypeTask=0, got %d", TaskTypeTask)
	}
	if TaskTypeProject != 1 {
		t.Errorf("expected TaskTypeProject=1, got %d", TaskTypeProject)
	}
	if TaskTypeHeading != 2 {
		t.Errorf("expected TaskTypeHeading=2, got %d", TaskTypeHeading)
	}
}

func TestTaskType_JSONRoundTrip(t *testing.T) {
	type wrapper struct {
		TP *TaskType `json:"tp"`
	}
	tp := TaskTypeHeading
	w := wrapper{TP: &tp}
	bs, err := json.Marshal(w)
	if err != nil {
		t.Fatal(err)
	}
	if string(bs) != `{"tp":2}` {
		t.Errorf("expected {\"tp\":2}, got %s", string(bs))
	}
	var w2 wrapper
	if err := json.Unmarshal(bs, &w2); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if *w2.TP != TaskTypeHeading {
		t.Errorf("expected TaskTypeHeading, got %d", *w2.TP)
	}
}

func TestTimestamp_UnmarshalJSON(t *testing.T) {
	testCases := []struct {
		JSON     string
		Expected string
	}{
		{"1496001956.2693141", "2017-05-28T20:05:17"},
		{"1496001956", "2017-05-28T20:05:17"},
	}
	for _, testCase := range testCases {
		bs := []byte(testCase.JSON)
		var tt Timestamp
		if err := json.Unmarshal(bs, &tt); err != nil {
			t.Fatal(err.Error())
		}
		if tt.Format("2006-01-02T15:04:06") != testCase.Expected {
			t.Fatalf("Expected %q but got %q", testCase.Expected, tt.Format("2006-01-02T15:04:06"))
		}
	}
}

func TestTaskActionItemPayload_AllFields(t *testing.T) {
	raw := `{
		"tt":"test","tp":0,"st":1,"ss":0,
		"sr":1770681600,"tir":1770681600,"dd":1771027200,"dds":null,
		"tr":false,"sp":null,"cd":1770713623.47,"md":1770713627.59,
		"ix":-346833,"ti":0,"do":0,"lt":false,"icp":false,"icc":0,
		"icsd":null,"sb":0,"ato":39600,"rmd":null,"acrd":null,
		"dl":[],"lai":null,
		"nt":{"_t":"tx","t":1,"ch":0,"v":""},
		"tg":[],"ar":[],"pr":[],"agr":[],"rt":[],"rr":null,
		"xx":{"sn":{},"_t":"oo"}
	}`
	var p TaskActionItemPayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if p.DueOrder == nil || *p.DueOrder != 0 {
		t.Error("expected DueOrder=0")
	}
	if p.AlarmTimeOffset == nil || *p.AlarmTimeOffset != 39600 {
		t.Error("expected AlarmTimeOffset=39600")
	}
	if p.Leavable == nil || *p.Leavable != false {
		t.Error("expected Leavable=false")
	}
	if p.SubtaskBehavior == nil || *p.SubtaskBehavior != 0 {
		t.Error("expected SubtaskBehavior=0")
	}
	if p.ExtensionData == nil {
		t.Error("expected ExtensionData to be set")
	}
}

func TestCheckListActionItemPayload_ExtensionData(t *testing.T) {
	raw := `{"tt":"item","ix":0,"cd":1770713708.70,"md":1770713711.01,
		"ss":0,"sp":null,"lt":false,"ts":["abc"],"xx":{"sn":{},"_t":"oo"}}`
	var p CheckListActionItemPayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if p.Leavable == nil || *p.Leavable != false {
		t.Error("expected Leavable=false")
	}
	if p.ExtensionData == nil {
		t.Error("expected ExtensionData to be set")
	}
}

func TestTagActionItemPayload_ExtensionData(t *testing.T) {
	raw := `{"tt":"tag","ix":0,"pn":[],"sh":null,"xx":{"sn":{},"_t":"oo"}}`
	var p TagActionItemPayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if p.ExtensionData == nil {
		t.Error("expected ExtensionData to be set")
	}
}

func TestTimestamp_MarshalJSON(t *testing.T) {
	testCases := []struct {
		Time     time.Time
		Expected string
	}{
		// Whole-second timestamps serialize as integers (no trailing .0)
		{time.Date(2017, time.May, 28, 22, 05, 17, 0, time.UTC), "1496009117"},
		// Sub-second precision is preserved as fractional epoch
		{time.Unix(1496009117, 500000000), "1496009117.5"},
	}
	for _, testCase := range testCases {

		tt := Timestamp(testCase.Time)
		ttt := &tt
		bs, err := ttt.MarshalJSON()
		if err != nil {
			t.Fatal(err.Error())
		}
		if string(bs) != testCase.Expected {
			t.Fatalf("Expected %q but got %q", testCase.Expected, string(bs))
		}
	}
}
