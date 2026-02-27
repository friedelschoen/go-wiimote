package linuxhid

// #include <string.h>
// #include <stdlib.h>
// #include <linux/input.h>
// #include <errno.h>
//
// unsigned int eviocgname(size_t sz) { return EVIOCGNAME(sz); }
import "C"
import (
	"time"
)

func cTimeMake(orig time.Time) C.struct_timeval {
	var t C.struct_timeval
	t.tv_sec = C.time_t(orig.Unix())
	t.tv_usec = C.time_t(orig.Nanosecond() / 1000)
	return t
}

// cTime takes an C timeval and converts it to time.Time
func cTime(t C.struct_timeval) time.Time {
	return time.Unix(int64(t.tv_sec), int64(t.tv_usec)*1000)
}
