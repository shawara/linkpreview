package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/shawara/goscraper"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"net/http"
	"net/url"
	"sort"
	"time"
)

type Image struct {
	Url    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type Preview struct {
	Name        string  `json:"name"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Icon        string  `json:"icon"`
	Images      []Image `json:"images"`
	Url         string  `json:"url"`
}

type workerData struct {
	Status int
	Data   string
}

type job struct {
	Url    string
	Result chan workerData
}
type apiHandler struct {
}

func (h *apiHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	u := r.FormValue("url")

	w.Header().Set("Server", "ami")
	w.Header().Set("Content-Type", "application/json")

	// to be able to retrieve data from javascript directly
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET")

	_, err := url.Parse(u)

	if err != nil {
		log.Printf("Invalid URL provided: %s", u)
		http.Error(w, "{\"status\": \"error\", \"message\":\"Invalid URL\"}", 500)
		return
	}
	log.Printf("Sending job: %s", u)
	c := make(chan workerData)
	jobPool <- job{Url: u, Result: c}
	data := <-c

	w.WriteHeader(data.Status)
	fmt.Fprintln(w, data.Data)
}

var jobPool chan job

// This is where the work actually happens
func worker(jobs <-chan job) {
	for {

		params := <-jobs
		s, err := goscraper.Scrape(params.Url, 5, "")
		if err != nil {
			params.Result <- workerData{Status: http.StatusBadRequest, Data: "{\"status\": \"error\", \"message\":\"Unable to retrieve information from provided url\"}"}
		} else {
			var pvw Preview
			pvw.Icon = s.Preview.Icon
			pvw.Url = s.Preview.Link
			pvw.Name = s.Preview.Name
			pvw.Title = s.Preview.Title
			pvw.Description = s.Preview.Description

			pvw.Images = []Image{}
			for _, v := range s.Preview.Images {
				conf, err := getImageConfig(v)
				log.Println("error:", err)
				if err != nil {
					pvw.Images = append(pvw.Images, Image{v, 0, 0})
				} else {
					pvw.Images = append(pvw.Images, Image{v, conf.Width, conf.Height})
				}
			}
			sort.Slice(pvw.Images, func(i, j int) bool {
				return pvw.Images[i].Width*pvw.Images[i].Height > pvw.Images[j].Width*pvw.Images[j].Height
			})

			res, _ := json.Marshal(pvw)
			params.Result <- workerData{
				Status: http.StatusOK,
				Data:   string(res),
			}
		}
	}
}

func getImageConfig(url string) (*image.Config, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	config, _, err := image.DecodeConfig(resp.Body)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func main() {
	workerCount := flag.Int("worker_count", 1000, "Amount of workers to start")
	host := flag.String("host", "localhost", "Host to listen on")
	port := flag.Int("port", 8000, "Port to listen on")
	waitTimeout := flag.Int("wait_timeout", 10, "How much time to wait for/fetch response from remote server")

	flag.Parse()

	log.Println("Starting workers:", *workerCount)

	jobPool = make(chan job)
	for i := 0; i < *workerCount; i++ {
		go worker(jobPool)
	}

	log.Println("All workers started. Starting server on port", *port)

	startServer(*host, *port, *waitTimeout)
}
func startServer(host string, port int, waitTimeout int) {
	s := &http.Server{
		Addr:           fmt.Sprintf("%s:%d", host, port),
		Handler:        &apiHandler{},
		ReadTimeout:    time.Duration(waitTimeout) * time.Second,
		WriteTimeout:   time.Duration(waitTimeout) * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	log.Fatal(s.ListenAndServe())
}
