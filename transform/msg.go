// Package transform provides utilities to initialize and use transformation pods.
/*
 * Copyright (c) 2018-2020, NVIDIA CORPORATION. All rights reserved.
 */
package transform

import (
	"github.com/NVIDIA/aistore/cmn"
)

type Msg struct {
	ID          string           `json:"id"`
	Spec        []byte           `json:"spec"`
	CommType    string           `json:"communication_type"`
	WaitTimeout cmn.DurationJSON `json:"wait_timeout"`
}
