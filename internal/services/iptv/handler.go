package iptv

import (
	"fmt"
	"net/http"
	"strconv"
	"vidbot-api/pkg/iptvstore"
	"vidbot-api/pkg/mediaresponse"

	"github.com/gin-gonic/gin"
)

type Handler struct{}

func NewHandler() *Handler {
	return &Handler{}
}

func (h *Handler) GetChannels(c *gin.Context) {
	country := c.Query("country")
	category := c.Query("category")
	streamsOnly := c.Query("streams_only") == "true"

	// validasi country
	if !iptvstore.Default.IsValidCountry(country) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    "INVALID_COUNTRY",
			"message": fmt.Sprintf("Country code '%s' is not valid.", country),
		})
		return
	}

	// validasi category
	if !iptvstore.Default.IsValidCategory(category) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    "INVALID_CATEGORY",
			"message": fmt.Sprintf("Category '%s' is not valid.", category),
		})
		return
	}

	// cek apakah user minta pagination
	pageStr := c.Query("page")
	limitStr := c.Query("limit")
	usePagination := pageStr != "" || limitStr != ""

	channels := iptvstore.Default.GetChannels(country, category, streamsOnly)
	total := len(channels)

	var page, limit, totalPages int

	if usePagination {
		page, _ = strconv.Atoi(pageStr)
		limit, _ = strconv.Atoi(limitStr)
		if page < 1 {
			page = 1
		}
		if limit < 1 || limit > 100 {
			limit = 50
		}
		totalPages = (total + limit - 1) / limit

		start := (page - 1) * limit
		end := start + limit
		if start >= total {
			channels = []iptvstore.Channel{}
		} else {
			if end > total {
				end = total
			}
			channels = channels[start:end]
		}
	}

	data := make([]mediaresponse.IPTVChannel, 0, len(channels))
	for _, ch := range channels {
		streams := iptvstore.Default.GetStreams(ch.ID)
		iptvStreams := make([]mediaresponse.IPTVStream, 0, len(streams))
		for _, st := range streams {
			iptvStreams = append(iptvStreams, mediaresponse.IPTVStream{
				Title:     st.Title,
				URL:       st.URL,
				Quality:   st.Quality,
				UserAgent: st.UserAgent,
				Referrer:  st.Referrer,
			})
		}
		logo := ch.Logo
		if logo == "" {
			logo = iptvstore.Default.GetLogo(ch.ID)
		}
		data = append(data, mediaresponse.IPTVChannel{
			ID:         ch.ID,
			Name:       ch.Name,
			Logo:       logo,
			Country:    ch.Country,
			Categories: ch.Categories,
			Website:    ch.Website,
			Streams:    iptvStreams,
		})
	}

	if usePagination {
		c.JSON(http.StatusOK, mediaresponse.IPTVResponse{
			Success:    true,
			Services:   "iptv",
			Country:    country,
			Category:   category,
			Total:      total,
			Page:       page,
			Limit:      limit,
			TotalPages: totalPages,
			Data:       data,
		})
		return
	}

	c.JSON(http.StatusOK, mediaresponse.IPTVResponse{
		Success:  true,
		Services: "iptv",
		Country:  country,
		Category: category,
		Total:    total,
		Data:     data,
	})
}

func (h *Handler) GetCountries(c *gin.Context) {
	countries := iptvstore.Default.GetCountries()
	data := make([]mediaresponse.IPTVCountry, 0, len(countries))
	for _, ct := range countries {
		data = append(data, mediaresponse.IPTVCountry{
			Name:      ct.Name,
			Code:      ct.Code,
			Languages: ct.Languages,
			Flag:      ct.Flag,
		})
	}
	c.JSON(http.StatusOK, mediaresponse.IPTVCountriesResponse{
		Success:  true,
		Services: "iptv",
		Total:    len(data),
		Data:     data,
	})
}

func (h *Handler) GetCategories(c *gin.Context) {
	categories := iptvstore.Default.GetCategories()
	data := make([]mediaresponse.IPTVCategory, 0, len(categories))
	for _, cat := range categories {
		data = append(data, mediaresponse.IPTVCategory{
			ID:          cat.ID,
			Name:        cat.Name,
			Description: cat.Description,
		})
	}
	c.JSON(http.StatusOK, mediaresponse.IPTVCategoriesResponse{
		Success:  true,
		Services: "iptv",
		Total:    len(data),
		Data:     data,
	})
}
