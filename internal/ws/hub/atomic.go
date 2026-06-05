package hub

import "sync/atomic"

// atomicAdd64 wraps sync/atomic.AddUint64 so client.go's id helper
// stays a single line.
func atomicAdd64(p *uint64, delta uint64) uint64 {
	return atomic.AddUint64(p, delta)
}
