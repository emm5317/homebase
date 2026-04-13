package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/emm5317/homebase/internal/cards"
)

type Weather struct {
	apiKey string
	lat    float64
	lon    float64
}

func NewWeather(apiKey string, lat, lon float64) *Weather {
	return &Weather{apiKey: apiKey, lat: lat, lon: lon}
}

func (w *Weather) Name() string            { return "weather" }
func (w *Weather) Interval() time.Duration { return 10 * time.Minute }

func (w *Weather) Fetch(ctx context.Context) ([]cards.Card, error) {
	url := fmt.Sprintf(
		"https://api.openweathermap.org/data/3.0/onecall?lat=%f&lon=%f&appid=%s&units=imperial&exclude=minutely",
		w.lat, w.lon, w.apiKey,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("weather API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("weather API status %d", resp.StatusCode)
	}

	var data owmResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decoding weather: %w", err)
	}

	var result []cards.Card

	// Alert cards (priority 0)
	for i, alert := range data.Alerts {
		result = append(result, cards.Card{
			ID:         fmt.Sprintf("weather-alert-%d", i),
			Source:     "weather",
			Type:       "alert",
			Priority:   0,
			Icon:       alertIcon(alert.Event),
			Title:      alert.Event,
			Body:       truncate(alert.Description, 200),
			AlertLevel: "severe",
			Color:      "#e86f5a",
		})
	}

	// Current conditions card
	current := data.Current
	todayHigh, todayLow := dailyHighLow(data.Daily)
	feelsLike := int(math.Round(current.FeelsLike))
	temp := int(math.Round(current.Temp))
	rainChance := hourlyRainChance(data.Hourly)

	subtitle := fmt.Sprintf("High %d° · Low %d° · Wind %d mph",
		int(math.Round(todayHigh)), int(math.Round(todayLow)), int(math.Round(current.WindSpeed)))

	var weatherStatus string
	var color string
	if rainChance > 60 {
		weatherStatus = "warning"
		color = "#d4943a"
	} else {
		weatherStatus = "ok"
		color = "#4a90d9"
	}

	metrics := []cards.Metric{
		{Label: "Now", Value: fmt.Sprintf("%d°", temp)},
		{Label: "Feels like", Value: fmt.Sprintf("%d°", feelsLike)},
		{Label: "High", Value: fmt.Sprintf("%d°", int(math.Round(todayHigh)))},
		{Label: "Low", Value: fmt.Sprintf("%d°", int(math.Round(todayLow)))},
	}
	if rainChance > 0 {
		metrics = append(metrics, cards.Metric{
			Label: "Rain", Value: fmt.Sprintf("%d%%", rainChance),
		})
	}

	desc := current.Weather[0].Description
	title := fmt.Sprintf("%d° — %s", temp, capitalize(desc))

	result = append(result, cards.Card{
		ID:       "weather-current",
		Source:   "weather",
		Type:     "status",
		Priority: 2,
		Icon:     weatherIcon(current.Weather[0].Icon),
		Title:    title,
		Subtitle: subtitle,
		Status:   weatherStatus,
		Color:    color,
		Metrics:  metrics,
		TimeWindow: &cards.TimeWindow{
			AllDay: true,
		},
	})

	// Weather tip card (contextual)
	if tip := weatherTip(temp, rainChance, current.WindSpeed); tip != "" {
		result = append(result, cards.Card{
			ID:       "weather-tip",
			Source:   "weather",
			Type:     "info",
			Priority: 8,
			Icon:     tipIcon(rainChance, temp),
			Title:    tip,
			Color:    "#d4943a",
			TimeWindow: &cards.TimeWindow{
				ActiveFrom:  "05:00",
				ActiveUntil: "09:00",
			},
		})
	}

	return result, nil
}

// OWM response types
type owmResponse struct {
	Current owmCurrent  `json:"current"`
	Hourly  []owmHourly `json:"hourly"`
	Daily   []owmDaily  `json:"daily"`
	Alerts  []owmAlert  `json:"alerts"`
}

type owmCurrent struct {
	Temp      float64      `json:"temp"`
	FeelsLike float64      `json:"feels_like"`
	WindSpeed float64      `json:"wind_speed"`
	Weather   []owmWeather `json:"weather"`
}

type owmWeather struct {
	Description string `json:"description"`
	Icon        string `json:"icon"`
}

type owmHourly struct {
	Dt  int64   `json:"dt"`
	Pop float64 `json:"pop"` // probability of precipitation 0-1
}

type owmDaily struct {
	Temp struct {
		Min float64 `json:"min"`
		Max float64 `json:"max"`
	} `json:"temp"`
}

type owmAlert struct {
	Event       string `json:"event"`
	Description string `json:"description"`
}

func dailyHighLow(daily []owmDaily) (high, low float64) {
	if len(daily) == 0 {
		return 0, 0
	}
	return daily[0].Temp.Max, daily[0].Temp.Min
}

func hourlyRainChance(hourly []owmHourly) int {
	// Max precipitation probability in next 6 hours
	maxPop := 0.0
	cutoff := time.Now().Add(6 * time.Hour).Unix()
	for _, h := range hourly {
		if h.Dt > cutoff {
			break
		}
		if h.Pop > maxPop {
			maxPop = h.Pop
		}
	}
	return int(math.Round(maxPop * 100))
}

func weatherIcon(owmIcon string) string {
	icons := map[string]string{
		"01d": "\u2600\ufe0f", "01n": "\U0001f319",
		"02d": "\U0001f324\ufe0f", "02n": "\U0001f319",
		"03d": "\u2601\ufe0f", "03n": "\u2601\ufe0f",
		"04d": "\U0001f325\ufe0f", "04n": "\U0001f325\ufe0f",
		"09d": "\U0001f327\ufe0f", "09n": "\U0001f327\ufe0f",
		"10d": "\U0001f326\ufe0f", "10n": "\U0001f327\ufe0f",
		"11d": "\u26c8\ufe0f", "11n": "\u26c8\ufe0f",
		"13d": "\U0001f328\ufe0f", "13n": "\U0001f328\ufe0f",
		"50d": "\U0001f32b\ufe0f", "50n": "\U0001f32b\ufe0f",
	}
	if icon, ok := icons[owmIcon]; ok {
		return icon
	}
	return "\U0001f321\ufe0f"
}

func alertIcon(event string) string {
	return "\u26a0\ufe0f"
}

func weatherTip(temp int, rainChance int, wind float64) string {
	if rainChance >= 60 {
		return "Grab an umbrella today"
	}
	if temp < 32 {
		return "Bundle up — below freezing"
	}
	if temp > 95 {
		return "Stay hydrated — it's hot out there"
	}
	if wind > 25 {
		return "Very windy today — hold onto your hat"
	}
	return ""
}

func tipIcon(rainChance int, temp int) string {
	if rainChance >= 60 {
		return "\u2614"
	}
	if temp < 32 {
		return "\U0001f9e3"
	}
	return "\U0001f321\ufe0f"
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return string(s[0]-32) + s[1:]
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
