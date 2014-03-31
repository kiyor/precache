/* -.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.

* File Name : precache.go

* Purpose :

* Creation Date : 03-19-2014

* Last Modified : Sat 29 Mar 2014 11:10:35 PM UTC

* Created By : Kiyor

_._._._._._._._._._._._._._._._._._._._._.*/

package main

import (
	"crypto/md5"
	"encoding/base64"
	"flag"
	"fmt"
	"github.com/dustin/go-humanize"
	"github.com/kiyor/gfind/lib"
	"github.com/vaughan0/go-ini"
	"github.com/wsxiaoys/terminal/color"
	"io"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type client struct {
	c   *http.Client
	loc *bool
}

func (c *client) lock() {
	*c.loc = true
}

func (c *client) unlock() {
	*c.loc = false
}

type worker struct {
	sync.WaitGroup
	counter *int
}

func (w *worker) add(n int) {
	w.Add(n)
	*w.counter += n
}

func (w *worker) done() {
	*w.counter--
	w.Done()
}

func initClient() client {
	c := &http.Client{Transport: &transport}
	return client{c, new(bool)}
}

func dialTimeout(network, addr string) (net.Conn, error) {
	return net.DialTimeout(network, addr, timeout)
}

func getClient(clients []client) int {
	for {
		for k, c := range clients {
			if !*c.loc {
				c.lock()
				return k
			}
		}
		time.Sleep(1 * time.Millisecond)
	}
}

var (
	fhosts *string = flag.String("hosts", "server.com", "http hostname")
	fvhost *string = flag.String("vhost", "client.com", "vhost hostname")
	do     *bool   = flag.Bool("do", false, "do request")

	fproc *int    = flag.Int("proc", 0, "worker")
	conff *string = flag.String("f", "conf.ini", "conf file location")

	purge           *bool   = flag.Bool("purgefirst", false, "do purge request before anything")
	ngxSecurityLink *string = flag.String("ngxsecuritylink", "", "Nginx Security Link security key")
	cStatus         *string = flag.String("cachestatus", "X-Cache", "Nginx Upstream status Header")
	timeOut         *string = flag.String("timeout", "0s", "request timeout, default will overwrite to 3s")

	verbose *bool = flag.Bool("v", false, "verbose")

	rootdir, vhost, security, cacheStatus string
	hosts, httphosts                      []string
	proc                                  int
	timeout                               time.Duration
	clients                               []client
	transport                             = http.Transport{
		Dial: dialTimeout,
	}
)

func readConff() {
	f, err := ini.LoadFile(*conff)
	chkErr(err)
	hs, _ := f.Get("precache", "hosts")
	hosts = strings.Split(hs, " ")
	vhost, _ = f.Get("precache", "vhost")
	security, _ = f.Get("precache", "security")
	cacheStatus, _ = f.Get("precache", "cachestatus")
	tiout, ok := f.Get("precache", "timeout")
	if !ok {
		timeout = 3 * time.Second
	} else {
		timeout, err = time.ParseDuration(tiout)
		chkErr(err)
	}

	p, ok := f.Get("precache", "proc")
	if !ok {
		p = "2"
	}
	if p != "" {
		proc, err = strconv.Atoi(p)
		if err != nil {
			fmt.Println("proc should be number")
		}
	}

	if *fhosts != "server.com" {
		hosts = strings.Split(*fhosts, " ")
	}
	if *fvhost != "client.com" {
		vhost = *fvhost
	}
	if *ngxSecurityLink != "" {
		security = *ngxSecurityLink
	}
	if *timeOut != "0s" {
		timeout, err = time.ParseDuration(*timeOut)
		chkErr(err)
	}
	if *cStatus != "X-Cache" {
		cacheStatus = *cStatus
	} else if cacheStatus == "" {
		cacheStatus = "X-Cache"
	}
	if ok && *fproc == 0 {

	} else if *fproc == 0 {
		proc = 2
	} else {
		proc = *fproc
	}

}

func init() {
	flag.Parse()
	runtime.GOMAXPROCS(runtime.NumCPU())

	readConff()
	for i := 0; i < proc; i++ {
		clients = append(clients, initClient())
	}
	for _, host := range hosts {
		host = "http://" + host
		httphosts = append(httphosts, host)
	}
}

func time2fmt(t time.Time) string {
	return t.Format("Mon, 02 Jan 2006 15:04:05 GMT")
}

func timespec2time(t syscall.Timespec) time.Time {
	return time.Unix(t.Sec, t.Nsec)
}

func main() {
	conf := gfind.InitFindConfByIni(*conff)
	fs := gfind.Find(conf)
	if *do {
		var wg sync.WaitGroup
		for _, h := range httphosts {
			wg.Add(1)
			go Gourl(h, fs, &wg)
		}
		wg.Wait()
	} else {
		gfind.Output(fs, *verbose)
	}
}

func Gourl(host string, fs []gfind.MyFile, wg *sync.WaitGroup) {
	var w worker
	w.counter = new(int)
	for k, v := range fs {
		w.add(1)
		id := getClient(clients)
		go func(id int, v gfind.MyFile, w *worker, k int) {
			clients[id].gogogo(host, v, k, id)
			w.done()
		}(id, v, &w, k)
	}
	w.Wait()
	wg.Done()
}

func (c *client) purge(host string, f gfind.MyFile, k int, id int) {
	var Url *url.URL
	Url, err := url.Parse(host)
	chkErr(err)
	Url.Path += "/purge"
	Url.Path += f.Relpath
	var u string
	u = Url.String()
	if security != "" {
		u += "?" + ParseNgxSecurityLink(security, host, f)
	}
	req, err := http.NewRequest("HEAD", u, nil)
	req.Close = true
	if vhost != "client.com" {
		req.Host = vhost
	}
	resp, err := c.c.Do(req)
	chkErr(err)
	if resp.StatusCode == 200 {
		color.Printf("%6v-%-2d @{g}%-17v@{|} %v %v\n", k, id, "PURGE:SUCCESS", Url.String(), f.Size)
	} else if resp.StatusCode == 404 {
		color.Printf("%6v-%-2d @{y}%-17v@{|} %v %v\n", k, id, "PURGE:NOFILE", Url.String(), f.Size)
	}

}

func ParseNgxSecurityLink(key string, host string, f gfind.MyFile) string {
	var Url *url.URL
	Url, err := url.Parse(host)
	chkErr(err)
	Url.Path += f.Relpath
	loc := Url.String()[len(host):]
	token := key + loc + "9999999999"
	h := md5.New()
	io.WriteString(h, token)
	str := base64.StdEncoding.EncodeToString(h.Sum(nil))
	r := strings.NewReplacer("/", "_", "=", "", "+", "-")
	return "st=" + r.Replace(str) + "&e=9999999999"
}

func (c *client) gogogo(host string, f gfind.MyFile, k int, id int) {
	if *purge {
		c.purge(host, f, k, id)
	}
	var Url *url.URL
	Url, err := url.Parse(host)
	chkErr(err)
	Url.Path += f.Relpath
	var u string
	u = Url.String()
	if security != "" {
		u += "?" + ParseNgxSecurityLink(security, host, f)
	}
	color.Printf("%6v-%-2d @{y}%-17v@{|} %v %v\n", k, id, "START", Url.String(), f.Size)
	req, err := http.NewRequest("HEAD", u, nil)
	req.Close = true
	if vhost != "client.com" {
		req.Host = vhost
	}
	for true {
		resp, err := c.c.Do(req)
		if err != nil {
			color.Printf("@{g}%v\n", err.Error())
			time.Sleep(5 * time.Second)
			continue
		}

		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			color.Printf("@{g}%6v-%-2d %v %v %v\n", k, id, Url.String(), "StatusCode", resp.StatusCode)
			time.Sleep(5 * time.Second)
			continue
		}
		var b1, b2, b3 bool

		if val, ok := resp.Header[cacheStatus]; ok {
			if val[0] == "HIT" {
				b1 = true
			} else if val[0] == "UPDATING" || val[0] == "MISS" {
				t, err := time.ParseDuration(fmt.Sprintf("%dms", f.Size/100000))
				chkErr(err)
				if *verbose {
					color.Printf("%6v-%-2d @{r}%7s:%-9s@{|} %v Size:%v SLEEP:%vms\n", k, id, cacheStatus, val[0], Url.String(), humanize.IBytes(uint64(f.Size)), f.Size/100000)
				}
				time.Sleep(t)
				continue
			} else {
				if *verbose {
					color.Printf("%6v-%-2d @{r}%7s:%-9s@{|} %v Size:%v SLEEP\n", k, id, cacheStatus, val[0], Url.String(), humanize.IBytes(uint64(f.Size)))
				}
				time.Sleep(2 * time.Second)
				continue
			}
		} else {
			if *verbose {
				color.Printf("%6v-%-2d @{r}%7s:%-9v@{|} %v SLEEP\n", k, id, cacheStatus, "NULL", Url.String())
			}
			time.Sleep(5 * time.Second)
			continue
		}
		if val, ok := resp.Header["Last-Modified"]; ok {
			if val[0] == time2fmt(timespec2time(f.Stat.Mtim)) {
				b2 = true
			} else {
				color.Printf("@{b}%6v-%-2d %v %v %v %v %v\n", k, id, Url.String(), " Header: ", val[0], " File: ", time2fmt(timespec2time(f.Stat.Mtim)))
				c.purge(host, f, k, id)
				continue
			}
		} else {
			color.Printf("@{g}%6v-%-2d %-17v %v %v\n", k, id, "NO:Last-Modified", Url.String())
		}
		if val, ok := resp.Header["Content-Length"]; ok {
			s, _ := strconv.Atoi(val[0])
			if int64(s) == f.Size {
				b3 = true
			} else {
				color.Printf("@{b}%6v-%-2d %v %v %v %v %v\n", k, id, Url.String(), " Header: ", val[0], " File: ", f.Size)
				c.purge(host, f, k, id)
				continue
			}
		} else {
			color.Printf("@{g}%6v-%-2d %-17v %v %v\n", k, id, "NO:Content-Length", Url.String())
		}

		// if cache hit, size and last mod match, then break loop
		if b1 && b2 && b3 {
			if *verbose {
				color.Printf("%6v-%-2d @{g}%-17v@{|} %v %v\n", k, id, "FINISH", Url.String(), f.Size)
			}
			break
		}

		time.Sleep(5000 * time.Millisecond)
	}
	c.unlock()
}

func chkErr(err error) {
	if err != nil {
		fmt.Println(err.Error())
	}
}
