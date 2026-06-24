package timeintent

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Kind string

const (
	KindNone      Kind = "none"
	KindRange     Kind = "range"
	KindInstant   Kind = "instant"
	KindAmbiguous Kind = "ambiguous"
)

type Result struct {
	Kind       Kind
	StartAt    time.Time
	EndAt      time.Time
	InstantAt  time.Time
	TimeZone   string
	Confidence string
	Matched    string
}

func (r Result) HasRange() bool {
	return r.Kind == KindRange && !r.StartAt.IsZero() && !r.EndAt.IsZero() && r.EndAt.After(r.StartAt)
}

func (r Result) HasInstant() bool {
	return r.Kind == KindInstant && !r.InstantAt.IsZero()
}

func Parse(text string, now time.Time, location *time.Location) Result {
	text = strings.TrimSpace(text)
	if text == "" {
		return Result{Kind: KindNone}
	}
	if location == nil {
		location = time.Local
	}
	if now.IsZero() {
		now = time.Now()
	}
	now = now.In(location)
	date, dateMatched, ok := parseDate(text, now, location)
	if !ok {
		return Result{Kind: KindNone, TimeZone: location.String()}
	}
	hour, minute, hasClock := parseClock(text)
	partStart, partEnd, hasPart := dayPart(text)
	result := Result{
		Kind:       KindRange,
		TimeZone:   location.String(),
		Confidence: "explicit",
		Matched:    strings.TrimSpace(dateMatched),
	}
	if hasClock {
		result.Kind = KindInstant
		result.InstantAt = time.Date(date.Year(), date.Month(), date.Day(), hour, minute, 0, 0, location)
		if !result.InstantAt.After(now) && isFutureExpression(text) {
			result.InstantAt = result.InstantAt.AddDate(0, 0, 1)
		}
		return result
	}
	if hasPart {
		result.StartAt = time.Date(date.Year(), date.Month(), date.Day(), partStart, 0, 0, 0, location)
		result.EndAt = time.Date(date.Year(), date.Month(), date.Day(), partEnd, 0, 0, 0, location)
		return result
	}
	result.StartAt = startOfDay(date, location)
	result.EndAt = result.StartAt.AddDate(0, 0, 1)
	return result
}

func parseDate(text string, now time.Time, location *time.Location) (time.Time, string, bool) {
	if matched := regexp.MustCompile(`(\d{4})[-/年](\d{1,2})[-/月](\d{1,2})日?`).FindStringSubmatch(text); len(matched) == 4 {
		year, _ := strconv.Atoi(matched[1])
		month, _ := strconv.Atoi(matched[2])
		day, _ := strconv.Atoi(matched[3])
		return time.Date(year, time.Month(month), day, 0, 0, 0, 0, location), matched[0], true
	}
	if matched := regexp.MustCompile(`(\d{1,2})月(\d{1,2})日?`).FindStringSubmatch(text); len(matched) == 3 {
		month, _ := strconv.Atoi(matched[1])
		day, _ := strconv.Atoi(matched[2])
		return time.Date(now.Year(), time.Month(month), day, 0, 0, 0, 0, location), matched[0], true
	}
	relative := []struct {
		term string
		days int
	}{
		{"前天", -2},
		{"昨天", -1},
		{"今日", 0},
		{"今天", 0},
		{"明天", 1},
		{"后天", 2},
	}
	for _, item := range relative {
		if strings.Contains(text, item.term) {
			return startOfDay(now.AddDate(0, 0, item.days), location), item.term, true
		}
	}
	if strings.Contains(text, "上周") {
		start := startOfWeek(now, location).AddDate(0, 0, -7)
		return start, "上周", true
	}
	if strings.Contains(text, "本周") || strings.Contains(text, "这周") {
		return startOfWeek(now, location), "本周", true
	}
	if strings.Contains(text, "下周") {
		start := startOfWeek(now, location).AddDate(0, 0, 7)
		weekday, ok := parseChineseWeekday(text)
		if ok {
			return start.AddDate(0, 0, weekday), "下周" + weekdayName(weekday), true
		}
		return start, "下周", true
	}
	if strings.Contains(text, "本月") || strings.Contains(text, "这个月") {
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, location), "本月", true
	}
	return time.Time{}, "", false
}

func parseClock(text string) (int, int, bool) {
	matched := regexp.MustCompile(`(\d{1,2})[:：](\d{1,2})`).FindStringSubmatch(text)
	if len(matched) == 3 {
		hour, _ := strconv.Atoi(matched[1])
		minute, _ := strconv.Atoi(matched[2])
		return normalizeHour(text, hour), minute, true
	}
	matched = regexp.MustCompile(`(\d{1,2})点半`).FindStringSubmatch(text)
	if len(matched) == 2 {
		hour, _ := strconv.Atoi(matched[1])
		return normalizeHour(text, hour), 30, true
	}
	matched = regexp.MustCompile(`(\d{1,2})点`).FindStringSubmatch(text)
	if len(matched) == 2 {
		hour, _ := strconv.Atoi(matched[1])
		return normalizeHour(text, hour), 0, true
	}
	return 0, 0, false
}

func normalizeHour(text string, hour int) int {
	if hour < 12 && (strings.Contains(text, "下午") || strings.Contains(text, "晚上") || strings.Contains(text, "晚间")) {
		return hour + 12
	}
	if hour == 12 && (strings.Contains(text, "凌晨") || strings.Contains(text, "早上")) {
		return 0
	}
	return hour
}

func dayPart(text string) (int, int, bool) {
	switch {
	case strings.Contains(text, "凌晨"):
		return 0, 6, true
	case strings.Contains(text, "早上") || strings.Contains(text, "上午"):
		return 6, 12, true
	case strings.Contains(text, "中午"):
		return 11, 14, true
	case strings.Contains(text, "下午"):
		return 12, 18, true
	case strings.Contains(text, "晚上") || strings.Contains(text, "晚间"):
		return 18, 24, true
	default:
		return 0, 0, false
	}
}

func startOfDay(value time.Time, location *time.Location) time.Time {
	value = value.In(location)
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, location)
}

func startOfWeek(value time.Time, location *time.Location) time.Time {
	day := startOfDay(value, location)
	weekday := int(day.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return day.AddDate(0, 0, 1-weekday)
}

func parseChineseWeekday(text string) (int, bool) {
	for index, term := range []string{"一", "二", "三", "四", "五", "六"} {
		if strings.Contains(text, "周"+term) || strings.Contains(text, "星期"+term) {
			return index, true
		}
	}
	if strings.Contains(text, "周日") || strings.Contains(text, "周天") || strings.Contains(text, "星期日") || strings.Contains(text, "星期天") {
		return 6, true
	}
	return 0, false
}

func weekdayName(offset int) string {
	names := []string{"一", "二", "三", "四", "五", "六", "日"}
	if offset < 0 || offset >= len(names) {
		return fmt.Sprintf("%d", offset)
	}
	return names[offset]
}

func isFutureExpression(text string) bool {
	return strings.Contains(text, "明天") || strings.Contains(text, "后天") || strings.Contains(text, "下周")
}
