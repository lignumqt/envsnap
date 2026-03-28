package collectors

import "context"

// Section holds the output of a single collector.
type Section struct {
	Name string
	Data any
}

// Collector is the interface all environment collectors must implement.
type Collector interface {
	// Name returns the unique name of the collector.
	Name() string
	// Collect gathers data and returns a populated Section.
	Collect(ctx context.Context) (Section, error)
}
