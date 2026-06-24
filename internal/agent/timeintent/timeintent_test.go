package timeintent

import (
	"testing"
	"time"
)

func TestParseChineseDateRange(t *testing.T) {
	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	now := time.Date(2026, 6, 24, 15, 0, 0, 0, location)
	result := Parse("查一下昨天上午的聊天", now, location)
	if !result.HasRange() {
		t.Fatalf("result = %#v, want range", result)
	}
	wantStart := time.Date(2026, 6, 23, 6, 0, 0, 0, location)
	wantEnd := time.Date(2026, 6, 23, 12, 0, 0, 0, location)
	if !result.StartAt.Equal(wantStart) || !result.EndAt.Equal(wantEnd) {
		t.Fatalf("range = %s - %s, want %s - %s", result.StartAt, result.EndAt, wantStart, wantEnd)
	}
}

func TestParseScheduleInstant(t *testing.T) {
	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	now := time.Date(2026, 6, 24, 15, 0, 0, 0, location)
	result := Parse("明天上午9点提醒我", now, location)
	if !result.HasInstant() {
		t.Fatalf("result = %#v, want instant", result)
	}
	want := time.Date(2026, 6, 25, 9, 0, 0, 0, location)
	if !result.InstantAt.Equal(want) {
		t.Fatalf("instant = %s, want %s", result.InstantAt, want)
	}
}

func TestParseExplicitDateTime(t *testing.T) {
	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	now := time.Date(2026, 6, 24, 15, 0, 0, 0, location)
	result := Parse("2026-06-25 18:30 发送消息", now, location)
	if !result.HasInstant() {
		t.Fatalf("result = %#v, want instant", result)
	}
	want := time.Date(2026, 6, 25, 18, 30, 0, 0, location)
	if !result.InstantAt.Equal(want) {
		t.Fatalf("instant = %s, want %s", result.InstantAt, want)
	}
}
