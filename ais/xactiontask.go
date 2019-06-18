// Package ais provides core functionality for the AIStore object storage.
/*
 * Copyright (c) 2019, NVIDIA CORPORATION. All rights reserved.
 */
package ais

import (
	"context"

	"github.com/NVIDIA/aistore/cmn"
	"github.com/NVIDIA/aistore/stats"
)

type bckListTaskEntry struct {
	baseEntry
	xact    *xactBckListTask
	t       *targetrunner
	id      int64
	msg     *cmn.SelectMsg
	bucket  string
	isLocal bool
	ctx     context.Context
	cached  bool
}

func (e *bckListTaskEntry) Start(_ int64) error {
	xact := &xactBckListTask{
		XactBase: *cmn.NewXactBase(e.id, cmn.ActAsyncTask),
		t:        e.t,
		msg:      e.msg,
		bucket:   e.bucket,
		isLocal:  e.isLocal,
		ctx:      e.ctx,
		cached:   e.cached,
	}
	e.xact = xact
	go xact.Run()
	return nil
}

func (e *bckListTaskEntry) Kind() string           { return cmn.ActAsyncTask }
func (e *bckListTaskEntry) IsGlobal() bool         { return false }
func (e *bckListTaskEntry) IsTask() bool           { return true }
func (e *bckListTaskEntry) Get() cmn.Xact          { return e.xact }
func (e *bckListTaskEntry) Stats() stats.XactStats { return e.stats.FromXact(e.xact, "") }