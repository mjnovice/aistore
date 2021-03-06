// Package commands provides the set of CLI commands used to communicate with the AIS cluster.
// This file handles bucket operations.
/*
 * Copyright (c) 2018-2020, NVIDIA CORPORATION. All rights reserved.
 */
package commands

import (
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/NVIDIA/aistore/api"
	"github.com/NVIDIA/aistore/cmd/cli/templates"
	"github.com/NVIDIA/aistore/cmn"
	"github.com/urfave/cli"
)

const (
	allBucketAccess       = "su"
	readwriteBucketAccess = "rw"
	readonlyBucketAccess  = "ro"

	emptyOrigin = "none"

	// max wait time for a function finishes before printing "Please wait"
	longCommandTime = 10 * time.Second
)

func validateBucket(c *cli.Context, bck cmn.Bck, tag string, optional bool) (cmn.Bck, *cmn.BucketProps, error) {
	var (
		p   *cmn.BucketProps
		err error
	)
	bck.Name = cleanBucketName(bck.Name)
	if bck.Name == "" {
		if optional {
			return bck, nil, nil
		}
		if tag != "" {
			err = incorrectUsageMsg(c, "%q: missing bucket name", tag)
		} else {
			err = incorrectUsageMsg(c, "missing bucket name")
		}
		return bck, nil, err
	}
	p, err = headBucket(bck)
	return bck, p, err
}

// Creates new ais bucket
func createBucket(c *cli.Context, bck cmn.Bck, props ...cmn.BucketPropsToUpdate) (err error) {
	if err = api.CreateBucket(defaultAPIParams, bck, props...); err != nil {
		if herr, ok := err.(*cmn.HTTPError); ok {
			if herr.Status == http.StatusConflict {
				desc := fmt.Sprintf("Bucket %q already exists", bck)
				if flagIsSet(c, ignoreErrorFlag) {
					fmt.Fprint(c.App.Writer, desc)
					return nil
				}
				return fmt.Errorf(desc)
			}
		}
		return fmt.Errorf("create bucket %q failed: %s", bck, err.Error())
	}
	fmt.Fprintf(c.App.Writer, "%q bucket created\n", bck)
	return
}

// Destroy ais buckets
func destroyBuckets(c *cli.Context, buckets []cmn.Bck) (err error) {
	for _, bck := range buckets {
		if err = api.DestroyBucket(defaultAPIParams, bck); err != nil {
			if herr, ok := err.(*cmn.HTTPError); ok {
				if herr.Status == http.StatusNotFound {
					desc := fmt.Sprintf("Bucket %q does not exist", bck)
					if !flagIsSet(c, ignoreErrorFlag) {
						return fmt.Errorf(desc)
					}
					fmt.Fprint(c.App.Writer, desc)
					continue
				}
			}
			return err
		}
		fmt.Fprintf(c.App.Writer, "%q bucket destroyed\n", bck)
	}
	return nil
}

// Rename ais bucket
func renameBucket(c *cli.Context, fromBck, toBck cmn.Bck) (err error) {
	if _, err = headBucket(fromBck); err != nil {
		return
	}
	if err = api.RenameBucket(defaultAPIParams, fromBck, toBck); err != nil {
		return
	}

	msgFmt := "Renaming bucket %q to %q in progress.\nTo check the status, run: ais show xaction %s %s\n"
	fmt.Fprintf(c.App.Writer, msgFmt, fromBck.Name, toBck.Name, cmn.ActRenameLB, toBck.Name)
	return
}

// Copy ais bucket
func copyBucket(c *cli.Context, fromBck, toBck cmn.Bck) (err error) {
	if err = api.CopyBucket(defaultAPIParams, fromBck, toBck); err != nil {
		return
	}

	msgFmt := "Copying bucket %q to %q in progress.\nTo check the status, run: ais show xaction %s %s\n"
	fmt.Fprintf(c.App.Writer, msgFmt, fromBck.Name, toBck.Name, cmn.ActCopyBucket, toBck.Name)
	return
}

// Evict a cloud bucket
func evictBucket(c *cli.Context, bck cmn.Bck) (err error) {
	if flagIsSet(c, dryRunFlag) {
		fmt.Fprintf(c.App.Writer, "EVICT: %q\n", bck)
		return
	}
	if err = api.EvictCloudBucket(defaultAPIParams, bck); err != nil {
		return
	}
	fmt.Fprintf(c.App.Writer, "%q bucket evicted\n", bck)
	return
}

type (
	bucketFilter func(cmn.Bck) bool
)

// List bucket names
func listBucketNames(c *cli.Context, query cmn.QueryBcks) (err error) {
	// TODO: Think if there is a need to make generic filter for buckets as well ?
	var (
		filter = func(_ cmn.Bck) bool { return true }
		regex  *regexp.Regexp
	)
	if regexStr := parseStrFlag(c, regexFlag); regexStr != "" {
		regex, err = regexp.Compile(regexStr)
		if err != nil {
			return
		}
		filter = func(bck cmn.Bck) bool { return regex.MatchString(bck.Name) }
	}

	bucketNames, err := api.ListBuckets(defaultAPIParams, query)
	if err != nil {
		return
	}
	printBucketNames(c, bucketNames, !flagIsSet(c, noHeaderFlag), filter)
	return
}

// Lists objects in bucket
func listBucketObj(c *cli.Context, bck cmn.Bck) error {
	var (
		props string
	)

	objectListFilter, err := newObjectListFilter(c)
	if err != nil {
		return err
	}

	prefix := parseStrFlag(c, prefixFlag)
	showUnmatched := flagIsSet(c, showUnmatchedFlag)
	if parseStrFlag(c, objPropsFlag) == "all" {
		props = strings.Join(cmn.GetPropsAll, ",")
	} else {
		props = "name,"
		props += parseStrFlag(c, objPropsFlag)
		if flagIsSet(c, allItemsFlag) && !strings.Contains(props, "status") {
			// If `all` flag is set print status of the file so that the output is easier to understand -
			// there might be multiple files with the same name listed (e.g EC replicas)
			props += ",status"
		}
	}

	msg := &cmn.SelectMsg{
		Props:       props,
		Prefix:      prefix,
		Cached:      flagIsSet(c, cachedFlag),
		Passthrough: flagIsSet(c, passthroughFlag),
	}

	if flagIsSet(c, fastFlag) {
		msg.Fast = true
		objList, err := api.ListObjectsFast(defaultAPIParams, bck, msg)
		if err != nil {
			return err
		}
		return printObjectProps(c, objList.Entries, objectListFilter, props, showUnmatched, !flagIsSet(c, noHeaderFlag))
	}

	if flagIsSet(c, markerFlag) {
		msg.PageMarker = parseStrFlag(c, markerFlag)
	}
	pageSize := parseIntFlag(c, pageSizeFlag)
	limit := parseIntFlag(c, objLimitFlag)
	if pageSize < 0 {
		return fmt.Errorf("page size (%d) cannot be negative", pageSize)
	}
	if limit < 0 {
		return fmt.Errorf("max object count (%d) cannot be negative", limit)
	}
	// set page size to limit if limit is less than page size
	msg.PageSize = uint(pageSize)
	if limit > 0 && (limit < pageSize || (limit < 1000 && pageSize == 0)) {
		msg.PageSize = uint(limit)
	}

	// retrieve the bucket content page by page and print on the fly
	if flagIsSet(c, pagedFlag) {
		pageCounter, maxPages, toShow := 0, parseIntFlag(c, maxPagesFlag), limit
		for {
			objList, err := api.ListObjectsPage(defaultAPIParams, bck, msg)
			if err != nil {
				return err
			}

			// print exact number of objects if it is `limit`ed: in case of
			// limit > page size, the last page is printed partially
			var toPrint []*cmn.BucketEntry
			if limit > 0 && toShow < len(objList.Entries) {
				toPrint = objList.Entries[:toShow]
			} else {
				toPrint = objList.Entries
			}
			err = printObjectProps(c, toPrint, objectListFilter, props, showUnmatched, !flagIsSet(c, noHeaderFlag))
			if err != nil {
				return err
			}

			// interrupt the loop if:
			// 1. the last page is printed
			// 2. maximum pages are printed
			// 3. printed `limit` number of objects
			if msg.PageMarker == "" {
				return nil
			}
			pageCounter++
			if maxPages > 0 && pageCounter >= maxPages {
				return nil
			}
			if limit > 0 {
				toShow -= len(objList.Entries)
				if toShow <= 0 {
					return nil
				}
			}
		}
	}

	// retrieve the entire bucket list and print it
	objList, err := api.ListObjects(defaultAPIParams, bck, msg, uint(limit))
	if err != nil {
		return err
	}

	return printObjectProps(c, objList.Entries, objectListFilter, props, showUnmatched, !flagIsSet(c, noHeaderFlag))
}

func bucketDetails(c *cli.Context, query cmn.QueryBcks) error {
	fDetails := func() error {
		return bucketDetailsSync(c, query)
	}
	return cmn.WaitForFunc(fDetails, longCommandTime)
}

// The function shows bucket details
func bucketDetailsSync(c *cli.Context, query cmn.QueryBcks) error {
	// Request bucket summaries
	msg := &cmn.SelectMsg{
		Fast:   flagIsSet(c, fastDetailsFlag),
		Cached: flagIsSet(c, cachedFlag),
	}
	summaries, err := api.GetBucketsSummaries(defaultAPIParams, query, msg)
	if err != nil {
		return err
	}
	tmpl := templates.BucketsSummariesTmpl
	if msg.Fast {
		tmpl = templates.BucketsSummariesFastTmpl
	}
	return templates.DisplayOutput(summaries, c.App.Writer, tmpl)
}

// Replace user-friendly properties like:
//  * `access=ro` with real values `access = GET | HEAD` (all numbers are
//     passed to API as is).
//  * `backend_bck=gcp://bucket_name` with `backend_bck.name=bucket_name` and
//    `backend_bck.provider=gcp` so they match the expected fields in structs.
//  * `backend_bck=none` with `backend_bck.name=""` and `backend_bck.provider=""`.

// TODO: support `allow` and `deny` verbs/operations on existing access permissions

func reformatBucketProps(nvs cmn.SimpleKVs) error {
	if v, ok := nvs[cmn.HeaderBackendBck]; ok {
		delete(nvs, cmn.HeaderBackendBck)
		var originBck cmn.Bck
		if v != emptyOrigin {
			var (
				objName string
				err     error
			)
			originBck, objName, err = parseBckObjectURI(v)
			if err != nil {
				return err
			}
			if objName != "" {
				return fmt.Errorf("invalid format of %q", cmn.HeaderBackendBck)
			}
		}
		nvs[cmn.HeaderBackendBckName] = originBck.Name
		nvs[cmn.HeaderBackendBckProvider] = originBck.Provider
	}

	if v, ok := nvs[cmn.HeaderBucketAccessAttrs]; ok {
		switch v {
		case allBucketAccess:
			nvs[cmn.HeaderBucketAccessAttrs] = cmn.AllAccess().String()
		case readwriteBucketAccess:
			nvs[cmn.HeaderBucketAccessAttrs] = cmn.ReadWriteAccess().String()
		case readonlyBucketAccess:
			nvs[cmn.HeaderBucketAccessAttrs] = cmn.ReadOnlyAccess().String()
		default:
			// arbitrary access-flags permutation - TODO validate vs cmn/api_access.go
			if _, err := strconv.ParseUint(v, 10, 64); err != nil {
				return fmt.Errorf("invalid bucket access %q, expecting uint64 or [%q, %q, %q]",
					v, readonlyBucketAccess, readwriteBucketAccess, allBucketAccess)
			}
		}
	}
	return nil
}

// Sets bucket properties
func setBucketProps(c *cli.Context, bck cmn.Bck) (err error) {
	props, err := parseBckPropsFromContext(c)
	if err != nil {
		return
	}
	if err = api.SetBucketProps(defaultAPIParams, bck, props); err != nil {
		return
	}
	fmt.Fprintln(c.App.Writer, "Bucket props successfully updated")
	return
}

// Resets bucket props
func resetBucketProps(c *cli.Context, bck cmn.Bck) (err error) {
	if err = api.ResetBucketProps(defaultAPIParams, bck); err != nil {
		return
	}

	fmt.Fprintln(c.App.Writer, "Bucket props successfully reset")
	return
}

// Get bucket props
func showBucketProps(c *cli.Context) (err error) {
	var (
		bck     cmn.Bck
		p       *cmn.BucketProps
		objName string
	)

	if c.NArg() > 2 {
		return incorrectUsageMsg(c, "too many arguments")
	}

	section := c.Args().Get(1)
	if bck, objName, err = parseBckObjectURI(c.Args().First()); err != nil {
		return
	}
	if objName != "" {
		return objectNameArgumentNotSupported(c, objName)
	}
	if _, p, err = validateBucket(c, bck, "", false); err != nil {
		return
	}
	if flagIsSet(c, jsonFlag) {
		return templates.DisplayOutput(p, c.App.Writer, "", true)
	}

	return printBckHeadTable(c, p, section)
}

func printBckHeadTable(c *cli.Context, props *cmn.BucketProps, section string) error {
	type prop struct {
		Name  string
		Value string
	}

	// List instead of map to keep properties in the same order always.
	// All names are one word ones - for easier parsing.
	var propList []prop

	if flagIsSet(c, verboseFlag) {
		err := cmn.IterFields(props, func(uniqueTag string, field cmn.IterField) (err error, b bool) {
			value := fmt.Sprintf("%v", field.Value())
			if uniqueTag == cmn.HeaderBucketAccessAttrs {
				value = props.Access.Describe()
			}
			propList = append(propList, prop{
				Name:  uniqueTag,
				Value: value,
			})
			return nil, false
		})
		cmn.AssertNoErr(err)
	} else {
		propList = []prop{
			{"created", time.Unix(0, props.Created).Format(time.RFC3339)},
			{"provider", props.Provider},
			{"access", props.Access.Describe()},
			{"checksum", props.Cksum.String()},
			{"mirror", props.Mirror.String()},
			{"ec", props.EC.String()},
			{"lru", props.LRU.String()},
			{"versioning", props.Versioning.String()},
		}
	}

	sort.Slice(propList, func(i, j int) bool {
		return propList[i].Name < propList[j].Name
	})

	if section != "" {
		tmpPropList := propList[:0]
		for _, v := range propList {
			if strings.HasPrefix(v.Name, section) {
				tmpPropList = append(tmpPropList, v)
			}
		}
		propList = tmpPropList
	}
	return templates.DisplayOutput(propList, c.App.Writer, templates.BucketPropsSimpleTmpl)
}

// Configure bucket as n-way mirror
func configureNCopies(c *cli.Context, bck cmn.Bck, copies int) (err error) {
	var xactID string
	if xactID, err = api.MakeNCopies(defaultAPIParams, bck, copies); err != nil {
		return
	}
	var baseMsg string
	if copies > 1 {
		baseMsg = fmt.Sprintf("Configured %q as %d-way mirror,", bck, copies)
	} else {
		baseMsg = fmt.Sprintf("Configured %q for single-replica (no redundancy),", bck)
	}
	fmt.Fprintln(c.App.Writer, baseMsg, xactProgressMsg(xactID))
	return
}

// erasure code the entire bucket
func ecEncode(c *cli.Context, bck cmn.Bck, data, parity int) (err error) {
	var xactID string
	if xactID, err = api.ECEncodeBucket(defaultAPIParams, bck, data, parity); err != nil {
		return
	}
	fmt.Fprintf(c.App.Writer, "Erasure-coding bucket %q, ", bck)
	fmt.Fprintln(c.App.Writer, xactProgressMsg(xactID))
	return
}

// This function returns bucket name and new bucket name based on arguments provided to the command.
// In case something is missing it also generates a meaningful error message.
func getOldNewBucketName(c *cli.Context) (bucket, newBucket string, err error) {
	if c.NArg() == 0 {
		return "", "", missingArgumentsError(c, "bucket name", "new bucket name")
	}
	if c.NArg() == 1 {
		return "", "", missingArgumentsError(c, "new bucket name")
	}

	bucket, newBucket = cleanBucketName(c.Args().Get(0)), cleanBucketName(c.Args().Get(1))
	return
}

func printBucketNames(c *cli.Context, bucketNames cmn.BucketNames, showHeaders bool, matches bucketFilter) {
	providerList := make([]string, 0, len(cmn.Providers))
	for provider := range cmn.Providers {
		providerList = append(providerList, provider)
	}
	sort.Strings(providerList)
	for _, provider := range providerList {
		query := cmn.QueryBcks{Provider: provider}
		bcks := bucketNames.Select(query)
		if len(bcks) == 0 {
			continue
		}
		filtered := bcks[:0]
		for _, bck := range bcks {
			if matches(bck) {
				filtered = append(filtered, bck)
			}
		}
		if showHeaders {
			fmt.Fprintf(c.App.Writer, "%s Buckets (%d)\n", strings.ToUpper(provider), len(filtered))
		}
		for _, bck := range filtered {
			fmt.Fprintf(c.App.Writer, "  %s\n", bck)
		}
	}
}

func buildOutputTemplate(props string, showHeaders bool) string {
	var (
		headSb strings.Builder
		bodySb strings.Builder

		propsList = makeList(props, ",")
	)

	bodySb.WriteString("{{range $obj := .}}")
	for _, field := range propsList {
		if _, ok := templates.ObjectPropsMap[field]; !ok {
			continue
		}
		columnName := strings.ReplaceAll(strings.ToUpper(field), "_", " ")
		headSb.WriteString(columnName + "\t ")
		bodySb.WriteString(templates.ObjectPropsMap[field] + "\t ")
	}
	headSb.WriteString("\n")
	bodySb.WriteString("\n{{end}}")

	if showHeaders {
		return headSb.String() + bodySb.String()
	}

	return bodySb.String()
}

func printObjectProps(c *cli.Context, entries []*cmn.BucketEntry, objectFilter *objectListFilter, props string, showUnmatched, showHeaders bool) error {
	var (
		outputTemplate        = buildOutputTemplate(props, showHeaders)
		matchingEntries, rest = objectFilter.filter(entries)
	)
	err := templates.DisplayOutput(matchingEntries, c.App.Writer, outputTemplate)
	if err != nil {
		return err
	}

	if showHeaders && showUnmatched {
		outputTemplate = "Unmatched objects:\n" + outputTemplate
		err = templates.DisplayOutput(rest, c.App.Writer, outputTemplate)
	}
	return err
}

type (
	entryFilter func(*cmn.BucketEntry) bool

	objectListFilter struct {
		predicates []entryFilter
	}
)

func (o *objectListFilter) addFilter(f entryFilter) {
	o.predicates = append(o.predicates, f)
}

func (o *objectListFilter) matchesAll(obj *cmn.BucketEntry) bool {
	// Check if object name matches *all* specified predicates
	for _, predicate := range o.predicates {
		if !predicate(obj) {
			return false
		}
	}
	return true
}

func (o *objectListFilter) filter(entries []*cmn.BucketEntry) (matching, rest []cmn.BucketEntry) {
	for _, obj := range entries {
		if o.matchesAll(obj) {
			matching = append(matching, *obj)
		} else {
			rest = append(rest, *obj)
		}
	}
	return
}

func newObjectListFilter(c *cli.Context) (*objectListFilter, error) {
	objFilter := &objectListFilter{}

	// if fastFlag is enabled, allFlag is enabled automatically because obj.Status is unset
	if !flagIsSet(c, allItemsFlag) && !flagIsSet(c, fastFlag) {
		// Filter out files with status different than OK
		objFilter.addFilter(func(obj *cmn.BucketEntry) bool { return obj.IsStatusOK() })
	}

	if regexStr := parseStrFlag(c, regexFlag); regexStr != "" {
		regex, err := regexp.Compile(regexStr)
		if err != nil {
			return nil, err
		}

		objFilter.addFilter(func(obj *cmn.BucketEntry) bool { return regex.MatchString(obj.Name) })
	}

	if bashTemplate := parseStrFlag(c, templateFlag); bashTemplate != "" {
		pt, err := cmn.ParseBashTemplate(bashTemplate)
		if err != nil {
			return nil, err
		}

		matchingObjectNames := make(cmn.StringSet)

		linksIt := pt.Iter()
		for objName, hasNext := linksIt(); hasNext; objName, hasNext = linksIt() {
			matchingObjectNames[objName] = struct{}{}
		}
		objFilter.addFilter(func(obj *cmn.BucketEntry) bool { _, ok := matchingObjectNames[obj.Name]; return ok })
	}

	return objFilter, nil
}

func cleanBucketName(bucket string) string {
	return strings.TrimSuffix(bucket, "/")
}
