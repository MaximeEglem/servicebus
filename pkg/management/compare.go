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
	names := map[string]bool{}
	for _, queues := range contexts {
		for _, q := range queues {
			names[q.Name] = true
		}
	}

	var results []QueueCompareResult
	for name := range names {
		r := QueueCompareResult{
			Name:      name,
			PresentIn: map[string]bool{},
			Counts:    map[string]int64{},
			DLCounts:  map[string]int64{},
		}
		for ctx, queues := range contexts {
			for _, q := range queues {
				if q.Name == name {
					r.PresentIn[ctx] = true
					r.Counts[ctx] = q.ActiveMessages
					r.DLCounts[ctx] = q.DeadLetterMessages
				}
			}
		}
		if len(r.PresentIn) < len(contexts) {
			r.HasIssue = true
		} else {
			var min, max int64 = 1<<62, 0
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
	names := map[string]bool{}
	for _, topics := range contexts {
		for _, t := range topics {
			names[t.Name] = true
		}
	}
	var results []TopicCompareResult
	for name := range names {
		r := TopicCompareResult{
			Name:      name,
			PresentIn: map[string]bool{},
			Sizes:     map[string]int64{},
		}
		for ctx, topics := range contexts {
			for _, t := range topics {
				if t.Name == name {
					r.PresentIn[ctx] = true
					r.Sizes[ctx] = t.SizeInMegabytes
				}
			}
		}
		if len(r.PresentIn) < len(contexts) {
			r.HasIssue = true
		}
		results = append(results, r)
	}
	return results
}
