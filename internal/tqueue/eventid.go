// Package tqueue implements the monotonic update queue with a StorageCallback
// persistence seam. Mirrors tdlib td/tddb/td/db/TQueue.h and TQueue.cpp.
package tqueue

import (
	"errors"
	"fmt"
)

// MaxID is the maximum EventId value, matching tdlib TQueue::EventId::MAX_ID.
const MaxID int32 = 2000000000

// EventID is a monotonic, per-queue event identifier. It is strictly increasing
// within a queue and bounded by MaxID. Mirrors tdlib TQueue::EventId.
type EventID int32

// Value returns the int32 value of the EventId.
func (e EventID) Value() int32 { return int32(e) }

// Empty reports whether this is the zero EventId (used as "no id").
func (e EventID) Empty() bool { return e == 0 }

// IsValid reports whether the id is in the valid range (0, MaxID].
func (e EventID) IsValid() bool { return int32(e) > 0 && int32(e) <= MaxID }

// Equal is an ordered comparison helper.
func (e EventID) Equal(o EventID) bool { return e == o }

// Less reports e < o.
func (e EventID) Less(o EventID) bool { return e < o }

// EventIDFromInt32 constructs an EventId from an int32, validating the range.
// Returns an error if id is out of range (mirrors EventId::from_int32).
func EventIDFromInt32(id int32) (EventID, error) {
	if id < 0 || id > MaxID {
		return 0, fmt.Errorf("tqueue: invalid EventId %d (must be in [0, %d])", id, MaxID)
	}
	return EventID(id), nil
}

// Next returns the successor EventId, erroring at overflow past MaxID.
func (e EventID) Next() (EventID, error) {
	if int32(e)+1 > MaxID {
		return 0, errors.New("tqueue: EventId overflow")
	}
	return e + 1, nil
}

// Advance returns the EventId offset positions ahead, erroring on overflow.
func (e EventID) Advance(offset int) (EventID, error) {
	v := int64(e) + int64(offset)
	if v <= 0 || v > int64(MaxID) {
		return 0, fmt.Errorf("tqueue: EventId advance out of range (%d)", v)
	}
	return EventID(v), nil
}
