package cron

import (
	"strings"
	"testing"
	"time"
)

func FuzzParseStandard(f *testing.F) {
	// Seed corpus: valid specs
	f.Add("* * * * *")
	f.Add("5 * * * *")
	f.Add("*/15 * * * *")
	f.Add("0 0 * * 0")
	f.Add("0 0 1 1 *")
	f.Add("@hourly")
	f.Add("@daily")
	f.Add("@weekly")
	f.Add("@monthly")
	f.Add("@yearly")
	f.Add("@annually")
	f.Add("@every 5m")
	f.Add("@midnight")

	// TZ prefixes
	f.Add("TZ=UTC * * * * *")
	f.Add("CRON_TZ=Asia/Tokyo 0 6 * * ?")

	// Edge cases and known-panic inputs
	f.Add("TZ=0")
	f.Add("TZ=")
	f.Add("CRON_TZ=Asia/Tokyo")
	f.Add("TZ=UTC ")
	f.Add("TZ=  ")
	f.Add("*/90 * * * *")
	f.Add("*/60 * * * *")
	f.Add("* * * * 7")
	f.Add("* * * * 0-7")
	f.Add("")
	f.Add(strings.Repeat("*", MaxSpecLength+1))

	f.Fuzz(func(t *testing.T, spec string) {
		standardParser.Parse(spec)
	})
}

func FuzzParseWithSeconds(f *testing.F) {
	parser := NewParser(Second | Minute | Hour | Dom | Month | Dow | Descriptor)

	f.Add("0 * * * * *")
	f.Add("*/5 * * * * *")
	f.Add("0 0 0 * * *")
	f.Add("0 0 0 1 1 0")
	f.Add("@hourly")
	f.Add("@every 1s")
	f.Add("TZ=UTC 0 * * * * *")

	f.Fuzz(func(t *testing.T, spec string) {
		parser.Parse(spec)
	})
}

func FuzzParseOptionalSeconds(f *testing.F) {
	parser := NewParser(SecondOptional | Minute | Hour | Dom | Month | Dow | Descriptor)

	f.Add("* * * * *")
	f.Add("0 * * * * *")
	f.Add("5 5 * * * *")
	f.Add("@hourly")

	f.Fuzz(func(t *testing.T, spec string) {
		parser.Parse(spec)
	})
}

func FuzzScheduleNext(f *testing.F) {
	f.Add("* * * * *", int64(0))
	f.Add("0 0 1 1 *", int64(1609459200))
	f.Add("@hourly", int64(1700000000))
	f.Add("*/5 * * * *", int64(-62135596800))

	f.Fuzz(func(t *testing.T, spec string, unixSec int64) {
		sched, err := standardParser.Parse(spec)
		if err != nil {
			return
		}
		// Clamp to reasonable range to avoid extremely slow Next() searches
		if unixSec < -62135596800 || unixSec > 253402300800 {
			return
		}
		tm := time.Unix(unixSec, 0).UTC()
		sched.Next(tm)
	})
}
