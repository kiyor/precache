/* -.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.

* File Name : find.go

* Purpose :

* Creation Date : 03-19-2014

* Last Modified : Wed 26 Mar 2014 01:03:44 AM UTC

* Created By : Kiyor

_._._._._._._._._._._._._._._._._._._._._.*/

package main

import (
	"flag"
	"fmt"
	"github.com/kiyor/gfind/lib"
	"github.com/vaughan0/go-ini"
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

	fproc *int    = flag.Int("proc", 2, "worker")
	conff *string = flag.String("f", "conf.ini", "conf file location")

	verbose *bool = flag.Bool("v", false, "verbose")

	rootdir, vhost   string
	hosts, httphosts []string
	proc             int
	clients          []client
	timeout          = 3 * time.Second
	transport        = http.Transport{
		Dial: dialTimeout,
	}
)

func readConff() {
	f, err := ini.LoadFile(*conff)
	chkErr(err)
	hs, _ := f.Get("precache", "hosts")
	hosts = strings.Split(hs, " ")
	vhost, _ = f.Get("precache", "vhost")
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
	if ok && *fproc == 2 {

	} else {
		proc = *fproc
	}

}

func init() {
	flag.Parse()
	runtime.GOMAXPROCS(runtime.NumCPU())

	hs := *fhosts
	hosts = strings.Split(hs, " ")
	vhost = *fvhost
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
			if *verbose {
				fmt.Println("id:", id, v.Path)
			}
			clients[id].gogogo(host, v, k)
			w.done()
		}(id, v, &w, k)
	}
	w.Wait()
	wg.Done()
}

func (c *client) purge(host string, f gfind.MyFile) {
	var Url *url.URL
	Url, err := url.Parse(host)
	chkErr(err)
	Url.Path += "/purge"
	Url.Path += f.Relpath
	req, err := http.NewRequest("HEAD", Url.String(), nil)
	req.Close = true
	if vhost != "client.com" {
		req.Host = vhost
	}
	c.c.Do(req)
}

func (c *client) gogogo(host string, f gfind.MyFile, k int) {
	var Url *url.URL
	Url, err := url.Parse(host)
	chkErr(err)
	Url.Path += f.Relpath
	fmt.Println(k, Url.String(), f.Size)
	req, err := http.NewRequest("HEAD", Url.String(), nil)
	req.Close = true
	if vhost != "client.com" {
		req.Host = vhost
	}
	for true {
		resp, err := c.c.Do(req)
		if err != nil {
			fmt.Println(err.Error())
			continue
		}

		defer resp.Body.Close()
		var b1, b2, b3 bool
		//
		// 		fmt.Println(resp.StatusCode, Url.String())
		// 		break
		//
		if val, ok := resp.Header["X-Cache"]; ok {
			if val[0] == "HIT" {
				b1 = true
			}
		}
		if val, ok := resp.Header["Last-Modified"]; ok {
			if val[0] == time2fmt(timespec2time(f.Stat.Mtim)) {
				b2 = true
			} else {
				fmt.Println(f.Path, " Header: ", val[0], " File: ", time2fmt(timespec2time(f.Stat.Mtim)))
				b2 = true
			}
		}
		if val, ok := resp.Header["Content-Length"]; ok {
			s, _ := strconv.Atoi(val[0])
			if int64(s) == f.Size {
				b3 = true
			} else {
				fmt.Println(f.Path, " Header: ", val[0], " File: ", f.Size)
				b3 = true
			}
		}
		if b1 && b2 && b3 {
			break
		}

		time.Sleep(500 * time.Millisecond)
	}
	c.unlock()
}

func chkErr(err error) {
	if err != nil {
		fmt.Println(err.Error())
	}
}
