// Package provides common low-level types and utilities for all aistore projects
/*
 * Copyright (c) 2018-2020, NVIDIA CORPORATION. All rights reserved.
 */
package cmn

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	jsoniter "github.com/json-iterator/go"
)

// ActionMsg is a JSON-formatted control structures for the REST API
type ActionMsg struct {
	Action string      `json:"action"` // ActShutdown, ActRebalance, and many more (see api_const.go)
	Name   string      `json:"name"`   // action-specific (e.g., bucket name)
	Value  interface{} `json:"value"`  // ditto
}

type ActValPromote struct {
	Target    string `json:"target"`
	ObjName   string `json:"objname"`
	Recurs    bool   `json:"recurs"`
	Overwrite bool   `json:"overwrite"`
	Verbose   bool   `json:"verbose"`
}

// SelectMsg represents properties and options for requests which fetch entities
// Note: if Fast is `true` then paging is disabled - all items are returned
//       in one response. The result list is unsorted and contains only object
//       names: even field `Status` is filled with zero value
type SelectMsg struct {
	Props       string `json:"props"`       // e.g. "checksum, size"|"atime, size"|"cached"|"bucket, size"
	TimeFormat  string `json:"time_format"` // "RFC822" default - see the enum above
	Prefix      string `json:"prefix"`      // object name filter: return only objects which name starts with prefix
	PageMarker  string `json:"pagemarker"`  // pageMarker - the last object in previous page
	Handle      string `json:"handle"`      // value passed between subsequent page requests
	UUID        string `json:"uuid"`        // ID to identify a single multi-page request
	PageSize    uint   `json:"pagesize"`    // maximum number of entries returned by list objects call
	Fast        bool   `json:"fast"`        // performs a fast traversal of the bucket contents (returns only names)
	Cached      bool   `json:"cached"`      // for cloud buckets - list only cached objects
	Passthrough bool   `json:"passthrough"` // do not use cache - always request targets for fresh data
}

type PageMarker string

// ListMsg contains a list of files and a duration within which to get them
type ListMsg struct {
	ObjNames []string `json:"objnames"`
}

// RangeMsg contains a Prefix, Regex, and Range for a Range Operation
type RangeMsg struct {
	Template string `json:"template"`
}

type InitTaskRespMsg struct {
	UUID   string `json:"uuid"`
	Handle string `json:"handle"`
}

// MountpathList contains two lists:
// * Available - list of local mountpaths available to the storage target
// * Disabled  - list of disabled mountpaths, the mountpaths that generated
//	         IO errors followed by (FSHC) health check, etc.
type MountpathList struct {
	Available []string `json:"available"`
	Disabled  []string `json:"disabled"`
}

// GetPropsDefault is a list of default (most relevant) GetProps* options
var GetPropsDefault = []string{
	GetPropsName, GetPropsChecksum, GetPropsSize, GetPropsAtime, GetPropsVersion,
}

// GetPropsAll is a list of all GetProps* options
var GetPropsAll = append(GetPropsDefault, GetPropsCached, GetTargetURL, GetPropsStatus, GetPropsCopies, GetPropsEC)

// NeedLocalData returns true if ListObjects for a cloud bucket needs
// to return object properties that can be retrieved only from local caches
func (msg *SelectMsg) NeedLocalData() bool {
	return strings.Contains(msg.Props, GetPropsAtime) ||
		strings.Contains(msg.Props, GetPropsStatus) ||
		strings.Contains(msg.Props, GetPropsCopies) ||
		strings.Contains(msg.Props, GetPropsCached)
}

// WantProp returns true if msg request requires to return propName property
func (msg *SelectMsg) WantProp(propName string) bool {
	return strings.Contains(msg.Props, propName)
}

func (msg *SelectMsg) AddProps(propNames ...string) {
	var props strings.Builder
	props.WriteString(msg.Props)
	for _, propName := range propNames {
		if msg.WantProp(propName) {
			continue
		}
		if props.Len() > 0 {
			props.WriteString(",")
		}
		props.WriteString(propName)
	}

	msg.Props = props.String()
}

func (msg *SelectMsg) PropsSet() (s StringSet) {
	props := strings.Split(msg.Props, ",")
	s = make(StringSet, len(props))
	for _, p := range props {
		s.Add(p)
	}
	return s
}

// nolint:interfacer // the bucket is expected
func (msg *SelectMsg) ListObjectsCacheID(bck Bck) string {
	f := 0
	if msg.Fast {
		f = 1
	}
	return fmt.Sprintf("%s/%s/%d", bck.String(), msg.Prefix, f)
}

func (msg *SelectMsg) WantObjectsCnt() uint {
	if msg.Fast {
		return msg.PageSize
	}

	if msg.PageSize == 0 {
		return DefaultListPageSize
	}
	return msg.PageSize
}

// Returns true if given pageMarker includes given object name.
// PageMarker includes an object name iff the object name would
// be included in response having given page marker.
func PageMarkerIncludesObject(pageMarker, objName string) bool {
	return strings.Compare(pageMarker, objName) >= 0
}

// BucketEntry corresponds to a single entry in the BucketList and
// contains file and directory metadata as per the SelectMsg
// Flags is a bit field:
// 0-2: objects status, all statuses are mutually exclusive, so it can hold up
//      to 8 different statuses. Now only OK=0, Moved=1, Deleted=2 are supported
// 3:   CheckExists (for cloud bucket it shows if the object in local cache)
type BucketEntry struct {
	Name      string `json:"name"`                  // name of the object - note: does not include the bucket name
	Size      int64  `json:"size,string,omitempty"` // size in bytes
	Checksum  string `json:"checksum,omitempty"`    // checksum
	Atime     string `json:"atime,omitempty"`       // formatted as per SelectMsg.TimeFormat
	Version   string `json:"version,omitempty"`     // version/generation ID. In GCP it is int64, in AWS it is a string
	TargetURL string `json:"target_url,omitempty"`  // URL of target which has the entry
	Copies    int16  `json:"copies,omitempty"`      // ## copies (non-replicated = 1)
	Flags     uint16 `json:"flags,omitempty"`       // object flags, like CheckExists, IsMoved etc
}

func (be *BucketEntry) CheckExists() bool {
	return be.Flags&EntryIsCached != 0
}
func (be *BucketEntry) SetExists() {
	be.Flags |= EntryIsCached
}

func (be *BucketEntry) IsStatusOK() bool {
	return be.Flags&EntryStatusMask == 0
}

// BucketList represents the contents of a given bucket - somewhat analogous to the 'ls <bucket-name>'
type BucketList struct {
	Entries    []*BucketEntry `json:"entries"`
	PageMarker string         `json:"pagemarker"`
	Handle     string         `json:"handle"`
}

type BucketSummary struct {
	Bck
	ObjCount       uint64  `json:"count,string"`
	Size           uint64  `json:"size,string"`
	TotalDisksSize uint64  `json:"disks_size,string"`
	UsedPct        float64 `json:"used_pct"`
}

func (bs *BucketSummary) Aggregate(bckSummary BucketSummary) {
	bs.ObjCount += bckSummary.ObjCount
	bs.Size += bckSummary.Size
	bs.TotalDisksSize += bckSummary.TotalDisksSize
	bs.UsedPct = float64(bs.Size) * 100 / float64(bs.TotalDisksSize)
}

type BucketsSummaries []BucketSummary

func (s BucketsSummaries) Len() int {
	return len(s)
}

func (s BucketsSummaries) Less(i, j int) bool {
	return s[i].Bck.Less(s[j].Bck)
}

func (s BucketsSummaries) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s BucketsSummaries) Aggregate(summary BucketSummary) BucketsSummaries {
	for idx, bckSummary := range s {
		if bckSummary.Bck.Equal(summary.Bck) {
			bckSummary.Aggregate(summary)
			s[idx] = bckSummary
			return s
		}
	}
	s = append(s, summary)
	return s
}

func (s BucketsSummaries) Get(bck Bck) (BucketSummary, bool) {
	for _, bckSummary := range s {
		if bckSummary.Bck.Equal(bck) {
			return bckSummary, true
		}
	}
	return BucketSummary{}, false
}

// BucketProps defines the configuration of the bucket with regard to
// its type, checksum, and LRU. These characteristics determine its behavior
// in response to operations on the bucket itself or the objects inside the bucket.
//
// Naming convention for setting/getting the particular props is defined as
// joining the json tags with dot. Eg. when referring to `EC.Enabled` field
// one would need to write `ec.enabled`. For more info refer to `IterFields`.
//
// nolint:maligned // no performance critical code
type BucketProps struct {
	// Provider of the bucket. The value contains explicit provider
	// meaning that `` or `cloud` values are forbidden.
	Provider string `json:"provider" list:"readonly"`

	// BackendBck if set it contains cloud bucket to which AIS bucket points to.
	BackendBck Bck `json:"backend_bck,omitempty"`

	// Versioning can be enabled or disabled on a per-bucket basis
	Versioning VersionConf `json:"versioning"`

	// Cksum is the embedded struct of the same name
	Cksum CksumConf `json:"checksum"`

	// LRU is the embedded struct of the same name
	LRU LRUConf `json:"lru"`

	// Mirror defines local-mirroring policy for the bucket
	Mirror MirrorConf `json:"mirror"`

	// EC defines erasure coding setting for the bucket
	EC ECConf `json:"ec"`

	// Bucket access attributes - see Allow* above
	Access AccessAttrs `json:"access,string"`

	// unique bucket ID
	BID uint64 `json:"bid,string" list:"omit"`

	// Bucket creation time
	Created int64 `json:"created,string" list:"readonly"`

	// non-empty when the bucket has been renamed (TODO: delayed deletion likewise)
	Renamed string `list:"omit"`
}

type BucketPropsToUpdate struct {
	BackendBck *BckToUpdate         `json:"backend_bck"`
	Versioning *VersionConfToUpdate `json:"versioning"`
	Cksum      *CksumConfToUpdate   `json:"checksum"`
	LRU        *LRUConfToUpdate     `json:"lru"`
	Mirror     *MirrorConfToUpdate  `json:"mirror"`
	EC         *ECConfToUpdate      `json:"ec"`
	Access     *AccessAttrs         `json:"access,string"`
}

type BckToUpdate struct {
	Name     *string `json:"name"`
	Provider *string `json:"provider"`
}

// ECConfig - per-bucket erasure coding configuration
type ECConf struct {
	ObjSizeLimit int64  `json:"objsize_limit"` // objects below this size are replicated instead of EC'ed
	DataSlices   int    `json:"data_slices"`   // number of data slices
	ParitySlices int    `json:"parity_slices"` // number of parity slices/replicas
	Compression  string `json:"compression"`   // see CompressAlways, etc. enum
	Enabled      bool   `json:"enabled"`       // EC is enabled
	BatchSize    int    `json:"batch_size"`    // Batch size for EC rebalance
}

type ECConfToUpdate struct {
	Enabled      *bool   `json:"enabled"`
	ObjSizeLimit *int64  `json:"objsize_limit"`
	DataSlices   *int    `json:"data_slices"`
	ParitySlices *int    `json:"parity_slices"`
	Compression  *string `json:"compression"`
}

func (c *VersionConf) String() string {
	if !c.Enabled {
		return "Disabled"
	}

	text := "Enabled | Validate on WarmGET: "
	if c.ValidateWarmGet {
		text += "yes"
	} else {
		text += "no"
	}

	return text
}

func (c *CksumConf) String() string {
	if c.Type == ChecksumNone {
		return "Disabled"
	}

	toValidate := make([]string, 0)
	add := func(val bool, name string) {
		if val {
			toValidate = append(toValidate, name)
		}
	}
	add(c.ValidateColdGet, "ColdGET")
	add(c.ValidateWarmGet, "WarmGET")
	add(c.ValidateObjMove, "ObjectMove")
	add(c.EnableReadRange, "ReadRange")

	toValidateStr := "Nothing"
	if len(toValidate) > 0 {
		toValidateStr = strings.Join(toValidate, ",")
	}

	return fmt.Sprintf("Type: %s | Validate: %s", c.Type, toValidateStr)
}

func (c *LRUConf) String() string {
	if !c.Enabled {
		return "Disabled"
	}
	return fmt.Sprintf("Watermarks: %d%%/%d%% | Do not evict time: %s | OOS: %v%%",
		c.LowWM, c.HighWM, c.DontEvictTimeStr, c.OOS)
}

func (c *MirrorConf) String() string {
	if !c.Enabled {
		return "Disabled"
	}

	return fmt.Sprintf("%d copies", c.Copies)
}

func (c *RebalanceConf) String() string {
	if c.Enabled {
		return "Enabled"
	}
	return "Disabled"
}

func (c *ECConf) String() string {
	if !c.Enabled {
		return "Disabled"
	}
	objSizeLimit := c.ObjSizeLimit
	return fmt.Sprintf("%d:%d (%s)", c.DataSlices, c.ParitySlices, B2S(objSizeLimit, 0))
}

func (c *ECConf) RequiredEncodeTargets() int {
	// data slices + parity slices + 1 target for original object
	return c.DataSlices + c.ParitySlices + 1
}

func (c *ECConf) RequiredRestoreTargets() int {
	// data slices + 1 target for original object
	return c.DataSlices + 1
}

// ObjectProps
type ObjectProps struct {
	Name         string           `json:"name"`
	Bck          Bck              `json:"bucket"`
	Size         int64            `json:"size"`
	Version      string           `json:"version"`
	Atime        int64            `json:"atime"`
	Checksum     ObjectCksumProps `json:"checksum"`
	NumCopies    int              `json:"copies"`
	DataSlices   int              `list:"omit"`
	ParitySlices int              `list:"omit"`
	IsECCopy     bool             `list:"omit"`
	Present      bool             `json:"present"`
}

type ObjectCksumProps struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

func DefaultBucketProps() *BucketProps {
	c := GCO.Clone()
	if c.Cksum.Type == "" {
		c.Cksum.Type = ChecksumXXHash
	}
	return &BucketProps{
		Cksum:      c.Cksum,
		LRU:        c.LRU,
		Mirror:     c.Mirror,
		Versioning: c.Versioning,
		Access:     AllAccess(),
		EC:         c.EC,
	}
}

func CloudBucketProps(header http.Header) (props *BucketProps) {
	props = DefaultBucketProps()
	Assert(len(header) > 0)

	props.Provider = header.Get(HeaderCloudProvider)
	Assert(IsValidProvider(props.Provider))
	if verStr := header.Get(HeaderBucketVerEnabled); verStr != "" {
		versioning, err := ParseBool(verStr)
		AssertNoErr(err)
		props.Versioning.Enabled = versioning
	}
	return props
}

func (bp *BucketProps) CopyFrom(from *BucketProps) {
	src, err := jsoniter.Marshal(from)
	AssertNoErr(err)
	err = jsoniter.Unmarshal(src, bp)
	AssertNoErr(err)
}

func (bp *BucketProps) Clone() *BucketProps {
	to := &BucketProps{}
	to.CopyFrom(bp)
	return to
}

func (bp *BucketProps) Equal(other *BucketProps) bool {
	var (
		jsonCompat = jsoniter.ConfigCompatibleWithStandardLibrary
		clone      = bp.Clone()
	)
	clone.BID = other.BID
	clone.Created = other.Created

	s1, _ := jsonCompat.Marshal(clone)
	s2, _ := jsonCompat.Marshal(other)
	return string(s1) == string(s2)
}

func (bp *BucketProps) Validate(targetCnt int) error {
	if !IsValidProvider(bp.Provider) {
		return fmt.Errorf("invalid cloud provider: %s, must be one of (%s)", bp.Provider, allProviders)
	}
	if !bp.BackendBck.IsEmpty() {
		if bp.BackendBck.Name == "" {
			return fmt.Errorf("backend bucket name should not be empty")
		}
		if !bp.BackendBck.IsCloud() {
			return fmt.Errorf("backend bucket should point to cloud bucket")
		}
		if bp.Provider != ProviderAIS {
			return fmt.Errorf("backend bucket can only be set for AIS buckets")
		}
	}

	validationArgs := &ValidationArgs{TargetCnt: targetCnt}
	validators := []PropsValidator{&bp.Cksum, &bp.LRU, &bp.Mirror, &bp.EC}
	for _, validator := range validators {
		if err := validator.ValidateAsProps(validationArgs); err != nil {
			return err
		}
	}

	if bp.Mirror.Enabled && bp.EC.Enabled {
		return fmt.Errorf("cannot enable mirroring and ec at the same time for the same bucket")
	}
	return nil
}

func (bp *BucketProps) Apply(propsToUpdate BucketPropsToUpdate) {
	copyProps(propsToUpdate, bp)
}

func NewBucketPropsToUpdate(nvs SimpleKVs) (props BucketPropsToUpdate, err error) {
	for key, val := range nvs {
		name, value := strings.ToLower(key), val

		// HACK: Some of the fields are present in `BucketProps` and not in
		// `BucketPropsToUpdate`. Therefore if a user would like to change such field,
		// `unknown field` would be returned in response. To make UX more pleasant we try
		// to set the value first in `BucketProps` which should report `readonly` in such case.
		if err := UpdateFieldValue(&BucketProps{}, name, value); err != nil {
			return props, err
		}

		if err := UpdateFieldValue(&props, name, value); err != nil {
			return props, err
		}
	}
	return
}

func AddBckToQuery(query url.Values, bck Bck) url.Values {
	if bck.Provider != "" {
		if query == nil {
			query = make(url.Values)
		}
		query.Set(URLParamProvider, bck.Provider)
	}
	if !bck.Ns.IsGlobal() {
		if query == nil {
			query = make(url.Values)
		}
		query.Set(URLParamNamespace, bck.Ns.Uname())
	}
	return query
}

func DelBckFromQuery(query url.Values) url.Values {
	query.Del(URLParamProvider)
	query.Del(URLParamNamespace)
	return query
}
