// Package cluster provides common interfaces and local access to cluster-level metadata
/*
 * Copyright (c) 2019, NVIDIA CORPORATION. All rights reserved.
 */
package cluster

import (
	"github.com/NVIDIA/aistore/cmn"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Bck", func() {
	Describe("Uname", func() {
		DescribeTable("should convert bucket and objname to uname and back",
			func(bckName, bckProvider, objName string) {
				bck := &Bck{Name: bckName, Provider: bckProvider}
				uname := bck.MakeUname(objName)

				gotBck, gotObjName := ParseUname(uname)
				Expect(gotBck.Name).To(Equal(bckName))
				Expect(gotBck.Provider).To(Equal(bckProvider))
				Expect(gotObjName).To(Equal(objName))
			},
			Entry(
				"regular ais bucket with simple object name",
				"bck", cmn.ProviderAIS, "obj",
			),
			Entry(
				"regular ais bucket with long object name",
				"bck", cmn.ProviderAIS, "obj/tmp1/tmp2",
			),
			Entry(
				"aws provider",
				"bck", cmn.ProviderAmazon, "obj",
			),
			Entry(
				"gcp provider",
				"bck", cmn.ProviderGoogle, "obj",
			),
			Entry(
				"cloud provider",
				"bck", cmn.ProviderGoogle, "obj",
			),
		)
	})

	Describe("Equal", func() {
		DescribeTable("should not be equal",
			func(a, b *Bck) {
				Expect(a.Equal(b)).To(BeFalse())
			},
			Entry(
				"not matching names",
				&Bck{Name: "a", Provider: cmn.ProviderAIS}, &Bck{Name: "b", Provider: cmn.ProviderAIS},
			),
			Entry(
				"empty providers",
				&Bck{Name: "a", Provider: ""}, &Bck{Name: "a", Provider: ""},
			),
			Entry(
				"not matching providers",
				&Bck{Name: "a", Provider: cmn.ProviderAIS}, &Bck{Name: "a", Provider: ""},
			),
			Entry(
				"not matching providers #2",
				&Bck{Name: "a", Provider: cmn.ProviderAIS}, &Bck{Name: "a", Provider: cmn.Cloud},
			),
			Entry(
				"not matching providers #3",
				&Bck{Name: "a", Provider: ""}, &Bck{Name: "a", Provider: cmn.Cloud},
			),
		)

		DescribeTable("should be equal",
			func(a, b *Bck) {
				Expect(a.Equal(b)).To(BeTrue())
			},
			Entry(
				"matching AIS providers",
				&Bck{Name: "a", Provider: cmn.ProviderAIS}, &Bck{Name: "a", Provider: cmn.ProviderAIS},
			),
			Entry(
				"matching Cloud providers",
				&Bck{Name: "a", Provider: cmn.ProviderGoogle}, &Bck{Name: "a", Provider: cmn.ProviderAmazon},
			),
			Entry(
				"matching Cloud providers #2",
				&Bck{Name: "a", Provider: cmn.ProviderAmazon}, &Bck{Name: "a", Provider: cmn.ProviderGoogle},
			),
		)
	})
})
