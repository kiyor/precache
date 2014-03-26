<!-----------------------------

- File Name : README.md

- Purpose :

- Creation Date : 03-25-2014

- Last Modified : Tue Mar 25 18:06:45 2014

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

[precache]
hosts = host1.com host2.com host3.com
vhost = vhost.com
proc = 8


```

then run

`precache -f config.ini`
this command will give you file list, you able to check file list before you do anything

`precache -f config.ini -do`
this command will starting requesting file

currently it will compare last mod time and content size, if the file not getting X-Cache header HIT, it will request again after 0.5 seconds

currently not do purge, but code already write, change whatever you need.

Do whatever you want to.