[![Go Reference](https://pkg.go.dev/badge/github.com/codeseasy/cron.svg)](https://pkg.go.dev/github.com/codeseasy/cron)

# cron

A cron expression parser and job scheduler for Go.

## Install

```bash
go get github.com/codeseasy/cron
```

```go
import "github.com/codeseasy/cron"
```

Requires Go 1.22 or later.

## Usage

```go
c := cron.New()
c.AddFunc("30 * * * *", func() { fmt.Println("Every hour on the half hour") })
c.AddFunc("@hourly",    func() { fmt.Println("Every hour") })
c.AddFunc("@every 1h30m", func() { fmt.Println("Every hour thirty") })
c.Start()

// Jobs run in their own goroutines, asynchronously.

// Add jobs to a running cron
c.AddFunc("@daily", func() { fmt.Println("Every day") })

// Inspect entries
inspect(c.Entries())

// Stop the scheduler (does not stop jobs already running)
c.Stop()
```

## Cron Expression Format

A cron expression represents a set of times, using 5 space-separated fields:

```
Field name   | Mandatory? | Allowed values  | Allowed special characters
----------   | ---------- | --------------  | --------------------------
Minutes      | Yes        | 0-59            | * / , -
Hours        | Yes        | 0-23            | * / , -
Day of month | Yes        | 1-31            | * / , - ?
Month        | Yes        | 1-12 or JAN-DEC | * / , -
Day of week  | Yes        | 0-7 or SUN-SAT  | * / , - ?
```

> **Note:** Day-of-week 7 is Sunday, same as 0 (per POSIX crontab convention). Both `0` and `7` represent Sunday.

Month and Day-of-week names are case insensitive (`SUN`, `Sun`, and `sun` are all accepted).

### Special Characters

| Character | Meaning |
|---|---|
| `*` | Match all values in the field |
| `/` | Step increment (e.g., `*/15` in minutes = every 15 minutes) |
| `,` | List separator (e.g., `MON,WED,FRI`) |
| `-` | Range (e.g., `9-17` = 9am through 5pm) |
| `?` | Same as `*` (can be used for day-of-month or day-of-week) |

**Step values** must not exceed the field's range. For example, `*/90` in the minutes field (range 0-59) is rejected because the step exceeds the field's range size.

### Predefined Schedules

```
Entry                  | Description                                | Equivalent To
-----                  | -----------                                | -------------
@yearly (or @annually) | Run once a year, midnight, Jan. 1st        | 0 0 1 1 *
@monthly               | Run once a month, midnight, first of month | 0 0 1 * *
@weekly                | Run once a week, midnight between Sat/Sun  | 0 0 * * 0
@daily (or @midnight)  | Run once a day, midnight                   | 0 0 * * *
@hourly                | Run once an hour, beginning of hour        | 0 * * * *
```

### Intervals

```go
// Run every 90 seconds
c.AddFunc("@every 1m30s", func() { ... })
```

`@every <duration>` schedules the job at a fixed interval, starting from when the cron is started. The duration string is parsed by Go's [`time.ParseDuration`](https://pkg.go.dev/time#ParseDuration). The duration must be positive (e.g., `@every 0s` and `@every -1m` are rejected).

> **Note:** The interval does not account for job runtime. If a job takes 3 minutes and is scheduled every 5 minutes, there will only be 2 minutes of idle time between runs.

### Seconds (Optional)

The standard 5-field format does not include seconds. To add a seconds field:

```go
// Seconds field, required (Quartz-compatible: sec min hour dom month dow)
cron.New(cron.WithSeconds())

// Seconds field, optional (accepts both 5-field and 6-field expressions)
cron.New(cron.WithParser(cron.NewParser(
    cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
)))
```

### Time Zones

```go
// Set the default time zone for the entire cron instance
cron.New(cron.WithLocation(time.UTC))

// Override per schedule using the CRON_TZ= or TZ= prefix
c.AddFunc("CRON_TZ=Asia/Tokyo 0 6 * * ?", ...)
c.AddFunc("TZ=America/New_York 0 9 * * MON-FRI", ...)
```

The default time zone is the machine's local time (`time.Local`).

> **Note:** Jobs scheduled during daylight-savings leap-ahead transitions will not be run.

### Job Wrappers

Job wrappers add cross-cutting behavior to jobs. **By default, `cron.New()` wraps all jobs with panic recovery** — any panic in a job is caught and logged instead of crashing the process.

```go
// Default behavior (panic recovery is automatic):
c := cron.New()

// Custom chain — replaces the default (no automatic recovery unless you include it):
cron.New(cron.WithChain(
    cron.Recover(logger),              // Catch panics and log them
    cron.SkipIfStillRunning(logger),   // Skip if the previous run is still in progress
))

// Opt out of all wrappers (panics will crash the process):
cron.New(cron.WithChain())
```

Available wrappers:

| Wrapper | Behavior |
|---|---|
| `Recover(logger)` | Catches panics and logs them (default) |
| `SkipIfStillRunning(logger)` | Skips the invocation if the previous one hasn't finished |
| `DelayIfStillRunning(logger)` | Delays (queues) the invocation until the previous one finishes |

You can also wrap individual jobs instead of all jobs:

```go
job = cron.NewChain(
    cron.SkipIfStillRunning(logger),
).Then(job)
```

### Thread Safety

All cron methods (`AddFunc`, `Remove`, `Entries`, `Start`, `Stop`) are safe to call from multiple goroutines.

## Origin

A maintained fork of [robfig/cron/v3](https://github.com/robfig/cron) — the most popular cron library for Go (13k+ stars). The original has been unmaintained since 2020 with 50+ unresolved issues. This fork keeps it alive with bug fixes, security patches, and modern Go support.

## License

MIT (same as original)
