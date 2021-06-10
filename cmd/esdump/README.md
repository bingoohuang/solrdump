# esdump


## usage

```sh
🕙[2021-06-09 23:23:52.630] ❯ esdump -query '{"size":3}' -max 3 
2021/06/09 23:23:59 total hists 3, cost 7.674592ms
2021/06/09 23:23:59 0000000001:{"idCode":"700a28db-8f26-4133-95a1-fdda48afb6dc","holderName":"阮蛉佦","holderNum":"426769199201221245","areaCode":"885845","createdDate":"2052-02-18T23:39:26Z"}
2021/06/09 23:23:59 0000000002:{"idCode":"70c8aabb-6ae6-46eb-b37a-91d02998163c","holderName":"皇甫癕器","holderNum":"443488201811286840","areaCode":"417569","createdDate":"1999-02-25T14:10:17Z"}
2021/06/09 23:23:59 0000000003:{"idCode":"a85c3edc-367c-4140-b6ab-2a398989e7f6","holderName":"童控櫭","holderNum":"214013201803237761","areaCode":"025853","createdDate":"2043-06-30T03:17:57Z"}

🕙[2021-06-09 23:23:59.562] ❯ esdump -query '{"size":3,"_source":["holderNum"]}' -max 3 -filter 'hits.hits.#._source.holderNum' 
2021/06/09 23:24:40 total hists 3, cost 10.133389ms
2021/06/09 23:24:40 0000000001:426769199201221245
2021/06/09 23:24:40 0000000002:443488201811286840
2021/06/09 23:24:40 0000000003:214013201803237761
```

## help

```sh
🕙[2021-06-09 23:25:38.706] ❯ esdump -h
Usage of esdump (0.1.0 2021-06-09 22:52:44):
  -es    string  Elasticsearch address, default 127.0.0.1:9202
  -index string  Index name, default zz
  -type  string  Index type, default _doc
  -scroll string Scroll time ttl, default 1m
  -max      int  Max docs to dump, default 10000
  -query string  Query json, like {"size":10000,"_source":["holderNum"]}
  -version       Show version and exit
  -filter string Filter expression, like hits.hits.#._source.holderIdentityNum.0, default hits.hits.#._source
  -out           Output, empty/stdout to stdout, else to badger path.
  -v             Verbose, -vv -vvv
```


## badger output

```sh
🕙[2021-06-09 23:27:49.288] ❯ esdump -query '{"size":3}' -max 3  -out badger-zz
2021/06/09 23:27:53 total hists 3, cost 7.869417ms
🕙[2021-06-10 00:24:54.689] ❯ esdump -out badger-zz -view-badger 10                                                        
0: {"idCode":"700a28db-8f26-4133-95a1-fdda48afb6dc","holderName":"阮蛉佦","holderNum":"426769199201221245","areaCode":"885845","createdDate":"2052-02-18T23:39:26Z"}
1: {"idCode":"70c8aabb-6ae6-46eb-b37a-91d02998163c","holderName":"皇甫癕器","holderNum":"443488201811286840","areaCode":"417569","createdDate":"1999-02-25T14:10:17Z"}
2: {"idCode":"a85c3edc-367c-4140-b6ab-2a398989e7f6","holderName":"童控櫭","holderNum":"214013201803237761","areaCode":"025853","createdDate":"2043-06-30T03:17:57Z"}
```

```sh
$ esdump -es 192.168.126.5:9202 -index license -type docs -query '{"size":10,"_source":["holderIdentityNum"]}' -filter 'hits.hits.#._source.holderIdentityNum.0' -max 10 -out bb-license         
2021/06/10 10:43:10 total hists 10, cost 725.982604ms
```

## badger open

1. 18G 的 badgerdb ，打开需要1分钟时间。
1. 18G 存储了6亿数据。

```sh
[root@fs04-192-168-126-5 bingoo]# esdump -out es-badger-db -view-badger 10
2021/06/10 13:49:28 badgerdb es-badger-db openning
2021/06/10 13:50:35 badgerdb es-badger-db opened
2021/06/10 13:50:35 start to walk
0: 627489197812144689
1: 510064200811063514
2: 644229200803277613
3: 658758198006161655
4: 710375198411203330
5: 440938201007184668
6: 521322197112095868
7: 326202198810269571
8: 368058199312158654
9: 139705198201143344
2021/06/10 13:50:35 end to walk
[root@fs04-192-168-126-5 bingoo]# du -sh es-badger-db/
18G	es-badger-db/
[root@fs04-192-168-126-5 bingoo]# tail -1 esdumo.nohup
total hists 606931329, cost 8h56m4.261514373s
```
