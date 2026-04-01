// Copyright 2026 CodesEasy. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package cron

import (
	"log"
	"strings"
	"testing"
	"time"
)

func TestWithLocation(t *testing.T) {
	c := New(WithLocation(time.UTC))
	if c.location != time.UTC {
		t.Errorf("expected UTC, got %v", c.location)
	}
}

func TestWithParser(t *testing.T) {
	var parser = NewParser(Dow)
	c := New(WithParser(parser))
	if c.parser != parser {
		t.Error("expected provided parser")
	}
}

func TestWithLocationNilPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil Location")
		}
	}()
	WithLocation(nil)
}

func TestWithParserNilPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil ScheduleParser")
		}
	}()
	WithParser(nil)
}

func TestWithLoggerNilPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil Logger")
		}
	}()
	WithLogger(nil)
}

func TestWithVerboseLogger(t *testing.T) {
	var buf syncWriter
	var logger = log.New(&buf, "", log.LstdFlags)
	c := New(WithLogger(VerbosePrintfLogger(logger)))
	if c.logger.(printfLogger).logger != logger {
		t.Error("expected provided logger")
	}

	c.AddFunc("@every 1s", func() {})
	c.Start()
	time.Sleep(OneSecond)
	c.Stop()
	out := buf.String()
	if !strings.Contains(out, "schedule,") ||
		!strings.Contains(out, "run,") {
		t.Error("expected to see some actions, got:", out)
	}
}
