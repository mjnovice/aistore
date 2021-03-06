// Package objwalk provides core functionality for reading the list of a bucket objects
/*
 * Copyright (c) 2018-2020, NVIDIA CORPORATION. All rights reserved.
 */
package walkinfo

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/NVIDIA/aistore/cluster"
	"github.com/NVIDIA/aistore/cmn"
	"github.com/NVIDIA/aistore/fs"
)

type (
	ctxKey int

	// used to traverse local filesystem and collect objects info
	WalkInfo struct {
		t            cluster.Target
		smap         *cluster.Smap
		postCallback PostCallbackFunc
		objectFilter cluster.ObjectFilter
		propNeeded   map[string]bool
		prefix       string
		Marker       string
		markerDir    string
		msg          *cmn.SelectMsg
		bucket       string
		timeFormat   string
		fast         bool
	}

	PostCallbackFunc func(lom *cluster.LOM)
)

const (
	CtxPostCallbackKey ctxKey = iota
)

var (
	wiProps = []string{
		cmn.GetPropsSize,
		cmn.GetPropsAtime,
		cmn.GetPropsChecksum,
		cmn.GetPropsVersion,
		cmn.GetPropsStatus,
		cmn.GetPropsCopies,
		cmn.GetTargetURL,
	}
)

func NewWalkInfo(ctx context.Context, t cluster.Target, bucket string, msg *cmn.SelectMsg) *WalkInfo {
	// Marker is always a file name, so we need to strip filename from path
	markerDir := ""
	if msg.PageMarker != "" {
		markerDir = filepath.Dir(msg.PageMarker)
		if markerDir == "." {
			markerDir = ""
		}
	}

	// A small optimization: set boolean variables need* to avoid
	// doing string search(strings.Contains) for every entry.
	postCallback, _ := ctx.Value(CtxPostCallbackKey).(PostCallbackFunc)

	propNeeded := make(map[string]bool, len(wiProps))
	for _, prop := range wiProps {
		propNeeded[prop] = msg.WantProp(prop)
	}
	return &WalkInfo{
		t:            t, // targetrunner
		smap:         t.GetSowner().Get(),
		postCallback: postCallback,
		prefix:       msg.Prefix,
		Marker:       msg.PageMarker,
		markerDir:    markerDir,
		msg:          msg,
		bucket:       bucket,
		fast:         msg.Fast,
		timeFormat:   msg.TimeFormat,
		propNeeded:   propNeeded,
	}
}

func NewDefaultWalkInfo(t cluster.Target, bucket string) *WalkInfo {
	propNeeded := make(map[string]bool, len(wiProps))
	for _, prop := range wiProps {
		propNeeded[prop] = true
	}
	return &WalkInfo{
		t:          t, // targetrunner
		smap:       t.GetSowner().Get(),
		bucket:     bucket,
		propNeeded: propNeeded,
	}
}

func (wi *WalkInfo) needSize() bool      { return wi.propNeeded[cmn.GetPropsSize] }
func (wi *WalkInfo) needAtime() bool     { return wi.propNeeded[cmn.GetPropsAtime] }
func (wi *WalkInfo) needCksum() bool     { return wi.propNeeded[cmn.GetPropsChecksum] }
func (wi *WalkInfo) needVersion() bool   { return wi.propNeeded[cmn.GetPropsVersion] }
func (wi *WalkInfo) needStatus() bool    { return wi.propNeeded[cmn.GetPropsStatus] } //nolint:unused // left for consistency
func (wi *WalkInfo) needCopies() bool    { return wi.propNeeded[cmn.GetPropsCopies] }
func (wi *WalkInfo) needTargetURL() bool { return wi.propNeeded[cmn.GetTargetURL] }

// Checks if the directory should be processed by cache list call
// Does checks:
//  - Object name must start with prefix (if it is set)
//  - Object name is not in early processed directories by the previous call:
//    paging support
func (wi *WalkInfo) ProcessDir(fqn string) error {
	ct, err := cluster.NewCTFromFQN(fqn, nil)
	if err != nil {
		return nil
	}

	// every directory has to either:
	// - be contained in prefix (for levels lower than prefix: prefix="abcd/def", directory="abcd")
	// - or include prefix (for levels deeper than prefix: prefix="a/", directory="a/b")
	if wi.prefix != "" && !(strings.HasPrefix(wi.prefix, ct.ObjName()) || strings.HasPrefix(ct.ObjName(), wi.prefix)) {
		return filepath.SkipDir
	}

	// When markerDir = "b/c/d/" we should skip directories: "a/", "b/a/",
	// "b/b/" etc. but should not skip entire "b/" or "b/c/" since it is our
	// parent which we want to traverse (see that: "b/" < "b/c/d/").
	if wi.markerDir != "" && ct.ObjName() < wi.markerDir && !strings.HasPrefix(wi.markerDir, ct.ObjName()) {
		return filepath.SkipDir
	}

	return nil
}

func (wi *WalkInfo) Callback(fqn string, de fs.DirEntry) (*cmn.BucketEntry, error) {
	if !wi.fast {
		return wi.walkCallback(fqn, de)
	}
	return wi.walkFastCallback(fqn, de)
}

func (wi *WalkInfo) SetObjectFilter(f cluster.ObjectFilter) {
	wi.objectFilter = f
}

// Adds an info about cached object to the list if:
//  - its name starts with prefix (if prefix is set)
//  - it has not been already returned by previous page request
//  - this target responses getobj request for the object
func (wi *WalkInfo) lsObject(lom *cluster.LOM, objStatus uint16) *cmn.BucketEntry {
	objName := lom.ParsedFQN.ObjName
	if wi.prefix != "" && !strings.HasPrefix(objName, wi.prefix) {
		return nil
	}
	if wi.Marker != "" && cmn.PageMarkerIncludesObject(wi.Marker, objName) {
		return nil
	}
	if wi.objectFilter != nil && !wi.objectFilter(lom) {
		return nil
	}

	// add the obj to the page
	fileInfo := &cmn.BucketEntry{
		Name:   objName,
		Atime:  "",
		Flags:  objStatus | cmn.EntryIsCached,
		Copies: 1,
	}
	if wi.needAtime() {
		fileInfo.Atime = cmn.FormatUnixNano(lom.AtimeUnix(), wi.timeFormat)
	}
	if wi.needCksum() && lom.Cksum() != nil {
		_, storedCksum := lom.Cksum().Get()
		fileInfo.Checksum = storedCksum
	}
	if wi.needVersion() {
		fileInfo.Version = lom.Version()
	}
	if wi.needCopies() {
		fileInfo.Copies = int16(lom.NumCopies())
	}
	if wi.needTargetURL() {
		fileInfo.TargetURL = wi.t.Snode().URL(cmn.NetworkPublic)
	}
	fileInfo.Size = lom.Size()
	if wi.postCallback != nil {
		wi.postCallback(lom)
	}
	return fileInfo
}

// fast alternative of generic listwalk: do not fetch any object information
// Returns all objects if the `msg.PageSize` was not specified. But the result
// may have 'ghost' or duplicated  objects.
func (wi *WalkInfo) walkFastCallback(fqn string, de fs.DirEntry) (*cmn.BucketEntry, error) {
	if de.IsDir() {
		return nil, nil
	}

	ct, err := cluster.NewCTFromFQN(fqn, nil)
	if err != nil {
		return nil, nil
	}

	if wi.prefix != "" && !strings.HasPrefix(ct.ObjName(), wi.prefix) {
		return nil, nil
	}
	if wi.Marker != "" && cmn.PageMarkerIncludesObject(wi.Marker, ct.ObjName()) {
		return nil, nil
	}
	fileInfo := &cmn.BucketEntry{
		Name:  ct.ObjName(),
		Flags: cmn.ObjStatusOK,
	}
	if wi.needSize() {
		fi, err := os.Stat(fqn)
		if err == nil {
			fileInfo.Size = fi.Size()
		}
	}

	return fileInfo, nil
}

func (wi *WalkInfo) walkCallback(fqn string, de fs.DirEntry) (*cmn.BucketEntry, error) {
	if de.IsDir() {
		return nil, nil
	}

	var (
		objStatus uint16 = cmn.ObjStatusOK
	)
	lom := &cluster.LOM{T: wi.t, FQN: fqn}
	if err := lom.Init(cmn.Bck{}); err != nil {
		return nil, err
	}

	if err := lom.Load(); err != nil {
		if cmn.IsErrObjNought(err) {
			return nil, nil
		}
		return nil, err
	}
	if lom.IsCopy() {
		return nil, nil
	}
	if !lom.IsHRW() {
		objStatus = cmn.ObjStatusMoved
	} else {
		si, err := cluster.HrwTarget(lom.Uname(), wi.smap)
		if err != nil {
			return nil, err
		}
		if wi.t.Snode().ID() != si.ID() {
			objStatus = cmn.ObjStatusMoved
		}
	}
	return wi.lsObject(lom, objStatus), nil
}
