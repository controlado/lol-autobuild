package lcu

import (
	"context"
	"strings"
)

func (c *Client) resolveChampionName(ctx context.Context, info connectionInfo, championID int) string {
	if championID <= 0 {
		return ""
	}

	if name, summaryLoaded, ok := c.cachedChampionName(championID); ok {
		return name
	} else if !summaryLoaded {
		names, err := c.fetchChampionSummaryNames(ctx, info)
		if err == nil {
			c.replaceChampionNameCache(names)
			if name := strings.TrimSpace(names[championID]); name != "" {
				return name
			}
		}
	}

	name, err := c.fetchChampionName(ctx, info, championID)
	if err != nil {
		return ""
	}

	c.storeChampionName(championID, name)
	return name
}

func (c *Client) cachedChampionName(championID int) (name string, summaryLoaded bool, ok bool) {
	c.championNameMu.Lock()
	defer c.championNameMu.Unlock()

	if c.championNameCache == nil {
		return "", c.championSummaryLoaded, false
	}

	name, ok = c.championNameCache[championID]
	return name, c.championSummaryLoaded, ok
}

func (c *Client) replaceChampionNameCache(names map[int]string) {
	c.championNameMu.Lock()
	defer c.championNameMu.Unlock()

	c.championNameCache = make(map[int]string, len(names))
	for id, name := range names {
		if id > 0 && strings.TrimSpace(name) != "" {
			c.championNameCache[id] = strings.TrimSpace(name)
		}
	}
	c.championSummaryLoaded = true
}

func (c *Client) storeChampionName(championID int, name string) {
	name = strings.TrimSpace(name)
	if championID <= 0 || name == "" {
		return
	}

	c.championNameMu.Lock()
	defer c.championNameMu.Unlock()

	if c.championNameCache == nil {
		c.championNameCache = map[int]string{}
	}
	c.championNameCache[championID] = name
}
