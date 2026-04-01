package cron

import (
	"time"
)

// Option represents a modification to the default behavior of a Cron.
type Option func(*Cron)

// WithLocation overrides the timezone of the cron instance.
func WithLocation(loc *time.Location) Option {
	if loc == nil {
		panic("cron: nil Location passed to WithLocation")
	}
	return func(c *Cron) {
		c.location = loc
	}
}

// WithSeconds overrides the parser used for interpreting job schedules to
// include a seconds field as the first one.
func WithSeconds() Option {
	return WithParser(NewParser(
		Second | Minute | Hour | Dom | Month | Dow | Descriptor,
	))
}

// WithParser overrides the parser used for interpreting job schedules.
func WithParser(p ScheduleParser) Option {
	if p == nil {
		panic("cron: nil ScheduleParser passed to WithParser")
	}
	return func(c *Cron) {
		c.parser = p
	}
}

// WithChain specifies Job wrappers to apply to all jobs added to this cron.
// Refer to the Chain* functions in this package for provided wrappers.
func WithChain(wrappers ...JobWrapper) Option {
	return func(c *Cron) {
		c.chain = NewChain(wrappers...)
		c.chainSet = true
	}
}

// WithLogger uses the provided logger.
func WithLogger(logger Logger) Option {
	if logger == nil {
		panic("cron: nil Logger passed to WithLogger")
	}
	return func(c *Cron) {
		c.logger = logger
	}
}
