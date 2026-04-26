package management

import "sort"

func SortQueues(queues []QueueInfo, by, dir string) {
	sort.SliceStable(queues, func(i, j int) bool {
		a, b := queues[i], queues[j]
		var less bool
		switch by {
		case "ActiveMessages":
			less = a.ActiveMessages < b.ActiveMessages
		case "DeadLetterMessages":
			less = a.DeadLetterMessages < b.DeadLetterMessages
		case "SizeInMegabytes":
			less = a.SizeInMegabytes < b.SizeInMegabytes
		case "CreationDate":
			less = a.CreatedAt.Before(b.CreatedAt)
		case "LastAccessDate":
			less = a.AccessedAt.Before(b.AccessedAt)
		default:
			less = a.Name < b.Name
		}
		if dir == "dsc" {
			return !less
		}
		return less
	})
}

func SortTopics(topics []TopicInfo, by, dir string) {
	sort.SliceStable(topics, func(i, j int) bool {
		a, b := topics[i], topics[j]
		var less bool
		switch by {
		case "SizeInMegabytes":
			less = a.SizeInMegabytes < b.SizeInMegabytes
		case "SubscriptionCount":
			less = a.SubscriptionCount < b.SubscriptionCount
		case "CreationDate":
			less = a.CreatedAt.Before(b.CreatedAt)
		case "LastAccessDate":
			less = a.AccessedAt.Before(b.AccessedAt)
		default:
			less = a.Name < b.Name
		}
		if dir == "dsc" {
			return !less
		}
		return less
	})
}

func SortSubscriptions(subs []SubscriptionInfo, by, dir string) {
	sort.SliceStable(subs, func(i, j int) bool {
		a, b := subs[i], subs[j]
		var less bool
		switch by {
		case "ActiveMessages":
			less = a.ActiveMessages < b.ActiveMessages
		case "DeadLetterMessages":
			less = a.DeadLetterMessages < b.DeadLetterMessages
		case "CreationDate":
			less = a.CreatedAt.Before(b.CreatedAt)
		case "LastAccessDate":
			less = a.AccessedAt.Before(b.AccessedAt)
		default:
			less = a.Name < b.Name
		}
		if dir == "dsc" {
			return !less
		}
		return less
	})
}
