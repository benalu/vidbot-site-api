package limiter

import "sync/atomic"

type Limiter struct {
	current int64
	max     int64
}

func New(max int) *Limiter {
	return &Limiter{max: int64(max)}
}

func (l *Limiter) Acquire() bool {
	for {
		current := atomic.LoadInt64(&l.current)
		if current >= l.max {
			return false
		}
		if atomic.CompareAndSwapInt64(&l.current, current, current+1) {
			return true
		}
	}
}

func (l *Limiter) Release() {
	atomic.AddInt64(&l.current, -1)
}

func (l *Limiter) Current() int64 {
	return atomic.LoadInt64(&l.current)
}

func (l *Limiter) Max() int64 {
	return l.max
}
