package domain

import "time"

const msec = time.Millisecond

func timeAt(ms int) time.Time { return time.Unix(1_700_000_000, 0).Add(time.Duration(ms) * msec) }
