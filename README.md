# README

Export documents from a SOLR index as JSON, fast and simply from the command line.

## Requirements

SOLR 4.7 or higher, since the cursor mechanism was introduced with SOLR 4.7
([2014-02-25](https://archive.apache.org/dist/lucene/solr/4.7.0/)) &mdash; see
also [efficient deep paging with cursors](https://solr.pl/en/2014/03/10/solr-4-7-efficient-deep-paging/).

## Installation

1. Via debian or rpm [package](https://github.com/gobars/solrdump/releases).
2. Or via go tool: `go install github.com/gobars/solrdump@latest`

## Features

1. `_version_` deleted from the result

## Usage

```shell
Usage of solrdump (0.1.2):
  -max int       Max number of rows (default 100)
  -q string      SOLR query (default "*:*")
  -rows int      Number of rows returned per request (default 100)
  -server string SOLR server with index name, eg. localhost:8983/solr/example
  -version       Show version and exit
  -remove-fields Remove fields, _version_ defaulted
  -output        Output file, or http url
  -v             Verbose, -v -vv -vvv
```

## Print docs

```shell
$ solrdump -server 192.168.126.16:8983/solr/zz -max 3          
2021/06/03 17:57:50 http://192.168.126.16:8983/solr/zz/select?cursorMark=%2A&fl=&q=%2A%3A%2A&rows=3&sort=id+asc&wt=json
{"dataType":"INTEGRATION","id":"000007c8-3d83-47c7-b9f0-1e0d15670599","createdDate":"2020-08-06T06:49:37Z"}
{"dataType":"MANUAL","id":"00004d53-d76d-43c3-906d-90ff475bd1a2","createdDate":"2021-05-10T08:14:14Z"}
{"dataType":"MANUAL","id":"000070fe-309f-4755-998e-2445cc66ef9f","createdDate":"2021-05-10T08:14:14Z"}
2021/06/03 17:57:50 fetched 3/509309 docs
```

### Write to elastic search

```sh
➜  500px solrdump -server 192.168.126.16:8983/solr/licenseIndex -max 3 -output 192.168.126.5:9202/bench/zz -v
2021/06/04 13:08:02 http://192.168.126.16:8983/solr/licenseIndex/select?cursorMark=%2A&fl=&q=%2A%3A%2A&rows=3&sort=id+asc&wt=json
2021/06/04 13:08:03 sent cost: 1.020677367s status: 201
2021/06/04 13:08:03 sent cost: 14.077046ms status: 201
2021/06/04 13:08:03 sent cost: 9.916851ms status: 201
```

## Resources

1. [o19s/solr-to-es](https://github.com/o19s/solr-to-es)
2. [solr cursor select query](https://github.com/frizner/glsolr)
3. [frizner/solrdump](https://github.com/frizner/solrdump)

## Pagination of Results

* https://cwiki.apache.org/confluence/display/solr/Pagination+of+Results

Requesting large number of documents from SOLR can lead to *Deep Paging*
problems:

> When you wish to fetch a very large number of sorted results from Solr to
> feed into an external system, using very large values for the start or rows
> parameters can be very inefficient.

See also: *Fetching A Large Number of Sorted Results: Cursors*

> As an alternative to increasing the "start" parameter to request subsequent
> pages of sorted results, Solr supports using a "Cursor" to scan through
> results. Cursors in Solr are a logical concept, that doesn't involve caching
> any state information on the server. Instead the sort values of the last
> document returned to the client are used to compute a "mark" representing a
> logical point in the ordered space of sort values.

`http://192.168.126.16:8983/solr/zz/select?q=*:*&wt=json&cursorMark=*&sort=id asc`

```json
{
  "responseHeader": {
    "status": 0,
    "QTime": 13,
    "params": {
      "q": "*:*",
      "cursorMark": "*",
      "sort": "id asc",
      "wt": "json"
    }
  },
  "response": {
    "numFound": 509309,
    "start": 0,
    "docs": [
      {
        "name": "测试",
        "id": "00013dd9-7326-43d7-977d-60cdab8deb95",
        "createdBy": "4f070706-c30c-481c-a837-9f39136c62de",
        "createdDate": "2021-05-10T08:14:14Z",
        "lastModifiedBy": "4f070706-c30c-481c-a837-9f39136c62de",
        "lastModifiedDate": "2021-05-28T04:33:22Z",
        "_version_": 1700975244065374200
      }
    ]
  },
  "nextCursorMark": "AoE/BTAwMDEzZGQ5LTczMjYtNDNkNy05NzdkLTYwY2RhYjhkZWI5NQ=="
}
```
