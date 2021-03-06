// Package commands provides the set of CLI commands used to communicate with the AIS cluster.
// This file handles commands that list cluster metadata information.
/*
 * Copyright (c) 2018-2020, NVIDIA CORPORATION. All rights reserved.
 */
package commands

import (
	"strings"

	"github.com/NVIDIA/aistore/cmn"
	"github.com/urfave/cli"
)

var (
	listCmdFlags = []cli.Flag{
		regexFlag,
		templateFlag,
		prefixFlag,
		pageSizeFlag,
		objPropsFlag,
		objLimitFlag,
		showUnmatchedFlag,
		allItemsFlag,
		fastFlag,
		noHeaderFlag,
		pagedFlag,
		maxPagesFlag,
		markerFlag,
		cachedFlag,
		passthroughFlag,
	}

	listCmds = []cli.Command{
		{
			Name:         commandList,
			Usage:        "list buckets and objects",
			Action:       defaultListHandler,
			ArgsUsage:    listCommandArgument,
			Flags:        listCmdFlags,
			BashComplete: bucketCompletions(bckCompletionsOpts{withProviders: true}),
		},
	}
)

func defaultListHandler(c *cli.Context) (err error) {
	var (
		bck     cmn.Bck
		objName string
	)
	if bck, objName, err = parseBckObjectURI(c.Args().First(), true /*query*/); err != nil {
		return
	}
	if objName != "" {
		return objectNameArgumentNotSupported(c, objName)
	}

	if bck, _, err = validateBucket(c, bck, "ls", true); err != nil {
		return
	}

	if bck.Name == "" {
		return listBucketNames(c, cmn.QueryBcks(bck))
	}

	bck.Name = strings.TrimSuffix(bck.Name, "/")
	return listBucketObj(c, bck)
}
