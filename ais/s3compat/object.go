// Package s3compat provides Amazon S3 compatibility layer
/*
 * Copyright (c) 2018-2020, NVIDIA CORPORATION. All rights reserved.
 */
package s3compat

import (
	"encoding/xml"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/NVIDIA/aistore/cluster"
	"github.com/NVIDIA/aistore/cmn"
)

type (
	// List objects response
	ListObjectResult struct {
		Ns          string     `xml:"xmlns,attr"`
		Prefix      string     `xml:"Prefix"`
		KeyCount    int        `xml:"KeyCount"` // number of objects in the response
		MaxKeys     int        `xml:"MaxKeys"`
		IsTruncated bool       `xml:"IsTruncated"`           // true if there are more pages to read
		PageMarker  string     `xml:"ContinuationToken"`     // original PageMarker
		NextMarker  string     `xml:"NextContinuationToken"` // PageMarker to read the next page
		Contents    []*ObjInfo `xml:"Contents"`              // list of objects
	}
	ObjInfo struct {
		Key          string `xml:"Key"`
		LastModified string `xml:"LastModified"`
		ETag         string `xml:"ETag"`
		Size         int64  `xml:"Size"`
		Class        string `xml:"StorageClass"`
	}

	// Response for object copy request
	CopyObjectResult struct {
		LastModified string `xml:"LastModified"`
		ETag         string `xml:"ETag"`
	}
)

func FillMsgFromS3Query(query url.Values, msg *cmn.SelectMsg) {
	mxStr := query.Get("max-keys")
	if pageSize, err := strconv.Atoi(mxStr); err == nil && pageSize > 0 {
		msg.PageSize = uint(pageSize)
	}
	if prefix := query.Get("prefix"); prefix != "" {
		msg.Prefix = prefix
	}
	var marker string
	if marker = query.Get("continuation-token"); marker != "" {
		msg.PageMarker = marker
	}
	// start-after makes sense only on first call. For the next call,
	// when continuation-token is set, start-after is ignored
	if after := query.Get("start-after"); after != "" && marker == "" {
		msg.PageMarker = after
	}
}

func NewListObjectResult() *ListObjectResult {
	return &ListObjectResult{
		Ns:       s3Namespace,
		MaxKeys:  1000,
		Contents: make([]*ObjInfo, 0),
	}
}

func (r *ListObjectResult) MustMarshal() []byte {
	b, err := xml.Marshal(r)
	cmn.AssertNoErr(err)
	return []byte(xml.Header + string(b))
}

func (r *ListObjectResult) Add(entry *cmn.BucketEntry) {
	r.Contents = append(r.Contents, entryToS3(entry))
}

func entryToS3(entry *cmn.BucketEntry) *ObjInfo {
	return &ObjInfo{
		Key:          entry.Name,
		LastModified: entry.Atime,
		ETag:         entry.Checksum,
		Size:         entry.Size,
	}
}

func (r *ListObjectResult) FillFromAisBckList(bckList *cmn.BucketList) {
	r.KeyCount = len(bckList.Entries)
	r.IsTruncated = bckList.PageMarker != ""
	r.NextMarker = bckList.PageMarker
	for _, e := range bckList.Entries {
		r.Add(e)
	}
}

func FormatTime(t time.Time) string {
	s := t.UTC().Format(time.RFC1123)
	return strings.Replace(s, "UTC", "GMT", 1) // expects: "%a, %d %b %Y %H:%M:%S GMT"
}

func SetHeaderFromLOM(header http.Header, lom *cluster.LOM, size int64) {
	if v, exists := lom.GetCustomMD(cluster.SourceObjMD); exists && v == cluster.SourceAmazonObjMD {
		if v, exists := lom.GetCustomMD(cluster.MD5ObjMD); exists {
			header.Set(headerETag, v)
		}
	}
	header.Set(headerAtime, FormatTime(lom.Atime()))
	header.Set(cmn.HeaderContentLength, strconv.FormatInt(size, 10))
	header.Set(cmn.HeaderContentType, GetContentType)
	header.Set(headerVersion, lom.Version())
}

func (r *CopyObjectResult) MustMarshal() []byte {
	b, err := xml.Marshal(r)
	cmn.AssertNoErr(err)
	return []byte(xml.Header + string(b))
}
