package management

import "sort"

func SortQueues(queues []QueueInfo, by, dir string) {
	sort.SliceStable(queues, func(i, j int) bool {
		a, b := queues[i], queues[j]
		switch by {
		case "ActiveMessages":
			if a.ActiveMessages != b.ActiveMessages {
				if dir == "dsc" {
					return a.ActiveMessages > b.ActiveMessages
				}
				return a.ActiveMessages < b.ActiveMessages
			}
		case "DeadLetterMessages":
			if a.DeadLetterMessages != b.DeadLetterMessages {
				if dir == "dsc" {
					return a.DeadLetterMessages > b.DeadLetterMessages
				}
				return a.DeadLetterMessages < b.DeadLetterMessages
			}
		case "SizeInMegabytes":
			if a.SizeInMegabytes != b.SizeInMegabytes {
				if dir == "dsc" {
					return a.SizeInMegabytes > b.SizeInMegabytes
				}
				return a.SizeInMegabytes < b.SizeInMegabytes
			}
		case "CreationDate":
			if !a.CreatedAt.Equal(b.CreatedAt) {
				if dir == "dsc" {
					return a.CreatedAt.After(b.CreatedAt)
				}
				return a.CreatedAt.Before(b.CreatedAt)
			}
		case "LastAccessDate":
			if !a.AccessedAt.Equal(b.AccessedAt) {
				if dir == "dsc" {
					return a.AccessedAt.After(b.AccessedAt)
				}
				return a.AccessedAt.Before(b.AccessedAt)
			}
		}
		return a.Name < b.Name
	})
}

func SortTopics(topics []TopicInfo, by, dir string) {
	sort.SliceStable(topics, func(i, j int) bool {
		a, b := topics[i], topics[j]
		switch by {
		case "SizeInMegabytes":
			if a.SizeInMegabytes != b.SizeInMegabytes {
				if dir == "dsc" {
					return a.SizeInMegabytes > b.SizeInMegabytes
				}
				return a.SizeInMegabytes < b.SizeInMegabytes
			}
		case "SubscriptionCount":
			if a.SubscriptionCount != b.SubscriptionCount {
				if dir == "dsc" {
					return a.SubscriptionCount > b.SubscriptionCount
				}
				return a.SubscriptionCount < b.SubscriptionCount
			}
		case "CreationDate":
			if !a.CreatedAt.Equal(b.CreatedAt) {
				if dir == "dsc" {
					return a.CreatedAt.After(b.CreatedAt)
				}
				return a.CreatedAt.Before(b.CreatedAt)
			}
		case "LastAccessDate":
			if !a.AccessedAt.Equal(b.AccessedAt) {
				if dir == "dsc" {
					return a.AccessedAt.After(b.AccessedAt)
				}
				return a.AccessedAt.Before(b.AccessedAt)
			}
		}
		return a.Name < b.Name
	})
}

func SortSubscriptions(subs []SubscriptionInfo, by, dir string) {
	sort.SliceStable(subs, func(i, j int) bool {
		a, b := subs[i], subs[j]
		switch by {
		case "ActiveMessages":
			if a.ActiveMessages != b.ActiveMessages {
				if dir == "dsc" {
					return a.ActiveMessages > b.ActiveMessages
				}
				return a.ActiveMessages < b.ActiveMessages
			}
		case "DeadLetterMessages":
			if a.DeadLetterMessages != b.DeadLetterMessages {
				if dir == "dsc" {
					return a.DeadLetterMessages > b.DeadLetterMessages
				}
				return a.DeadLetterMessages < b.DeadLetterMessages
			}
		case "CreationDate":
			if !a.CreatedAt.Equal(b.CreatedAt) {
				if dir == "dsc" {
					return a.CreatedAt.After(b.CreatedAt)
				}
				return a.CreatedAt.Before(b.CreatedAt)
			}
		case "LastAccessDate":
			if !a.AccessedAt.Equal(b.AccessedAt) {
				if dir == "dsc" {
					return a.AccessedAt.After(b.AccessedAt)
				}
				return a.AccessedAt.Before(b.AccessedAt)
			}
		}
		return a.Name < b.Name
	})
}
