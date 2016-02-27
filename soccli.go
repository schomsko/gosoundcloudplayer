package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/kardianos/osext"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// contains settings
type Setting struct {
	MinD int
	MaxD int
}

func (s *Setting) setMinD(v string) {
	vi, _ := strconv.Atoi(v)
	s.MinD = vi * 60 * 1000
}
func (s *Setting) setMaxD(v string) {
	vi, _ := strconv.Atoi(v)
	s.MaxD = vi * 60 * 1000
}

// represents a soundcloud user account
type SoundCloudUser struct {
	Id        int
	Username  string
	City      string
	Website   string
	Full_name string
}

// represents a search result
type SearchResult struct {
	Title             string
	Created_at        string
	Duration          int
	Stream_url        string
	Description       string
	Permalink_url     string
	Download_url      string
	User              SoundCloudUser
	CreatedAtFormated string
	Downloadable      bool
}

// all search results of a search
type SearchResults []*SearchResult

func (s SearchResults) Len() int      { return len(s) }
func (s SearchResults) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// a type that has a comparator method
type ByLength struct{ SearchResults }

func (s ByLength) Less(i, j int) bool {
	return s.SearchResults[i].Duration < s.SearchResults[j].Duration
}

// a type that has a comparator method
type ByAge struct{ SearchResults }

func (s ByAge) Less(i, j int) bool {
	return s.SearchResults[i].Created_at < s.SearchResults[j].Created_at
}

type Player struct {
	srs       SearchResults
	client_id []byte
	vlc       *os.Process
	li        string // last input
	setting   Setting
}

// exits track or program
func (p *Player) exit() {
	if p.vlc == nil {
		os.Exit(0)
	} else {
		p.vlc.Kill()
		p.vlc = nil
	}
}

// sets Settings
func (p *Player) set() {
	p.li = strings.TrimLeft(p.li, "set ")
	iLsl := strings.Split(p.li, " ")
	if iLsl[0] == "range" {
		p.setting.setMinD(iLsl[1])
		p.setting.setMaxD(iLsl[2])
	}
}

// displays info about a track
func (p *Player) info() {
	p.li = strings.TrimLeft(p.li, "i ")
	index, _ := strconv.Atoi(string(p.li))
	println("\x1b[33m" + p.srs[index].Description + "\x1b[0m")
}

// start and end vlc processes
func (p *Player) killAndPlay() {
	var err error
	var vlcexe *exec.Cmd
	var done chan bool
	i, _ := strconv.Atoi(p.li)
	fmt.Fprintf(os.Stdout, "Playing %s ...\n", p.srs[i].Title)
	fmt.Fprintf(os.Stdout, "Link: \n%s\n", p.srs[i].Permalink_url)
	fmt.Fprintf(os.Stdout, "Stream: \n%s?client_id=%s\n", p.srs[i].Stream_url, p.client_id)
	fmt.Fprintf(os.Stdout, "Download: \n%s?client_id=%s\n", p.srs[i].Download_url, p.client_id)

	durl := fmt.Sprintf("%s?client_id=%s", p.srs[i].Download_url, p.client_id)
	surl := fmt.Sprintf("%s?client_id=%s", p.srs[i].Stream_url, p.client_id)

	if !p.srs[i].Downloadable {
		vlcexe = exec.Command("/Applications/VLC.app/Contents/MacOS/VLC", surl)
		err = vlcexe.Start()
		println("Streaming 128kbps ... bummer!")
	} else {
		client := &http.Client{}
		resp, _ := client.Get(durl)

		done := make(chan bool, 1)
		go copyToTmp(resp, done)

		time.Sleep(time.Second)
		unixFileCmd := exec.Command("file", "/tmp/scpfile")
		out, _ := unixFileCmd.Output()
		fmt.Printf("Downloading and playing from local file: %s\n", out)

		//vlcexe = exec.Command("/Applications/VLC.app/Contents/MacOS/VLC", "/tmp/scpfile")
		vlcexe = exec.Command("afplay", "-v", "0.7", "/tmp/scpfile")
		err = vlcexe.Start()
	}
	if err != nil {
		log.Fatal(err)
	} else if p.vlc != nil {
		p.vlc.Kill()
	}
	p.vlc = vlcexe.Process
	<-done
}

func copyToTmp(resp *http.Response, done chan bool) {
	file, _ := os.Create("/tmp/scpfile")
	io.Copy(file, resp.Body)
	fmt.Printf("%s\n", "Download finished")
	done <- true
}

// search and fill result object
func (p *Player) searchSoundCloud() {
	fmt.Fprintf(os.Stdout, "Searching %s ...\n\n", p.li)
	query := fmt.Sprintf("http://api.soundcloud.com/tracks.json?"+
		"client_id=%s&q=%s"+
		"&duration[from]="+fmt.Sprint(p.setting.MinD)+
		"&duration[to]="+fmt.Sprint(p.setting.MaxD)+
		"&filter=streamable,public", p.client_id,
		strings.Replace(p.li, " ", "+", -1))
	res, err := http.Get(query)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("... end of search\n")
	resbody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Fatal(err)
	}
	res.Body.Close()
	err = json.Unmarshal(resbody, &p.srs)
	sort.Sort(ByLength{p.srs})
	sort.Sort(ByAge{p.srs})
}

// display results
func (p *Player) showResultList() {
	var rank string
	var m30Max int = 0
	for _, v := range p.srs {
		duration, _ := time.ParseDuration(fmt.Sprintf("%d%s", v.Duration/1000, "s"))
		m30 := int(duration.Minutes()) / 30
		if m30Max < m30 {
			m30Max = m30
		}
	}
	for k, v := range p.srs {
		if k <= 9 {
			rank = " " + strconv.Itoa(k)
		} else {
			rank = "" + strconv.Itoa(k)
		}
		// convert duration
		duration, _ := time.ParseDuration(fmt.Sprintf("%d%s", v.Duration/1000, "s"))
		d := duration.String()
		// build visual duration indicator
		m30 := int(duration.Minutes()) / 30
		lengthIndicator := bytes.NewBufferString("")
		for i := 0; i < m30Max; i++ {
			if i < m30 {
				fmt.Fprint(lengthIndicator, "-")
			} else {
				fmt.Fprint(lengthIndicator, " ")
			}
		}

		// indicate description using info symbol '[i]'
		descAvail := "    "
		if v.Description != "" {
			descAvail = " \x1b[33m[i]\x1b[0m"
		}
		if !v.Downloadable {
			rank = "\x1b[37m" + rank + "\x1b[0m"
		}
		createdtime, _ := time.Parse("2006/01/02 15:04:05 +0000", v.Created_at)
		fmt.Printf("%s %s %s %s  \x1b[36m-> %s -> %s %s %s %s\x1b[0m\n",
			rank,
			string(lengthIndicator.Bytes()),
			descAvail,
			v.Title,
			d,
			v.User.Username,
			strconv.Itoa(createdtime.Year()),
			createdtime.Month(),
			strconv.Itoa(createdtime.Day()))
	}
}

// helper functions
func isAllint(slice string) (isAllint bool) {
	isAllint = true
	sliceslice := []byte(slice)
	for _, v := range sliceslice {
		if unicode.IsNumber(rune(v)) == false {
			isAllint = false
		}
	}
	return
}

// start
func main() {
	var p Player
	folderPath, _ := osext.ExecutableFolder()
	p.client_id, _ = ioutil.ReadFile(folderPath + "/client_id.txt")
	p.setting.MinD = 50 * 60 * 1000
	p.setting.MaxD = 500 * 60 * 1000
	println("Please type a search term or 'x' to exit ....")
	r := bufio.NewReader(os.Stdin)
	for {
		i, _, _ := r.ReadLine()
		p.li = string(i)
		switch {
		case p.li == "x":
			p.exit()
		case p.li == "ll":
			p.showResultList()
		case strings.HasPrefix(p.li, "set "):
			p.set()
		case strings.HasPrefix(p.li, "i "):
			p.info()
		case isAllint(p.li):
			go p.killAndPlay()
		case true:
			p.searchSoundCloud()
			p.showResultList()
		}
	}
}
