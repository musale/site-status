package main

import (
	"encoding/xml"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"
)

// siteURL has the sitemap
const siteURL = "https://musale.github.io/sitemap.xml"

// store has the entire store for the sites from sitemap
var store = NewSiteStore()

// SiteStore handles the sites which we check the status for
type SiteStore struct {
	mutex sync.Mutex
	sites []Site
}

// NewSiteStore creates a new instance of SiteStore
func NewSiteStore() *SiteStore {
	return &SiteStore{}
}

// getSites returns the Sites from the sitemap
func (s *SiteStore) getSites() []Site {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.sites
}

// setSites sets the Sites from the sitemap into the store
func (s *SiteStore) setSites(sites []Site) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.sites = sites
}

// fetchSites gets the Sites from the sitemap URL response
func (s *SiteStore) fetchSites() error {
	sitemap, err := http.Get(siteURL)
	if err != nil {
		return err
	}
	defer sitemap.Body.Close()

	vals, err := ioutil.ReadAll(sitemap.Body)

	type siteMapVals struct {
		Name xml.Name `xml:"urlset"`
		URLs []Site   `xml:"url"`
	}
	var smVals siteMapVals
	if err := xml.Unmarshal(vals, &smVals); err != nil {
		return err
	}
	s.setSites(smVals.URLs)
	return nil
}

// fetchSitesStatuses does a check of status for each Site
func (s *SiteStore) fetchSitesStatuses() {
	s.fetchSites()
	sitesCh := make(chan Site)
	sites := []Site{}
	for _, site := range s.sites {
		go func(si Site) {
			sitesCh <- si.checkStatus()
		}(site)
	}
	for i := 0; i < len(s.sites); i++ {
		sites = append(sites, <-sitesCh)
	}
	s.setSites(sites)

}

// Site showing the URL and Up true if it's available
type Site struct {
	URL string `xml:"loc"`
	Up  bool
}

// checkStatus does a check if the URL still returns a 200OK
func (s *Site) checkStatus() Site {
	resp, err := http.Get(s.URL)
	if err != nil {
		s.Up = false
	}
	if resp.StatusCode != http.StatusOK {
		s.Up = false
	} else {
		s.Up = true
	}
	return *s
}

// siteStatusHandler handles the homepage
func siteStatusHandler(w http.ResponseWriter, r *http.Request) {
	t, err := template.ParseFiles("templates/home.html")
	if err != nil {
		http.Error(w, fmt.Sprintf("Error loading templates: %s", err), http.StatusInternalServerError)
	}
	t.Execute(w, store.getSites())
}

func main() {

	go store.fetchSitesStatuses()

	refresh := time.NewTicker(30 * time.Minute)

	go func() {
		for {
			select {
			case <-refresh.C:
				log.Println("Refreshing the sites store")
				store.fetchSitesStatuses()
			}
		}

	}()

	http.HandleFunc("/", siteStatusHandler)
	log.Fatal(http.ListenAndServe(":9090", nil))
}
