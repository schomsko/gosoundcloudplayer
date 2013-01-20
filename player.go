package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
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

type SearchResult struct {
	Title         string
	Duration      int
	Stream_url    string
	Description   string
	Permalink_url string
	Download_url  string
}

type SearchResults []*SearchResult

func (s SearchResults) Len() int      { return len(s) }
func (s SearchResults) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

type ByLength struct{ SearchResults }

func (s ByLength) Less(i, j int) bool {
	return s.SearchResults[i].Duration < s.SearchResults[j].Duration
}

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

var srs SearchResults
var setting = new(Setting)
var client_id string

/*
 * Brace yourself - winter is coming...
 */
func main() {
	var client_id_arr []byte
	client_id_arr, _ = ioutil.ReadFile("./client_id.txt")
	client_id = string( client_id_arr )
	var inputString string
	var vlc *os.Process
	r := bufio.NewReader(os.Stdin)

	setting.setMinD("50")
	setting.setMaxD("500")
	println("Please type a search term or 'x' to exit ...")

	for {
		inputBytes, _, _ := r.ReadLine()
		inputString = string(inputBytes)

		if inputString == "x" {
			if vlc == nil {
				os.Exit(0)
			} else {
				vlc.Kill()
				vlc = nil
			}
		} else if inputString == "ll" {
			showResultList()
		} else if strings.HasPrefix(inputString, "set ") {
			inputString = strings.TrimLeft(inputString, "set ")
			iLsl := strings.Split(inputString, " ")
			if iLsl[0] == "range" {
				setting.setMinD(iLsl[1])
				setting.setMaxD(iLsl[2])
			}
		} else if strings.HasPrefix(inputString, "i ") {
			inputString = strings.TrimLeft(inputString, "i ")
			index, _ := strconv.Atoi(string(inputString))
			println(srs[index].Description)
		} else if isAllint(inputString) {
			vlc = playAndKill(vlc, inputString)
		} else {
			searchSoundCloud(inputString)
		}
	}
}

//
// start and end vlc processes
//
func playAndKill(vlc *os.Process, inputString string) *os.Process {
	inputint, _ := strconv.Atoi(inputString)
	fmt.Fprintf(os.Stdout, "Playing %s ...\n", srs[inputint].Title)
	fmt.Fprintf(os.Stdout, "%s\n", srs[inputint].Permalink_url)
	fmt.Fprintf(os.Stdout, "%s?client_id=%s\n", srs[inputint].Download_url, client_id)
	surl := fmt.Sprintf("%s?client_id=%s", srs[inputint].Stream_url, client_id)
	vlcexe := exec.Command("/Applications/VLC.app/Contents/MacOS/VLC", surl)
	err := vlcexe.Start()

	if err != nil {
		log.Fatal(err)
	} else if vlc != nil {
		vlc.Kill()
	}

	vlc = vlcexe.Process
	return vlc
}

//
// search and fill result object
//
func searchSoundCloud(inputString string) {
	fmt.Fprintf(os.Stdout, "Searching %s ...\n\n", inputString)
	iLs := inputString
	iLs = strings.Replace(iLs, " ", "+", -1)
	query := fmt.Sprintf("http://api.soundcloud.com/tracks.json?" +
			"client_id=%s&q=%s" +
			"&duration[from]=" +
			fmt.Sprint(setting.MinD) +
			"&duration[to]=" +
			fmt.Sprint(setting.MaxD) +
			"&filter=streamable,public", client_id, iLs)
	res, err := http.Get(query)
	if err != nil {
		log.Fatal(err)
	}

	resbody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Fatal(err)
	}
	res.Body.Close()
	err = json.Unmarshal(resbody, &srs)
	sort.Sort(ByLength{srs})
	showResultList()
}

//
// display results
//
func showResultList() {
	var filler string
	for k, v := range srs {
		if k <= 9 {
			filler = " "
		} else {
			filler = ""
		}

		// covert duration
		duration, _ := time.ParseDuration(fmt.Sprintf("%d%s", v.Duration/1000, "s"))
		d := duration.String()

		// build visual duration indicator
		m30 := int(duration.Minutes()) / 30
		indi := bytes.NewBufferString("")
		for i := 0; i < m30; i++ {
			fmt.Fprint(indi, "-")
		}

		// indicate description using info symbol '[i]'
		desc := "    "
		if v.Description != "" {
			desc = " [i]"
		}

		fmt.Printf("%s%d %s>\t %s %s  -> %s\n", filler, k, string(indi.Bytes()), desc, v.Title, d)
	}
}

//
// helper functions
//
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
