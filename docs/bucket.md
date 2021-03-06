## Table of Contents

- [Bucket](#bucket)
  - [Cloud Provider](#cloud-provider)
- [AIS Bucket](#ais-bucket)
  - [CLI examples: create, rename and, destroy ais bucket](#cli-examples-create-rename-and-destroy-ais-bucket)
  - [CLI example: working with remote AIS bucket](#cli-example-working-with-remote-ais-bucket)
- [Cloud Bucket](#cloud-bucket)
  - [Prefetch/Evict Objects](#prefetchevict-objects)
  - [Evict Cloud Bucket](#evict-cloud-bucket)
- [Backend Bucket](#backend-bucket)
- [Bucket Access Attributes](#bucket-access-attributes)
- [List Objects](#list-objects)
  - [Properties and Options](#properties-and-options)
  - [CLI examples: listing and setting bucket properties](#cli-examples-listing-and-setting-bucket-properties)
- [Recover Buckets](#recover-buckets)
  - [Example: recovering buckets](#example-recovering-buckets)

## Bucket

AIS uses the popular-and-well-known bucket abstraction. In a flat storage hierarchy, bucket is a named container of user dataset(s) (represented as objects) and, simultaneously, a point of applying storage management policies: erasure coding, mirroring, etc.

Each object is assigned to (and stored in) a basic container called *bucket*. AIS buckets *contain* user data; in that sense they are very similar to:

* [Amazon S3 buckets](https://docs.aws.amazon.com/AmazonS3/latest/dev/UsingBucket.html)
* [Google Cloud (GCP) buckets](https://cloud.google.com/storage/docs/key-terms#buckets)
* [Microsoft Azure Blob containers](https://docs.microsoft.com/en-us/azure/storage/blobs/storage-blobs-introduction)

AIS supports two kinds of buckets: **ais buckets** and **3rd party Cloud-based buckets** (or simply **cloud buckets**).

All the [supported storage services](storage_svcs.md) equally apply to both kinds of buckets, with only a few exceptions. The following table summarizes them.

| Kind | Description | Supported Storage Services |
| --- | --- | --- |
| ais buckets | buckets that are **not** 3rd party Cloud-based. AIS buckets store user objects and support user-specified bucket properties (e.g., 3 copies). Unlike cloud buckets, ais buckets can be created through the [RESTful API](http_api.md). Similar to cloud buckets, ais buckets are distributed and balanced, content-wise, across the entire AIS cluster. | [Checksumming](storage_svcs.md#checksumming), [LRU (advanced usage)](storage_svcs.md#lru-for-local-buckets), [Erasure Coding](storage_svcs.md#erasure-coding), [Local Mirroring and Load Balancing](storage_svcs.md#local-mirroring-and-load-balancing) |
| cloud buckets | When AIS is deployed as [fast tier](/docs/overview.md#fast-tier), buckets in the cloud storage can be viewed and accessed through the [RESTful API](http_api.md) in AIS, in the exact same way as ais buckets. When this happens, AIS creates local instances of said buckets which then serves as a cache. These are referred to as **Cloud-based buckets** (or **cloud buckets** for short). | [Checksumming](storage_svcs.md#checksumming), [LRU](storage_svcs.md#lru), [Erasure Coding](storage_svcs.md#erasure-coding), [Local mirroring and load balancing](storage_svcs.md#local-mirroring-and-load-balancing) |

Cloud-based and ais buckets support the same API with minor exceptions. Cloud buckets can be *evicted* from AIS. AIS buckets are the only buckets that can be created, renamed, and deleted via the [RESTful API](http_api.md).

### Cloud Provider

[Cloud Provider](./providers.md) is an abstraction, and, simultaneously, an API-supported option that allows to delineate between "remote" and "local" buckets with respect to a given (any given) AIS cluster. For complete definition and details, plase refer to the [Cloud Provider](./providers.md) document.

Cloud provider is realized as an optional parameter in the GET, PUT, APPEND, DELETE and [Range/List](batch.md) operations with supported enumerated values that include:

* `ais` for ais buckets
* `aws` or `s3` - for Amazon S3 buckets
* `gcp` or `gs` - for Google Cloud
* `azure` - for Microsoft Blob Storage

* and finally, you can simple say `cloud` to designate any one of the 3 (three) Cloud providers listed above.

For API reference, please refer [to the RESTful API and examples](http_api.md). The rest of this document serves to further explain features and concepts specific to storage buckets.

## AIS Bucket
AIS buckets are the AIStore-own distributed buckets that are not associated with any 3rd party Cloud.

The [RESTful API](docs/http_api.md) can be used to create, rename and, destroy ais buckets.

New ais buckets must be given a unique name that does not duplicate any existing ais or cloud bucket.

### CLI examples: create, rename and, destroy ais bucket

To create an ais bucket with the name 'myBucket', rename it to 'myBucket2' and delete it, run:

```console
$ ais create bucket myBucket
$ ais rename bucket ais://myBucket ais://myBucket2
$ ais rm bucket ais://myBucket2
```

Please note that rename bucket is not an instant operation, especially if the bucket contains data. Follow the `rename` command tips to monitor when the operation completes.

### CLI example: working with remote AIS bucket

AIS clusters can be attached to each other, thus forming a global (and globally accessible) namespace of all individually hosted datasets. For background and details on AIS multi-clustering, please refer to this [document](providers.md).

The following example creates an attachment between two clusters, lists all remote buckets, and then list objects in one of those remote buckets (see comments inline):

```console

# attach remote AIS cluster and assign it an alias `teamZ` (for convenience and for future reference):
$ ais attach remote teamZ=http://cluster.ais.org:51080
Remote cluster (teamZ=http://cluster.ais.org:51080) successfully attached

# the cluster at http://cluster.ais.org:51080 is now persistently attached:
$ ais show remote
UUID      URL                            Alias     Primary      Smap   Targets  Online
MCBgkFqp  http://cluster.ais.org:51080   teamZ     p[primary]   v317   10       yes

# list all buckets in all remote clusters
# notice the syntax: by convention, we use `@` to prefix remote cluster UUIDs, and so
# `ais://@` translates as "AIS cloud provider, any remote cluster"

$ ais ls ais://@
AIS Buckets (4)
	  ais://@MCBgkFqp/imagenet
	  ais://@MCBgkFqp/coco
	  ais://@MCBgkFqp/imagenet-augmented
	  ais://@MCBgkFqp/imagenet-inflated

# list all buckets in the remote cluster with UUID = MCBgkFqp
# notice again the syntax: `ais://@some-string` translates as "remote AIS cluster with alias or UUID equal some-string"

$ ais ls ais://@MCBgkFqp
AIS Buckets (4)
	  ais://@MCBgkFqp/imagenet
	  ais://@MCBgkFqp/coco
	  ais://@MCBgkFqp/imagenet-augmented
	  ais://@MCBgkFqp/imagenet-inflated

# list all buckets with name matching the regex pattern "tes*"
$ ais ls --regex "tes*"
AWS Buckets (3)
  aws://test1
  aws://test2
  aws://test2

# we can conveniently keep using our previously selected alias for the remote cluster -
# the following lists selected remote bucket using the cluster's alias:
$ ais ls ais://@teamZ/imagenet-augmented
NAME              SIZE
train-001.tgz     153.52KiB
train-002.tgz     136.44KiB
...

# the same, but this time using the cluster's UUID:
$ ais ls ais://@MCBgkFqp/imagenet-augmented
NAME              SIZE
train-001.tgz     153.52KiB
train-002.tgz     136.44KiB
...
```

## Cloud Bucket

Cloud buckets are existing buckets in the 3rd party Cloud storage when AIS is deployed as [fast tier](/docs/overview.md#fast-tier).

> By default, AIS does not keep track of the cloud buckets in its configuration map. However, if users modify the properties of the cloud bucket, AIS will then keep track.

### Prefetch/Evict Objects

Objects within cloud buckets are automatically fetched into storage targets when accessed through AIS and are evicted based on the monitored capacity and configurable high/low watermarks when [LRU](storage_svcs.md#lru) is enabled.

The [RESTful API](docs/http_api.md) can be used to manually fetch a group of objects from the cloud bucket (called prefetch) into storage targets or to remove them from AIS (called evict).

Objects are prefetched or evicted using [List/Range Operations](batch.md#listrange-operations).

For example, to use a [list operation](batch.md#list) to prefetch 'o1', 'o2', and, 'o3' from Amazon S3 cloud bucket `abc`, run:

```console
$ ais start prefetch aws://abc --list o1,o2,o3
```

To use a [range operation](batch.md#range) to evict the 1000th to 2000th objects in the cloud bucket `abc` from AIS, which names begin with the prefix `__tst/test-`, run:

```console
$ ais evict aws://abc --template "__tst/test-{1000..2000}"
```

### Evict Cloud Bucket

Before a cloud bucket is accessed through AIS, the cluster has no awareness of the bucket.

Once there is a request to access the bucket, or a request to change the bucket's properties (see `set bucket props` in [REST API](http_api.md)), then the AIS cluster starts keeping track of the bucket.

In an evict bucket operation, AIS will remove all traces of the cloud bucket within the AIS cluster. This effectively resets the AIS cluster to the point before any requests to the bucket have been made. This does not affect the objects stored within the cloud bucket.

For example, to evict the `abc` cloud bucket from the AIS cluster, run:

```console
$ ais evict aws://myS3bucket
```

## Backend Bucket

So far, we have covered AIS and cloud buckets. These abstractions are sufficient for almost all use cases.  But there are times when we would like to download objects from an existing cloud bucket and then make use of the features available only for AIS buckets.

One way of accomplishing that could be:

* prefetch cloud objects
* create AIS bucket
* and then use the bucket-copying [API](http_api.md) or [CLI](/cmd/cli/resources/bucket.md) to copy over the objects from the cloud bucket to the newly created AIS bucket.

However, the extra-copying involved may prove to be time and/or space consuming. Hence, AIS-supported capability to establish an **ad-hoc** 1-to-1 relationship between a given AIS bucket and an existing cloud (*backend*).

> As aside, the term "backend" - something that is on the back, usually far (or farther) away - is often used for data redundancy, data caching, and/or data sharing. AIS *backend bucket* allows to achieve all of the above.

For example:

```console
$ ais create bucket abc
"abc" bucket created
$ ais set props ais://abc backend_bck=gcp://xyz
Bucket props successfully updated
```

After that, you can access all objects from `gcp://xyz` via `ais://abc`. **On-demand persistent caching** (from the `gcp://xyz`) becomes then automatically available, as well as **all other AIS-supported storage services** configurable on a per-bucket basis.

For example:

```console
$ ais ls gcp://xyz
NAME		 SIZE		 VERSION
shard-0.tar	 2.50KiB	 1
shard-1.tar	 2.50KiB	 1
$ ais ls ais://abc
NAME		 SIZE		 VERSION
shard-0.tar	 2.50KiB	 1
shard-1.tar	 2.50KiB	 1
$ ais get ais://abc/shard-0.tar /dev/null # cache/prefetch cloud object
"shard-0.tar" has the size 2.50KiB (2560 B)
$ ais ls ais://abc --cached
NAME		 SIZE		 VERSION
shard-0.tar	 2.50KiB	 1
$ ais set props ais://abc backend_bck=none # disconnect backend bucket
Bucket props successfully updated
$ ais ls ais://abc
NAME		 SIZE		 VERSION
shard-0.tar	 2.50KiB	 1
```

For more examples please refer to [CLI docs](/cmd/cli/resources/bucket.md#connectdisconnect-ais-bucket-tofrom-cloud-bucket).

## Bucket Access Attributes

Bucket access is controlled by a single 64-bit `access` value in the [Bucket Properties structure](../cmn/api.go), whereby its bits have the following mapping as far as allowed (or denied) operations:

| Operation | Bit Mask |
| --- | --- |
| GET | 0x1 |
| HEAD | 0x2 |
| PUT, APPEND | 0x4 |
| Cold GET | 0x8 |
| DELETE | 0x16 |

For instance, to make bucket `abc` read-only, execute the following [AIS CLI](../cmd/cli/README.md) command:

```console
$ ais set props abc 'access=ro'
```

The same expressed via `curl` will look as follows:

```console
$ curl -i -X PATCH  -H 'Content-Type: application/json' -d '{"action": "setbprops", "value": {"access": 18446744073709551587}}' http://localhost:8080/v1/buckets/abc
```

> 18446744073709551587 = 0xffffffffffffffe3 = 0xffffffffffffffff ^ (4|8|16)

## List Objects

ListObjects API returns a page of object names and, optionally, their properties (including sizes, access time, checksums, and more), in addition to a token that serves as a cursor or a marker for the *next* page retrieval.

Immutability of a bucket is assumed between subsequent ListObjects request, due to a local caching mechanism.
If a bucket has been updated after ListObjects request, a user should call ListObjectsInvalidateCache API to get
correct ListObjects results. This is the temporary requirement and will be removed in next AIS versions.

### Properties and options

The properties-and-options specifier must be a JSON-encoded structure, for instance '{"props": "size"}' (see examples). An empty structure '{}' results in getting just the names of the objects (from the specified bucket) with no other metadata.

| Property/Option | Description | Value |
| --- | --- | --- |
| props | The properties to return with object names | A comma-separated string containing any combination of: "checksum","size","atime","version","target_url","copies","status". <sup id="a1">[1](#ft1)</sup> |
| time_format | The standard by which times should be formatted | Any of the following [golang time constants](http://golang.org/pkg/time/#pkg-constants): RFC822, Stamp, StampMilli, RFC822Z, RFC1123, RFC1123Z, RFC3339. The default is RFC822. |
| prefix | The prefix which all returned objects must have | For example, "my/directory/structure/" |
| pagemarker | The token identifying the next page to retrieve | Returned in the "nextpage" field from a call to ListObjects that does not retrieve all keys. When the last key is retrieved, NextPage will be the empty string |
| pagesize | The maximum number of object names returned in response | Default value is 1000. GCP and ais bucket support greater page sizes. AWS is unable to return more than [1000 objects in one page](https://docs.aws.amazon.com/AmazonS3/latest/API/RESTBucketGET.html) |
| fast | Perform fast traversal of bucket contents | If `true`, the list of objects is generated much faster but the result is less accurate and has a few limitations: the only name of object is returned(props is ignored) and paging is unsupported as it always returns the entire bucket list(unless prefix is defined) |
| cached | Return only objects that are cached on local drives | For ais buckets the option is ignored. For cloud buckets, if `cached` is `true`, the cluster does not retrieve any data from the cloud, it reads only information from local drives |
| taskid | ID of the list objects operation (string) | Listing objects is an asynchronous operation. First, a client should start the operation by sending `"0"` as `taskid` - `"0"` means initialize a new list operation. In response, a proxy returns a `taskid` generated for the operation. Then the client should poll the operation status using the same JSON-encoded structure but with `taskid` set to the received value. If the operation is still in progress the proxy returns status code 202(Accepted) and an empty body. If the operation is completed, it returns 200(OK) and the list of objects. The proxy can return status 410(Gone) indicating that the operation restarted and got a new ID. In this case, the client should read new operation ID from the response body |

The full list of bucket properties are:

| Bucket Property | JSON | Description | Fields |
| --- | --- | --- | --- |
| Provider | `provider` | "aws", "gcp" or "ais" | `"provider": "aws"/"gcp"/"ais"` |
| Cksum | `checksum` | Please refer to [Supported Checksums and Brief Theory of Operations](checksum.md) | |
| LRU | `lru` | Configuration for [LRU](storage_svcs.md#lru). `lowwm` and `highwm` is the used capacity low-watermark and high-watermark (% of total local storage capacity) respectively. `out_of_space` if exceeded, the target starts failing new PUTs and keeps failing them until its local used-cap gets back below `highwm`. `atime_cache_max` represents the maximum number of entries. `dont_evict_time` denotes the period of time during which eviction of an object is forbidden [atime, atime + `dont_evict_time`]. `capacity_upd_time` denotes the frequency at which AIStore updates local capacity utilization. `enabled` LRU will only run when set to true. | `"lru": { "lowwm": int64, "highwm": int64, "out_of_space": int64, "atime_cache_max": int64, "dont_evict_time": "120m", "capacity_upd_time": "10m", "enabled": bool }` |
| Mirror | `mirror` | Configuration for [Mirroring](storage_svcs.md#local-mirroring-and-load-balancing). `copies` represents the number of local copies. `burst_buffer` represents channel buffer size.  `util_thresh` represents the threshold when utilizations are considered equivalent. `optimize_put` represents the optimization objective. `enabled` will only generate local copies when set to true. | `"mirror": { "copies": int64, "burst_buffer": int64, "util_thresh": int64, "optimize_put": bool, "enabled": bool }` |
| EC | `ec` | Configuration for [erasure coding](storage_svcs.md#erasure-coding). `objsize_limit` is the limit in which objects below this size are replicated instead of EC'ed. `data_slices` represents the number of data slices. `parity_slices` represents the number of parity slices/replicas. `enabled` represents if EC is enabled. | `"ec": { "objsize_limit": int64, "data_slices": int, "parity_slices": int, "enabled": bool }` |
| Versioning | `versioning` | Configuration for object versioning support. `enabled` represents if object versioning is enabled for a bucket. For Cloud-based bucket, its versioning must be enabled in the cloud prior to enabling on AIS side. `validate_warm_get`: determines if the object's version is checked(if in Cloud-based bucket) | `"versioning": { "enabled": true, "validate_warm_get": false }`|
| AccessAttrs | `access` | Bucket access [attributes](#bucket-access-attributes). Default value is 0 - full access | `"access": "0" ` |
| BID | `bid` | Readonly property: unique bucket ID  | `"bid": "10e45"` |
| Created | `created` | Readonly property: bucket creation date, in nanoseconds(Unix time) | `"created": "1546300800000000000"` |

`SetBucketProps` allows the following configurations to be changed:

| Property | Type | Description |
| --- | --- | --- |
| `ec.enabled` | bool | enables EC on the bucket |
| `ec.data_slices` | int | number of data slices for EC |
| `ec.parity_slices` | int | number of parity slices for EC |
| `ec.objsize_limit` | int | size limit in which objects below this size are replicated instead of EC'ed |
| `ec.compression` | string | LZ4 compression parameters used when EC sends its fragments and replicas over network |
| `mirror.enabled` | bool | enable local mirroring |
| `mirror.copies` | int | number of local copies |
| `mirror.util_thresh` | int | threshold when utilizations are considered equivalent |

 <a name="ft1">1</a>: The objects that exist in the Cloud but are not present in the AIStore cache will have their atime property empty (""). The atime (access time) property is supported for the objects that are present in the AIStore cache. [↩](#a1)

### CLI examples: listing and setting bucket properties

1. List bucket properties:

```console
$ ais show props mybucket
```

or, the same to get output in a (raw) JSON form:

```console
$ ais show props mybucket --json
```

2. Enable erasure coding on a bucket:

```console
$ ais set props mybucket ec.enabled=true
```

3. Enable object versioning and then list updated bucket properties:

```console
$ ais set props mybucket ver.enabled=true
$ ais show props mybucket
```
