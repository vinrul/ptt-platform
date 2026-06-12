package geo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"ptt-fleet/services/api-server/internal/apiutil"
)

type Handler struct {
	client            *http.Client
	reverseGeocodeURL string
	routeServiceURL   string
}

func NewHandler(reverseGeocodeURL string, routeServiceURL string) *Handler {
	return &Handler{
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
		reverseGeocodeURL: strings.TrimRight(reverseGeocodeURL, "/"),
		routeServiceURL:   strings.TrimRight(routeServiceURL, "/"),
	}
}

func (h *Handler) Reverse(c *gin.Context) {
	lat, lng, ok := parseLatLng(c)
	if !ok {
		apiutil.Error(c, http.StatusBadRequest, "validation_error", "lat and lng are required", nil)
		return
	}

	baseURL, err := url.Parse(h.reverseGeocodeURL)
	if err != nil {
		apiutil.Error(c, http.StatusInternalServerError, "server_error", "Reverse geocode URL is invalid", nil)
		return
	}
	query := baseURL.Query()
	query.Set("format", "jsonv2")
	query.Set("lat", fmt.Sprintf("%f", lat))
	query.Set("lon", fmt.Sprintf("%f", lng))
	query.Set("zoom", "18")
	query.Set("addressdetails", "1")
	baseURL.RawQuery = query.Encode()

	body, err := h.get(c.Request.Context(), baseURL.String())
	if err != nil {
		apiutil.Error(c, http.StatusBadGateway, "upstream_error", err.Error(), nil)
		return
	}

	var response struct {
		DisplayName string `json:"display_name"`
	}
	if err := json.Unmarshal(body, &response); err != nil || response.DisplayName == "" {
		apiutil.Error(c, http.StatusBadGateway, "upstream_error", "Reverse geocode response is invalid", nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"displayName": response.DisplayName})
}

func (h *Handler) RouteLine(c *gin.Context) {
	var input struct {
		Points []Point `json:"points"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || len(input.Points) < 2 {
		apiutil.Error(c, http.StatusBadRequest, "validation_error", "At least two points are required", nil)
		return
	}

	points := sampledPoints(input.Points)
	coordinates := make([]string, 0, len(points))
	for _, point := range points {
		if !validPoint(point) {
			apiutil.Error(c, http.StatusBadRequest, "validation_error", "Route points are invalid", nil)
			return
		}
		coordinates = append(coordinates, fmt.Sprintf("%f,%f", point.Lng, point.Lat))
	}

	routeURL := fmt.Sprintf(
		"%s/route/v1/driving/%s?overview=full&geometries=geojson&steps=false",
		h.routeServiceURL,
		strings.Join(coordinates, ";"),
	)
	body, err := h.get(c.Request.Context(), routeURL)
	if err != nil {
		apiutil.Error(c, http.StatusBadGateway, "upstream_error", err.Error(), nil)
		return
	}

	var response struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Routes  []struct {
			Geometry struct {
				Coordinates [][]float64 `json:"coordinates"`
			} `json:"geometry"`
		} `json:"routes"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		apiutil.Error(c, http.StatusBadGateway, "upstream_error", "Route response is invalid", nil)
		return
	}
	if response.Code != "" && response.Code != "Ok" {
		apiutil.Error(c, http.StatusBadGateway, "upstream_error", response.Message, nil)
		return
	}
	if len(response.Routes) == 0 {
		apiutil.Error(c, http.StatusBadGateway, "upstream_error", "Route is unavailable", nil)
		return
	}

	c.JSON(http.StatusOK, gin.H{"coordinates": response.Routes[0].Geometry.Coordinates})
}

func (h *Handler) get(ctx context.Context, requestURL string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "PTTFleetAPI/0.1.0 (vinrul/ptt-platform)")

	response, err := h.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return nil, errors.New("upstream returned " + response.Status)
	}
	return bytes.TrimSpace(body), nil
}

type Point struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

func parseLatLng(c *gin.Context) (float64, float64, bool) {
	lat, latErr := parseFloat(c.Query("lat"))
	lng, lngErr := parseFloat(c.Query("lng"))
	return lat, lng, latErr == nil && lngErr == nil && validPoint(Point{Lat: lat, Lng: lng})
}

func parseFloat(value string) (float64, error) {
	return strconv.ParseFloat(value, 64)
}

func validPoint(point Point) bool {
	return !math.IsNaN(point.Lat) &&
		!math.IsInf(point.Lat, 0) &&
		!math.IsNaN(point.Lng) &&
		!math.IsInf(point.Lng, 0) &&
		point.Lat >= -90 &&
		point.Lat <= 90 &&
		point.Lng >= -180 &&
		point.Lng <= 180
}

func sampledPoints(points []Point) []Point {
	if len(points) <= 80 {
		return points
	}
	result := make([]Point, 80)
	interval := float64(len(points)-1) / 79
	for i := range result {
		result[i] = points[int(math.Round(float64(i)*interval))]
	}
	return result
}
