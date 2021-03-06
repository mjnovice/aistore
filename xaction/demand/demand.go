// Package demand provides core functionality for the AIStore on-demand extended actions.
/*
 * Copyright (c) 2018-2020, NVIDIA CORPORATION. All rights reserved.
 */
package demand

import (
	"time"

	"github.com/NVIDIA/aistore/3rdparty/atomic"
	"github.com/NVIDIA/aistore/cmn"
	"github.com/NVIDIA/aistore/cmn/debug"
	"github.com/NVIDIA/aistore/hk"
)

var (
	// Default on-demand xaction idle timeout
	// (to confirm idle-ness we may in fact stay around for twice as much)
	xactIdleTimeout = time.Minute
)

type (
	//
	// xaction that self-terminates after staying idle for a while
	// with an added capability to renew itself and ref-count its pending work
	//
	XactDemand interface {
		cmn.Xact
		IdleTimer() <-chan struct{}
		IncPending()
		DecPending()
		SubPending(n int)
	}

	idleInfo struct {
		dur    time.Duration
		ticks  *cmn.StopCh
		likely bool
	}

	XactDemandBase struct {
		cmn.XactBase

		pending atomic.Int64
		active  atomic.Int64
		hkName  string
		idle    idleInfo
	}
)

var (
	_ XactDemand = &XactDemandBase{}
)

////////////////////
// XactDemandBase //
////////////////////

func NewXactDemandBaseBck(kind string, bck cmn.Bck, idleTimes ...time.Duration) *XactDemandBase {
	idleTime := xactIdleTimeout
	if len(idleTimes) != 0 {
		idleTime = idleTimes[0]
	}
	r := &XactDemandBase{
		XactBase: *cmn.NewXactBaseBck("", kind, bck),
		hkName:   kind + "/" + cmn.GenUUID(),
		idle:     idleInfo{dur: idleTime, ticks: cmn.NewStopCh()},
	}
	r.init()
	return r
}

func NewXactDemandBase(uuid, kind string, idleTimes ...time.Duration) *XactDemandBase {
	var hkName string
	idleTime := xactIdleTimeout
	if len(idleTimes) != 0 {
		idleTime = idleTimes[0]
	}
	if uuid == "" {
		hkName = kind + cmn.GenUUID()
	} else {
		hkName = kind + "/" + uuid
	}
	r := &XactDemandBase{
		XactBase: *cmn.NewXactBase(cmn.XactBaseID(uuid), kind),
		hkName:   hkName,
		idle:     idleInfo{dur: idleTime, ticks: cmn.NewStopCh()},
	}
	r.init()
	return r
}

func (r *XactDemandBase) init() {
	hk.Reg(r.hkName, func() time.Duration {
		active := r.active.Swap(0)
		if r.Pending() > 0 || active > 0 {
			r.idle.likely = false // not idle
		} else if active == 0 {
			if r.idle.likely {
				r.idle.ticks.Close() // idleness confirmed: send "idle tick".
			} else {
				// likely idle (haven't seen any activity): prepare to send "idle tick"
				r.idle.likely = true
			}
		}
		return r.idle.dur
	})
}

func (r *XactDemandBase) IdleTimer() <-chan struct{} { return r.idle.ticks.Listen() }
func (r *XactDemandBase) Pending() int64             { return r.pending.Load() }
func (r *XactDemandBase) IncPending()                { r.pending.Inc(); r.active.Inc() }
func (r *XactDemandBase) DecPending()                { r.SubPending(1) }
func (r *XactDemandBase) SubPending(n int) {
	r.pending.Sub(int64(n))
	debug.Assert(r.Pending() >= 0)
}

func (r *XactDemandBase) Stop() {
	hk.Unreg(r.hkName)
	r.idle.ticks.Close()
}
