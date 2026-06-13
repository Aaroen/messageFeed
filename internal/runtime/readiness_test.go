package runtime

import (
	"testing"
	"time"
)

// TestNewProcessReadinessReport 验证第一阶段的进程级 ready 报告。
func TestNewProcessReadinessReport(t *testing.T) {
	checkedAt := time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)

	report := NewProcessReadinessReport(checkedAt)

	if report.Status != ReadinessReady {
		t.Fatalf("Status = %q, want %q", report.Status, ReadinessReady)
	}
	if !report.Ready() {
		t.Fatal("Ready() = false, want true")
	}
	if got, want := len(report.Checks), 1; got != want {
		t.Fatalf("Checks length = %d, want %d", got, want)
	}
	if report.Checks[0].Name != "process" {
		t.Fatalf("process check name = %q", report.Checks[0].Name)
	}
	if !report.CheckedAt.Equal(checkedAt) {
		t.Fatalf("CheckedAt = %s, want %s", report.CheckedAt, checkedAt)
	}
}

// TestNewReadinessReportMarksNotReady 验证只要任一检查项未就绪，整体状态即为 not_ready。
func TestNewReadinessReportMarksNotReady(t *testing.T) {
	report := NewReadinessReport([]ReadinessCheck{
		{Name: "process", Status: ReadinessReady},
		{Name: "postgres", Status: ReadinessNotReady},
	}, time.Now())

	if report.Status != ReadinessNotReady {
		t.Fatalf("Status = %q, want %q", report.Status, ReadinessNotReady)
	}
	if report.Ready() {
		t.Fatal("Ready() = true, want false")
	}
}

// TestNewReadinessReportDefaultsCheckedAt 验证未传入检查时间时会生成非零 UTC 时间。
func TestNewReadinessReportDefaultsCheckedAt(t *testing.T) {
	report := NewReadinessReport(nil, time.Time{})

	if report.CheckedAt.IsZero() {
		t.Fatal("CheckedAt is zero")
	}
	if report.CheckedAt.Location() != time.UTC {
		t.Fatalf("CheckedAt location = %s, want UTC", report.CheckedAt.Location())
	}
}

// TestNewReadinessReportCopiesChecks 验证报告会复制检查项切片，避免调用方修改输入后污染报告。
func TestNewReadinessReportCopiesChecks(t *testing.T) {
	checks := []ReadinessCheck{{Name: "process", Status: ReadinessReady}}

	report := NewReadinessReport(checks, time.Now())
	checks[0].Status = ReadinessNotReady

	if report.Checks[0].Status != ReadinessReady {
		t.Fatalf("report check status = %q, want %q", report.Checks[0].Status, ReadinessReady)
	}
}
