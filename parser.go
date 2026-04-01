// Copyright 2026 CodesEasy. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package cron

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// MaxSpecLength is the maximum allowed length for a cron spec string.
const MaxSpecLength = 1024

// Configuration options for creating a parser. Most options specify which
// fields should be included, while others enable features. If a field is not
// included the parser will assume a default value. These options do not change
// the order fields are parse in.
type ParseOption int

const (
	Second         ParseOption = 1 << iota // Seconds field, default 0
	SecondOptional                         // Optional seconds field, default 0
	Minute                                 // Minutes field, default 0
	Hour                                   // Hours field, default 0
	Dom                                    // Day of month field, default *
	Month                                  // Month field, default *
	Dow                                    // Day of week field, default *
	DowOptional                            // Optional day of week field, default *
	Descriptor                             // Allow descriptors such as @monthly, @weekly, etc.
)

var places = []ParseOption{
	Second,
	Minute,
	Hour,
	Dom,
	Month,
	Dow,
}

var defaults = []string{
	"0",
	"0",
	"0",
	"*",
	"*",
	"*",
}

// A custom Parser that can be configured.
type Parser struct {
	options ParseOption
}

// NewParser creates a Parser with custom options.
//
// It panics if more than one Optional is given, since it would be impossible to
// correctly infer which optional is provided or missing in general.
//
// Examples
//
//	// Standard parser without descriptors
//	specParser := NewParser(Minute | Hour | Dom | Month | Dow)
//	sched, err := specParser.Parse("0 0 15 */3 *")
//
//	// Same as above, just excludes time fields
//	specParser := NewParser(Dom | Month | Dow)
//	sched, err := specParser.Parse("15 */3 *")
//
//	// Same as above, just makes Dow optional
//	specParser := NewParser(Dom | Month | DowOptional)
//	sched, err := specParser.Parse("15 */3")
func NewParser(options ParseOption) Parser {
	optionals := 0
	if options&DowOptional > 0 {
		optionals++
	}
	if options&SecondOptional > 0 {
		optionals++
	}
	if optionals > 1 {
		panic("multiple optionals may not be configured")
	}
	hasField := false
	for _, place := range places {
		if options&place > 0 {
			hasField = true
			break
		}
	}
	if !hasField && options&Descriptor == 0 {
		panic("at least one field or Descriptor must be configured")
	}
	return Parser{options}
}

// Parse returns a new crontab schedule representing the given spec.
// It returns a descriptive error if the spec is not valid.
// It accepts crontab specs and features configured by NewParser.
func (p Parser) Parse(spec string) (Schedule, error) {
	if len(spec) == 0 {
		return nil, fmt.Errorf("empty spec string")
	}
	if len(spec) > MaxSpecLength {
		return nil, fmt.Errorf("spec too long: %d > %d", len(spec), MaxSpecLength)
	}

	// Extract timezone if present
	var loc = time.Local
	originalSpec := spec
	if strings.HasPrefix(spec, "TZ=") || strings.HasPrefix(spec, "CRON_TZ=") {
		var err error
		i := strings.IndexAny(spec, " \t")
		if i == -1 {
			return nil, fmt.Errorf("missing fields after timezone in spec %q", originalSpec)
		}
		eq := strings.Index(spec, "=")
		tzName := spec[eq+1 : i]
		if tzName == "" {
			return nil, fmt.Errorf("empty timezone in spec %q", originalSpec)
		}
		if loc, err = time.LoadLocation(tzName); err != nil {
			return nil, fmt.Errorf("provided bad location %s: %v", tzName, err)
		}
		spec = strings.TrimSpace(spec[i:])
		if spec == "" {
			return nil, fmt.Errorf("missing fields after timezone in spec %q", originalSpec)
		}
	}

	// Handle named schedules (descriptors), if configured
	if strings.HasPrefix(spec, "@") {
		if p.options&Descriptor == 0 {
			return nil, fmt.Errorf("parser does not accept descriptors: %v", spec)
		}
		return parseDescriptor(spec, loc)
	}

	// Split on whitespace.
	fields := strings.Fields(spec)

	// Validate & fill in any omitted or optional fields
	var err error
	fields, err = normalizeFields(fields, p.options)
	if err != nil {
		return nil, err
	}

	field := func(field string, r bounds) uint64 {
		if err != nil {
			return 0
		}
		var bits uint64
		bits, err = getField(field, r)
		return bits
	}

	var (
		second     = field(fields[0], seconds)
		minute     = field(fields[1], minutes)
		hour       = field(fields[2], hours)
		dayofmonth = field(fields[3], dom)
		month      = field(fields[4], months)
		dayofweek  = normalizeDOW(field(fields[5], dow))
	)
	if err != nil {
		return nil, err
	}

	return &SpecSchedule{
		Second:   second,
		Minute:   minute,
		Hour:     hour,
		Dom:      dayofmonth,
		Month:    month,
		Dow:      dayofweek,
		Location: loc,
	}, nil
}

// normalizeFields takes a subset set of the time fields and returns the full set
// with defaults (zeroes) populated for unset fields.
//
// As part of performing this function, it also validates that the provided
// fields are compatible with the configured options.
func normalizeFields(fields []string, options ParseOption) ([]string, error) {
	// Validate optionals & add their field to options
	optionals := 0
	if options&SecondOptional > 0 {
		options |= Second
		optionals++
	}
	if options&DowOptional > 0 {
		options |= Dow
		optionals++
	}
	if optionals > 1 {
		return nil, fmt.Errorf("multiple optionals may not be configured")
	}

	// Figure out how many fields we need
	max := 0
	for _, place := range places {
		if options&place > 0 {
			max++
		}
	}
	min := max - optionals

	// Validate number of fields
	if count := len(fields); count < min || count > max {
		if min == max {
			return nil, fmt.Errorf("expected exactly %d fields, found %d: %s", min, count, fields)
		}
		return nil, fmt.Errorf("expected %d to %d fields, found %d: %s", min, max, count, fields)
	}

	// Populate the optional field if not provided
	if min < max && len(fields) == min {
		switch {
		case options&DowOptional > 0:
			fields = append(fields, defaultFor(Dow))
		case options&SecondOptional > 0:
			fields = append([]string{defaultFor(Second)}, fields...)
		default:
			return nil, fmt.Errorf("unknown optional field")
		}
	}

	// Populate all fields not part of options with their defaults
	n := 0
	expandedFields := make([]string, len(places))
	copy(expandedFields, defaults)
	for i, place := range places {
		if options&place > 0 {
			expandedFields[i] = fields[n]
			n++
		}
	}
	return expandedFields, nil
}

// defaultFor returns the default value for a given field.
func defaultFor(field ParseOption) string {
	for i, place := range places {
		if place == field {
			return defaults[i]
		}
	}
	return "*"
}

var standardParser = NewParser(
	Minute | Hour | Dom | Month | Dow | Descriptor,
)

// ParseStandard returns a new crontab schedule representing the given
// standardSpec (https://en.wikipedia.org/wiki/Cron). It requires 5 entries
// representing: minute, hour, day of month, month and day of week, in that
// order. It returns a descriptive error if the spec is not valid.
//
// It accepts
//   - Standard crontab specs, e.g. "* * * * ?"
//   - Descriptors, e.g. "@midnight", "@every 1h30m"
func ParseStandard(standardSpec string) (Schedule, error) {
	return standardParser.Parse(standardSpec)
}

// ValidateSpec checks whether spec is a valid standard cron expression.
// It returns nil if valid, or an error describing the problem.
func ValidateSpec(spec string) error {
	_, err := standardParser.Parse(spec)
	return err
}

// getField returns an Int with the bits set representing all of the times that
// the field represents or error parsing field value.  A "field" is a comma-separated
// list of "ranges".
func getField(field string, r bounds) (uint64, error) {
	var bits uint64
	ranges := strings.FieldsFunc(field, func(r rune) bool { return r == ',' })
	for _, expr := range ranges {
		bit, err := getRange(expr, r)
		if err != nil {
			return bits, err
		}
		bits |= bit
	}
	return bits, nil
}

// parseRangeBounds parses the start, end, and star-bit from the range portion
// of a cron field expression (e.g. "*", "1-5", "3"). The expr parameter is
// used only for error messages.
func parseRangeBounds(expr string, lowAndHigh []string, r bounds) (start, end uint, extra uint64, err error) {
	if lowAndHigh[0] == "*" || lowAndHigh[0] == "?" {
		return r.min, r.max, starBit, nil
	}
	start, err = parseIntOrName(lowAndHigh[0], r.names)
	if err != nil {
		return 0, 0, 0, err
	}
	switch len(lowAndHigh) {
	case 1:
		end = start
	case 2:
		end, err = parseIntOrName(lowAndHigh[1], r.names)
		if err != nil {
			return 0, 0, 0, err
		}
	default:
		return 0, 0, 0, fmt.Errorf("too many hyphens: %s", expr)
	}
	return start, end, 0, nil
}

// parseStep parses the step value from the range expression and adjusts end
// and extra for the "N/step" shorthand. The expr parameter is used only for
// error messages.
func parseStep(expr string, rangeAndStep []string, singleDigit bool, end uint, extra uint64, r bounds) (step, newEnd uint, newExtra uint64, err error) {
	switch len(rangeAndStep) {
	case 1:
		return 1, end, extra, nil
	case 2:
		step, err = mustParseInt(rangeAndStep[1])
		if err != nil {
			return 0, 0, 0, err
		}

		// Special handling: "N/step" means "N-max/step".
		if singleDigit {
			end = r.max
		}
		if step > 1 {
			extra = 0
		}
		return step, end, extra, nil
	default:
		return 0, 0, 0, fmt.Errorf("too many slashes: %s", expr)
	}
}

// getRange returns the bits indicated by the given expression:
//
//	number | number "-" number [ "/" number ]
//
// or error parsing range.
func getRange(expr string, r bounds) (uint64, error) {
	rangeAndStep := strings.Split(expr, "/")
	lowAndHigh := strings.Split(rangeAndStep[0], "-")
	singleDigit := len(lowAndHigh) == 1

	start, end, extra, err := parseRangeBounds(expr, lowAndHigh, r)
	if err != nil {
		return 0, err
	}

	step, end, extra, err := parseStep(expr, rangeAndStep, singleDigit, end, extra, r)
	if err != nil {
		return 0, err
	}

	if start < r.min {
		return 0, fmt.Errorf("beginning of range (%d) below minimum (%d): %s", start, r.min, expr)
	}
	if end > r.max {
		return 0, fmt.Errorf("end of range (%d) above maximum (%d): %s", end, r.max, expr)
	}
	if start > end {
		return 0, fmt.Errorf("beginning of range (%d) beyond end of range (%d): %s", start, end, expr)
	}
	if step == 0 {
		return 0, fmt.Errorf("step of range should be a positive number: %s", expr)
	}
	if rangeSize := end - start + 1; step > 1 && step > rangeSize {
		return 0, fmt.Errorf("step (%d) exceeds range size (%d): %s", step, rangeSize, expr)
	}

	return getBits(start, end, step) | extra, nil
}

// parseIntOrName returns the (possibly-named) integer contained in expr.
func parseIntOrName(expr string, names map[string]uint) (uint, error) {
	if names != nil {
		if namedInt, ok := names[strings.ToLower(expr)]; ok {
			return namedInt, nil
		}
	}
	return mustParseInt(expr)
}

// mustParseInt parses the given expression as an int or returns an error.
func mustParseInt(expr string) (uint, error) {
	num, err := strconv.Atoi(expr)
	if err != nil {
		return 0, fmt.Errorf("failed to parse int from %s: %s", expr, err)
	}
	if num < 0 {
		return 0, fmt.Errorf("negative number (%d) not allowed: %s", num, expr)
	}

	return uint(num), nil
}

// getBits sets all bits in the range [min, max], modulo the given step size.
func getBits(min, max, step uint) uint64 {
	var bits uint64

	// If step is 1, use shifts.
	if step == 1 {
		return ^(math.MaxUint64 << (max + 1)) & (math.MaxUint64 << min)
	}

	// Else, use a simple loop.
	for i := min; i <= max; i += step {
		bits |= 1 << i
	}
	return bits
}

// all returns all bits within the given bounds.  (plus the star bit)
func all(r bounds) uint64 {
	return getBits(r.min, r.max, 1) | starBit
}

// normalizeDOW maps Sunday=7 to Sunday=0 so that the schedule only uses bits 0-6.
func normalizeDOW(bits uint64) uint64 {
	if bits&(1<<7) != 0 {
		bits |= 1 << 0
		bits &^= 1 << 7
	}
	return bits
}

// parseDescriptor returns a predefined schedule for the expression, or error if none matches.
func parseDescriptor(descriptor string, loc *time.Location) (Schedule, error) {
	switch descriptor {
	case "@yearly", "@annually":
		return &SpecSchedule{
			Second:   1 << seconds.min,
			Minute:   1 << minutes.min,
			Hour:     1 << hours.min,
			Dom:      1 << dom.min,
			Month:    1 << months.min,
			Dow:      normalizeDOW(all(dow)),
			Location: loc,
		}, nil

	case "@monthly":
		return &SpecSchedule{
			Second:   1 << seconds.min,
			Minute:   1 << minutes.min,
			Hour:     1 << hours.min,
			Dom:      1 << dom.min,
			Month:    all(months),
			Dow:      normalizeDOW(all(dow)),
			Location: loc,
		}, nil

	case "@weekly":
		return &SpecSchedule{
			Second:   1 << seconds.min,
			Minute:   1 << minutes.min,
			Hour:     1 << hours.min,
			Dom:      all(dom),
			Month:    all(months),
			Dow:      1 << dow.min,
			Location: loc,
		}, nil

	case "@daily", "@midnight":
		return &SpecSchedule{
			Second:   1 << seconds.min,
			Minute:   1 << minutes.min,
			Hour:     1 << hours.min,
			Dom:      all(dom),
			Month:    all(months),
			Dow:      normalizeDOW(all(dow)),
			Location: loc,
		}, nil

	case "@hourly":
		return &SpecSchedule{
			Second:   1 << seconds.min,
			Minute:   1 << minutes.min,
			Hour:     all(hours),
			Dom:      all(dom),
			Month:    all(months),
			Dow:      normalizeDOW(all(dow)),
			Location: loc,
		}, nil

	}

	const every = "@every "
	if strings.HasPrefix(descriptor, every) {
		duration, err := time.ParseDuration(descriptor[len(every):])
		if err != nil {
			return nil, fmt.Errorf("failed to parse duration %s: %s", descriptor, err)
		}
		if duration <= 0 {
			return nil, fmt.Errorf("non-positive duration is not allowed: %s", descriptor)
		}
		return Every(duration), nil
	}

	return nil, fmt.Errorf("unrecognized descriptor: %s", descriptor)
}
