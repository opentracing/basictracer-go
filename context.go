package basictracer

// Context holds the basic Span metadata.
type Context struct {
	// A probabilistically unique identifier for a [multi-span] trace.
	TraceID uint64

	// A probabilistically unique identifier for a span.
	SpanID uint64

	// The SpanID of this Context's parent, or 0 if there is no parent.
	ParentSpanID uint64

	// Whether the trace is sampled.
	Sampled bool

	// The span's associated baggage.
	Baggage map[string]string // initialized on first use
}
