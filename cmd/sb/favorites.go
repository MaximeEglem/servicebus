package main

import (
	"servicebus/internal/config"
	"servicebus/pkg/management"
)

// activeFavorites returns the favorites of the currently selected context.
// Resolution mirrors resolveCredWithConfig: -c flag > current-context > saved state.
func activeFavorites(cfg *config.Config) config.Favorites {
	name := gf.context
	if name == "" {
		name = cfg.CurrentContext
	}
	if name == "" {
		if state, err := config.LoadState(); err == nil {
			name = state.Context
		}
	}
	if name == "" {
		return config.Favorites{}
	}
	return cfg.Contexts[name].Favorites
}

// activeFavoritesNoErr loads config and returns favorites, swallowing errors
// (favorites are a best-effort UX feature; never fail a list because of them).
func activeFavoritesNoErr() config.Favorites {
	cfg, err := config.Load()
	if err != nil {
		return config.Favorites{}
	}
	return activeFavorites(cfg)
}

// bubbleQueueFavorites stably moves favorited queues to the top, preserving
// the existing order within each partition.
func bubbleQueueFavorites(qs []management.QueueInfo, fav config.Favorites) {
	if len(fav.Queues) == 0 || len(qs) <= 1 {
		return
	}
	favs := make([]management.QueueInfo, 0, len(qs))
	rest := make([]management.QueueInfo, 0, len(qs))
	for _, q := range qs {
		if fav.HasQueue(q.Name) {
			favs = append(favs, q)
		} else {
			rest = append(rest, q)
		}
	}
	copy(qs, append(favs, rest...))
}

func bubbleTopicFavorites(ts []management.TopicInfo, fav config.Favorites) {
	if len(fav.Topics) == 0 || len(ts) <= 1 {
		return
	}
	favs := make([]management.TopicInfo, 0, len(ts))
	rest := make([]management.TopicInfo, 0, len(ts))
	for _, t := range ts {
		if fav.HasTopic(t.Name) {
			favs = append(favs, t)
		} else {
			rest = append(rest, t)
		}
	}
	copy(ts, append(favs, rest...))
}

func bubbleSubFavorites(ss []management.SubscriptionInfo, fav config.Favorites) {
	if len(fav.Subscriptions) == 0 || len(ss) <= 1 {
		return
	}
	favs := make([]management.SubscriptionInfo, 0, len(ss))
	rest := make([]management.SubscriptionInfo, 0, len(ss))
	for _, s := range ss {
		if fav.HasSubscription(s.TopicName, s.Name) {
			favs = append(favs, s)
		} else {
			rest = append(rest, s)
		}
	}
	copy(ss, append(favs, rest...))
}

// markQueueFavoritesForTable prefixes the Name of favorited queues with a star.
// Only call this when output is the table format — it mutates Name.
func markQueueFavoritesForTable(qs []management.QueueInfo, fav config.Favorites) {
	if len(fav.Queues) == 0 {
		return
	}
	for i := range qs {
		if fav.HasQueue(qs[i].Name) {
			qs[i].Name = "★ " + qs[i].Name
		}
	}
}

func markTopicFavoritesForTable(ts []management.TopicInfo, fav config.Favorites) {
	if len(fav.Topics) == 0 {
		return
	}
	for i := range ts {
		if fav.HasTopic(ts[i].Name) {
			ts[i].Name = "★ " + ts[i].Name
		}
	}
}

func markSubFavoritesForTable(ss []management.SubscriptionInfo, fav config.Favorites) {
	if len(fav.Subscriptions) == 0 {
		return
	}
	for i := range ss {
		if fav.HasSubscription(ss[i].TopicName, ss[i].Name) {
			ss[i].Name = "★ " + ss[i].Name
		}
	}
}
