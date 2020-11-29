package proxy

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"

	"../rpc"
	"../util"
)

func (s *ProxyServer) StatsIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	hashrate, hashrate24h, totalOnline, miners := s.collectMinersStats()
	stats := map[string]interface{}{
		"miners":      miners,
		"hashrate":    hashrate,
		"hashrate24h": hashrate24h,
		"totalMiners": len(miners),
		"totalOnline": totalOnline,
		"timedOut":    len(miners) - totalOnline,
	}

	var upstreams []interface{}
	current := atomic.LoadInt32(&s.upstream)

	for i, u := range s.upstreams {
		upstream := convertUpstream(u)
		upstream["current"] = current == int32(i)
		upstreams = append(upstreams, upstream)
	}
	stats["upstreams"] = upstreams
	stats["current"] = convertUpstream(s.rpc())
	stats["url"] = "http://" + s.config.Proxy.Listen + "/miner/<diff>/<id>"

	t := s.currentBlockTemplate()
	stats["height"] = t.Height
	stats["diff"] = t.Difficulty
	stats["luck"] = s.getLuckStats()
	stats["now"] = util.MakeTimestamp()
	json.NewEncoder(w).Encode(stats)
}

func convertUpstream(u *rpc.RPCClient) map[string]interface{} {
	upstream := map[string]interface{}{
		"name":             u.Name,
		"url":              u.Url.String(),
		"pool":             u.Pool,
		"sick":             u.Sick(),
		"accepts":          atomic.LoadUint64(&u.Accepts),
		"rejects":          atomic.LoadUint64(&u.Rejects),
		"lastSubmissionAt": atomic.LoadInt64(&u.LastSubmissionAt),
		"failsCount":       atomic.LoadUint64(&u.FailsCount),
	}
	return upstream
}

func (s *ProxyServer) collectMinersStats() (int64, int64, int, []interface{}) {
	now := util.MakeTimestamp()
	var result []interface{}
	totalHashrate := int64(0)
	totalHashrate24h := int64(0)
	totalOnline := 0
	window24h := 24 * time.Hour

	for m := range s.miners.Iter() {
		stats := make(map[string]interface{})
		lastBeat := m.Val.getLastBeat()
		hashrate := m.Val.hashrate(s.hashrateWindow)
		hashrate24h := m.Val.hashrate(window24h)
		totalHashrate += hashrate
		totalHashrate24h += hashrate24h
		stats["name"] = m.Key
		stats["hashrate"] = hashrate
		stats["hashrate24h"] = hashrate24h
		stats["lastBeat"] = lastBeat
		stats["validShares"] = atomic.LoadUint64(&m.Val.validShares)
		stats["invalidShares"] = atomic.LoadUint64(&m.Val.invalidShares)
		stats["accepts"] = atomic.LoadUint64(&m.Val.accepts)
		stats["rejects"] = atomic.LoadUint64(&m.Val.rejects)
		stats["ip"] = m.Val.IP

		if now-lastBeat > (int64(s.timeout/2) / 1000000) {
			stats["warning"] = true
		}
		if now-lastBeat > (int64(s.timeout) / 1000000) {
			stats["timeout"] = true
		} else {
			totalOnline++
		}
		result = append(result, stats)
	}
	return totalHashrate, totalHashrate24h, totalOnline, result
}

func (s *ProxyServer) getLuckStats() map[string]interface{} {
	now := util.MakeTimestamp()
	var variance float64
	var totalVariance float64
	var blocksCount int
	var totalBlocksCount int

	s.blocksMu.Lock()
	defer s.blocksMu.Unlock()

	for k, v := range s.blockStats {
		if k >= now-int64(s.luckWindow) {
			blocksCount++
			variance += v
		}
		if k >= now-int64(s.luckLargeWindow) {
			totalBlocksCount++
			totalVariance += v
		} else {
			delete(s.blockStats, k)
		}
	}
	if blocksCount != 0 {
		variance = variance / float64(blocksCount)
	}
	if totalBlocksCount != 0 {
		totalVariance = totalVariance / float64(totalBlocksCount)
	}
	result := make(map[string]interface{})
	result["variance"] = variance
	result["blocksCount"] = blocksCount
	result["window"] = s.config.Proxy.LuckWindow
	result["totalVariance"] = totalVariance
	result["totalBlocksCount"] = totalBlocksCount
	result["largeWindow"] = s.config.Proxy.LargeLuckWindow
	return result
}
