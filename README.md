<!-----------------------------

- File Name : README.md

- Purpose :

- Creation Date : 03-25-2014

- Last Modified : Mon 31 Mar 2014 10:17:41 PM UTC

- Created By : Kiyor

------------------------------->

# precache

for nginx to pre cache file when you are in origin server, in origin server, this will loop file in your location and start pre cache files, support multi hosts at same time and able to change number of workers.

## How to use

create a config file whatever name, make it looks like that

```

[gfind]
dir = /data/user/public_html
rootdir = /data/user/public_html
type = f
mtime = 1
ctime = 1

[precache]
hosts = host1.com host2.com host3.com
vhost = vhost.com
proc = 8
; this is the header you setup in nginx like `add_header X-Cache $upstream_cache_status;`
cachestatus = X-Cache
; header request timeout
timeout = 3s
; if you have [HttpSecureLinkModule](http://wiki.nginx.org/HttpSecureLinkModule)
security = securityKey

```

then run

`precache -f config.ini`
  
this command will give you file list, you able to check file list before you do anything
  

`precache -f config.ini -do`
  
this command will starting requesting file

you can add flag -purgefirst to purge file before precache, it use for some time.
  
currently it will compare last mod time and content size, if the file not getting X-Cache header HIT, it will request again after 1 seconds

currently not do purge, but code already write, change whatever you need.
  
You'd better add proxy_cache_lock in your nginx config, otherwise it will fucked up your nginx server;
  
```
proxy_cache_lock on;
proxy_cache_lock_timeout 300s;
proxy_cache_use_stale updating;

```

Do whatever you want to.
