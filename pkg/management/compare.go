package management

// QueueCompareResult is one row in the N-way queue comparison.
type QueueCompareResult struct {
	Name      string
	PresentIn map[string]bool  // context name → present
	Counts    map[string]int64 // context name → ActiveMessages
	DLCounts  map[string]int64 // context name → DeadLetterMessages
	HasIssue  bool             // true if missing in any context or counts diverge significantly
}

// CompareQueues performs an N-way diff of queue lists across contexts.
func CompareQueues(contexts map[string][]QueueInfo) []QueueCompareResult {
	// Build per-context index: ctx -> queueName -> *QueueInfo
	index := make(map[string]map[string]*QueueInfo, len(contexts))
	names := map[string]struct{}{}
	for ctx, queues := range contexts {
		m := make(map[string]*QueueInfo, len(queues))
		for i := range queues {
			q := &queues[i]
			m[q.Name] = q
			names[q.Name] = struct{}{}
		}
		index[ctx] = m
	}

	results := make([]QueueCompareResult, 0, len(names))
	for name := range names {
		r := QueueCompareResult{
			Name:      name,
			PresentIn: make(map[string]bool, len(contexts)),
			Counts:    make(map[string]int64, len(contexts)),
			DLCounts:  make(map[string]int64, len(contexts)),
		}
		for ctx, m := range index {
			if q, ok := m[name]; ok {
				r.PresentIn[ctx] = true
				r.Counts[ctx] = q.ActiveMessages
				r.DLCounts[ctx] = q.DeadLetterMessages
			}
		}
		if len(r.PresentIn) < len(contexts) {
			r.HasIssue = true
		} else {
			var min, max int64 = 1 << 62, 0
			for _, c := range r.Counts {
				if c < min {
					min = c
				}
				if c > max {
					max = c
				}
			}
			if max > 0 && min == 0 {
				r.HasIssue = true
			} else if min > 0 && max/min > 10 {
				r.HasIssue = true
			}
		}
		results = append(results, r)
	}
	return results
}

// TopicCompareResult is one row in the N-way topic comparison.
type TopicCompareResult struct {
	Name      string
	PresentIn map[string]bool
	Sizes     map[string]int64
	HasIssue  bool
}

// CompareTopics performs an N-way diff of topic lists across contexts.
func CompareTopics(contexts map[string][]TopicInfo) []TopicCompareResult {
	index := make(map[string]map[string]*TopicInfo, len(contexts))
	names := map[string]struct{}{}
	for ctx, topics := range contexts {
		m := make(map[string]*TopicInfo, len(topics))
		for i := range topics {
			t := &topics[i]
			m[t.Name] = t
			names[t.Name] = struct{}{}
		}
		index[ctx] = m
	}
	results := make([]TopicCompareResult, 0, len(names))
	for name := range names {
		r := TopicCompareResult{
			Name:      name,
			PresentIn: make(map[string]bool, len(contexts)),
			Sizes:     make(map[string]int64, len(contexts)),
		}
		for ctx, m := range index {
			if t, ok := m[name]; ok {
				r.PresentIn[ctx] = true
				r.Sizes[ctx] = t.SizeInMegabytes
			}
		}
		if len(r.PresentIn) < len(contexts) {
			r.HasIssue = true
		}
		results = append(results, r)
	}
	return results
}
