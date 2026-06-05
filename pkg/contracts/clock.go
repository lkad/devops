package contracts

import "time"

// nowFn is the clock used by JWTClaims.IsExpired. It is a package-level
// variable so tests can swap it for a deterministic time without
// having to plumb a clock through every API.
var nowFn = func() int64 { return time.Now().Unix() }
