package iptvstore

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

type Channel struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Logo       string   `json:"logo"`
	Country    string   `json:"country"`
	Categories []string `json:"categories"`
	IsNSFW     bool     `json:"is_nsfw"`
	Website    string   `json:"website"`
}

type Stream struct {
	Channel   string `json:"channel"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	Quality   string `json:"quality"`
	UserAgent string `json:"user_agent"`
	Referrer  string `json:"referrer"`
}

type Country struct {
	Name      string   `json:"name"`
	Code      string   `json:"code"`
	Languages []string `json:"languages"`
	Flag      string   `json:"flag"`
}

type Category struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Store struct {
	mu         sync.RWMutex
	channels   []Channel
	streams    []Stream
	logos      map[string]string
	countries  map[string]Country
	categories map[string]Category

	// pre-built indexes
	channelsByCountry  map[string][]Channel
	streamsByChannelID map[string][]Stream
}

type Logo struct {
	Channel string `json:"channel"`
	URL     string `json:"url"`
}

var Default = &Store{}

func (s *Store) loadCountries() error {
	data, err := fetchJSON("https://iptv-org.github.io/api/countries.json")
	if err != nil {
		return err
	}
	var countries []Country
	if err := json.Unmarshal(data, &countries); err != nil {
		return err
	}
	m := make(map[string]Country, len(countries))
	for _, c := range countries {
		m[c.Code] = c
	}
	s.mu.Lock()
	s.countries = m
	s.mu.Unlock()
	return nil
}

func (s *Store) loadCategories() error {
	data, err := fetchJSON("https://iptv-org.github.io/api/categories.json")
	if err != nil {
		return err
	}
	var categories []Category
	if err := json.Unmarshal(data, &categories); err != nil {
		return err
	}
	m := make(map[string]Category, len(categories))
	for _, c := range categories {
		m[c.ID] = c
	}
	s.mu.Lock()
	s.categories = m
	s.mu.Unlock()
	return nil
}

func (s *Store) IsValidCountry(code string) bool {
	if code == "" {
		return true
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.countries[code]
	return ok
}

func (s *Store) IsValidCategory(id string) bool {
	if id == "" {
		return true
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.categories[id]
	return ok
}

func (s *Store) GetCountries() []Country {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Country, 0, len(s.countries))
	for _, c := range s.countries {
		result = append(result, c)
	}
	return result
}

func (s *Store) GetCategories() []Category {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Category, 0, len(s.categories))
	for _, c := range s.categories {
		result = append(result, c)
	}
	return result
}

func (s *Store) Init() error {
	if err := s.loadChannels(); err != nil {
		return fmt.Errorf("load channels: %w", err)
	}
	if err := s.loadStreams(); err != nil {
		return fmt.Errorf("load streams: %w", err)
	}
	if err := s.loadLogos(); err != nil {
		log.Printf("[iptv] load logos error: %v", err)
	}
	if err := s.loadCountries(); err != nil {
		log.Printf("[iptv] load countries error: %v", err)
	}
	if err := s.loadCategories(); err != nil {
		log.Printf("[iptv] load categories error: %v", err)
	}
	s.buildIndexes()
	go s.startRefresh()

	log.Printf("[iptv] loaded %d channels, %d streams, %d logos, %d countries, %d categories",
		len(s.channels), len(s.streams), len(s.logos), len(s.countries), len(s.categories))
	return nil
}

func (s *Store) loadLogos() error {
	data, err := fetchJSON("https://iptv-org.github.io/api/logos.json")
	if err != nil {
		return err
	}
	var logos []Logo
	if err := json.Unmarshal(data, &logos); err != nil {
		return err
	}
	logoMap := make(map[string]string, len(logos))
	for _, l := range logos {
		if l.URL != "" {
			logoMap[l.Channel] = l.URL
		}
	}
	s.mu.Lock()
	s.logos = logoMap
	s.mu.Unlock()
	return nil
}

func (s *Store) startRefresh() {
	staticTicker := time.NewTicker(3 * 24 * time.Hour)
	streamTicker := time.NewTicker(6 * time.Hour)

	for {
		select {
		case <-staticTicker.C:
			log.Println("[iptv] refreshing static data...")
			if err := s.loadChannels(); err != nil {
				log.Printf("[iptv] refresh channels error: %v", err)
			}
			if err := s.loadLogos(); err != nil {
				log.Printf("[iptv] refresh logos error: %v", err)
			}
			if err := s.loadCountries(); err != nil {
				log.Printf("[iptv] refresh countries error: %v", err)
			}
			if err := s.loadCategories(); err != nil {
				log.Printf("[iptv] refresh categories error: %v", err)
			}
			s.buildIndexes()
			log.Println("[iptv] static data refreshed")
		case <-streamTicker.C:
			log.Println("[iptv] refreshing streams...")
			if err := s.loadStreams(); err != nil {
				log.Printf("[iptv] refresh streams error: %v", err)
			} else {
				s.buildIndexes()
				log.Println("[iptv] streams refreshed")
			}
		}
	}
}

func (s *Store) GetLogo(channelID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.logos[channelID]
}

func (s *Store) loadChannels() error {
	data, err := fetchJSON("https://iptv-org.github.io/api/channels.json")
	if err != nil {
		return err
	}
	var channels []Channel
	if err := json.Unmarshal(data, &channels); err != nil {
		return err
	}
	s.mu.Lock()
	s.channels = channels
	s.mu.Unlock()
	return nil
}

func (s *Store) loadStreams() error {
	data, err := fetchJSON("https://iptv-org.github.io/api/streams.json")
	if err != nil {
		return err
	}
	var streams []Stream
	if err := json.Unmarshal(data, &streams); err != nil {
		return err
	}
	s.mu.Lock()
	s.streams = streams
	s.mu.Unlock()
	return nil
}

func (s *Store) buildIndexes() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// index channels by country
	byCountry := make(map[string][]Channel)
	for _, c := range s.channels {
		byCountry[c.Country] = append(byCountry[c.Country], c)
	}

	// index streams by channel id
	byChannelID := make(map[string][]Stream)
	for _, st := range s.streams {
		if st.Channel == "" {
			continue
		}
		byChannelID[st.Channel] = append(byChannelID[st.Channel], st)
	}

	s.channelsByCountry = byCountry
	s.streamsByChannelID = byChannelID
}

func (s *Store) GetChannels(country, category string, streamsOnly bool) []Channel {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var source []Channel
	if country != "" {
		source = s.channelsByCountry[country]
	} else {
		source = s.channels
	}

	result := []Channel{}
	for _, c := range source {
		if c.IsNSFW {
			continue
		}
		if category != "" && !hasCategory(c.Categories, category) {
			continue
		}
		if streamsOnly && len(s.streamsByChannelID[c.ID]) == 0 {
			continue
		}
		result = append(result, c)
	}
	return result
}

func (s *Store) GetStreams(channelID string) []Stream {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.streamsByChannelID[channelID]
}

func fetchJSON(url string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("fetch %s: HTTP %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func hasCategory(categories []string, target string) bool {
	for _, c := range categories {
		if c == target {
			return true
		}
	}
	return false
}
