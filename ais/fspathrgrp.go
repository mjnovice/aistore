// Package ais provides core functionality for the AIStore object storage.
/*
 * Copyright (c) 2018-2020, NVIDIA CORPORATION. All rights reserved.
 */
package ais

import (
	"sync"

	"github.com/NVIDIA/aistore/3rdparty/glog"
	"github.com/NVIDIA/aistore/cmn"
	"github.com/NVIDIA/aistore/fs"
	"github.com/NVIDIA/aistore/xaction"
)

const (
	addMpathAct     = "Added"
	enableMpathAct  = "Enabled"
	removeMpathAct  = "Removed"
	disableMpathAct = "Disabled"
)

type (
	// implements fs.PathRunGroup interface
	fsprungroup struct {
		sync.RWMutex
		t       *targetrunner
		runners map[string]fs.PathRunner // subgroup of the daemon.runners rungroup
	}
)

func (g *fsprungroup) init(t *targetrunner) {
	g.t = t
	g.runners = make(map[string]fs.PathRunner, 8)
}

func (g *fsprungroup) Reg(r fs.PathRunner) {
	cmn.Assert(r.Name() != "")
	g.Lock()
	_, ok := g.runners[r.Name()]
	cmn.Assert(!ok)
	g.runners[r.Name()] = r
	g.Unlock()
}

func (g *fsprungroup) Unreg(r fs.PathRunner) {
	g.Lock()
	_, ok := g.runners[r.Name()]
	cmn.Assert(ok)
	delete(g.runners, r.Name())
	g.Unlock()
}

// enableMountpath enables mountpath and notifies necessary runners about the
// change if mountpath actually was disabled.
func (g *fsprungroup) enableMountpath(mpath string) (enabled bool, err error) {
	gfnActive := g.t.gfn.local.Activate()
	if enabled, err = fs.Enable(mpath); err != nil || !enabled {
		if !gfnActive {
			g.t.gfn.local.Deactivate()
		}
		return
	}

	g.addMpathEvent(enableMpathAct, mpath)
	return
}

// disableMountpath disables mountpath and notifies necessary runners about the
// change if mountpath actually was disabled.
func (g *fsprungroup) disableMountpath(mpath string) (disabled bool, err error) {
	gfnActive := g.t.gfn.local.Activate()
	if disabled, err = fs.Disable(mpath); err != nil || !disabled {
		if !gfnActive {
			g.t.gfn.local.Deactivate()
		}
		return disabled, err
	}

	g.delMpathEvent(disableMpathAct, mpath)
	return true, nil
}

// addMountpath adds mountpath and notifies necessary runners about the change
// if the mountpath was actually added.
func (g *fsprungroup) addMountpath(mpath string) (err error) {
	gfnActive := g.t.gfn.local.Activate()
	if err = fs.Add(mpath); err != nil {
		if !gfnActive {
			g.t.gfn.local.Deactivate()
		}
		return
	}

	g.addMpathEvent(addMpathAct, mpath)
	return
}

// removeMountpath removes mountpath and notifies necessary runners about the
// change if the mountpath was actually removed.
func (g *fsprungroup) removeMountpath(mpath string) (err error) {
	gfnActive := g.t.gfn.local.Activate()
	if err = fs.Remove(mpath); err != nil {
		if !gfnActive {
			g.t.gfn.local.Deactivate()
		}
		return
	}

	g.delMpathEvent(removeMpathAct, mpath)
	return
}

func (g *fsprungroup) addMpathEvent(action, mpath string) {
	xaction.Registry.AbortAllMountpathsXactions()
	g.RLock()
	for _, r := range g.runners {
		switch action {
		case enableMpathAct:
			r.ReqEnableMountpath(mpath)
		case addMpathAct:
			r.ReqAddMountpath(mpath)
		default:
			cmn.AssertMsg(false, action)
		}
	}
	g.RUnlock()
	go func() {
		g.t.rebManager.RunResilver("", false /*skipGlobMisplaced*/)
		xaction.Registry.MakeNCopiesOnMpathEvent(g.t, "add-mp")
	}()
	g.checkEnable(action, mpath)
}

func (g *fsprungroup) delMpathEvent(action, mpath string) {
	xaction.Registry.AbortAllMountpathsXactions()
	g.RLock()
	for _, r := range g.runners {
		switch action {
		case disableMpathAct:
			r.ReqDisableMountpath(mpath)
		case removeMpathAct:
			r.ReqRemoveMountpath(mpath)
		default:
			cmn.AssertMsg(false, action)
		}
	}
	g.RUnlock()
	if g.checkZeroMountpaths(action) {
		return
	}

	go func() {
		g.t.rebManager.RunResilver("", false /*skipGlobMisplaced*/)
		xaction.Registry.MakeNCopiesOnMpathEvent(g.t, "del-mp")
	}()
}

// Check for no mountpaths and unregister(disable) the target if detected.
func (g *fsprungroup) checkZeroMountpaths(action string) (disabled bool) {
	availablePaths, _ := fs.Get()
	if len(availablePaths) > 0 {
		return false
	}
	if err := g.t.disable(); err != nil {
		glog.Errorf("%s the last available mountpath, failed to unregister target %s (self), err: %v", action, g.t.si, err)
	} else {
		glog.Errorf("%s the last available mountpath and unregistered target %s (self)", action, g.t.si)
	}
	return true
}

func (g *fsprungroup) checkEnable(action, mpath string) {
	availablePaths, _ := fs.Get()
	if len(availablePaths) > 1 {
		glog.Infof("%s mountpath %s", action, mpath)
	} else {
		glog.Infof("%s the first mountpath %s", action, mpath)
		if err := g.t.enable(); err != nil {
			glog.Errorf("Failed to re-register %s (self), err: %v", g.t.si, err)
		}
	}
}
