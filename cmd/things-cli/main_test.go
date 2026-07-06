package main

import (
	"encoding/json"
	"testing"
)

func requirePayloadMap(t *testing.T, env any) map[string]any {
	t.Helper()
	envelope, ok := env.(writeEnvelope)
	if !ok {
		t.Fatalf("expected writeEnvelope, got %T", env)
	}
	payload, ok := envelope.payload.(map[string]any)
	if !ok {
		t.Fatalf("expected map payload, got %T", envelope.payload)
	}
	return payload
}

func assertAnytimeSchedule(t *testing.T, payload map[string]any) {
	t.Helper()
	if payload["st"] != 1 {
		t.Fatalf("st = %v, want 1", payload["st"])
	}
	if payload["sr"] != nil {
		t.Fatalf("sr = %v, want nil", payload["sr"])
	}
	if payload["tir"] != nil {
		t.Fatalf("tir = %v, want nil", payload["tir"])
	}
}

func TestTaskUpdateAnytimeClearsScheduleDates(t *testing.T) {
	payload := newTaskUpdate().Project("project-1").Anytime().build()

	assertAnytimeSchedule(t, payload)
	if got := payload["pr"]; got == nil {
		t.Fatal("project field was not set")
	}
}

func TestHasExplicitSchedule(t *testing.T) {
	tests := []struct {
		name string
		opts map[string]string
		want bool
	}{
		{
			name: "none",
			opts: map[string]string{},
			want: false,
		},
		{
			name: "when",
			opts: map[string]string{"when": "today"},
			want: true,
		},
		{
			name: "scheduled",
			opts: map[string]string{"scheduled": "2026-05-20"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasExplicitSchedule(tt.opts); got != tt.want {
				t.Fatalf("hasExplicitSchedule() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBatchMoveToProjectUsesNullScheduleDates(t *testing.T) {
	env, _, err := buildBatchMoveToProject(BatchOp{
		UUID:    "task-1",
		Project: "project-1",
	})
	if err != nil {
		t.Fatalf("buildBatchMoveToProject failed: %v", err)
	}

	payload := requirePayloadMap(t, env)
	assertAnytimeSchedule(t, payload)

	bs, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}
	var wire struct {
		P map[string]any `json:"p"`
	}
	if err := json.Unmarshal(bs, &wire); err != nil {
		t.Fatalf("unmarshal wire payload failed: %v", err)
	}
	if wire.P["sr"] != nil {
		t.Fatalf("wire sr = %v, want null", wire.P["sr"])
	}
	if wire.P["tir"] != nil {
		t.Fatalf("wire tir = %v, want null", wire.P["tir"])
	}
}

func TestBatchMoveToAreaUsesNullScheduleDates(t *testing.T) {
	env, _, err := buildBatchMoveToArea(BatchOp{
		UUID: "task-1",
		Area: "area-1",
	})
	if err != nil {
		t.Fatalf("buildBatchMoveToArea failed: %v", err)
	}

	assertAnytimeSchedule(t, requirePayloadMap(t, env))
}

func TestBatchEditAutoAnytimeUsesNullScheduleDates(t *testing.T) {
	tests := []struct {
		name string
		op   BatchOp
	}{
		{
			name: "project",
			op:   BatchOp{UUID: "task-1", Project: "project-1"},
		},
		{
			name: "area",
			op:   BatchOp{UUID: "task-1", Area: "area-1"},
		},
		{
			name: "heading",
			op:   BatchOp{UUID: "task-1", Heading: "heading-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, _, err := buildBatchEdit(tt.op)
			if err != nil {
				t.Fatalf("buildBatchEdit failed: %v", err)
			}
			assertAnytimeSchedule(t, requirePayloadMap(t, env))
		})
	}
}

func TestBatchEditExplicitWhenWinsOverAutoAnytime(t *testing.T) {
	env, _, err := buildBatchEdit(BatchOp{
		UUID:    "task-1",
		Project: "project-1",
		When:    "someday",
	})
	if err != nil {
		t.Fatalf("buildBatchEdit failed: %v", err)
	}

	payload := requirePayloadMap(t, env)
	if payload["st"] != 2 {
		t.Fatalf("st = %v, want 2", payload["st"])
	}
	if payload["sr"] != nil {
		t.Fatalf("sr = %v, want nil", payload["sr"])
	}
	if payload["tir"] != nil {
		t.Fatalf("tir = %v, want nil", payload["tir"])
	}
}
