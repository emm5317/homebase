package providers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	gtfs "github.com/MobilityData/gtfs-realtime-bindings/golang/gtfs"
	"github.com/emm5317/homebase/internal/cards"
	"github.com/emm5317/homebase/internal/config"
	"google.golang.org/protobuf/proto"
)

// Default public Metra GTFS-RT endpoints. See https://metra.com/metra-gtfs-api.
const (
	DefaultMetraTripUpdatesURL = "https://gtfspublic.metrarr.com/gtfs/public/tripupdates"
	DefaultMetraAlertsURL      = "https://gtfspublic.metrarr.com/gtfs/public/alerts"
	DefaultMetraPositionsURL   = "https://gtfspublic.metrarr.com/gtfs/public/positions"
)

// Metra polls the Metra GTFS-Realtime feeds for upcoming trains at the
// configured station heading to the configured destination, plus any active
// service alerts affecting the route.
type Metra struct {
	apiToken        string
	tripUpdatesURL  string
	alertsURL       string
	route           string
	station         string
	destination     string
	loc             *time.Location
	httpClient      *http.Client
	maxDepartures   int
}

func NewMetra(apiToken string, cfg config.MetraConfig, loc *time.Location) *Metra {
	tripURL := cfg.TripUpdatesURL
	if tripURL == "" {
		tripURL = DefaultMetraTripUpdatesURL
	}
	alertsURL := cfg.AlertsURL
	if alertsURL == "" {
		alertsURL = DefaultMetraAlertsURL
	}
	return &Metra{
		apiToken:       apiToken,
		tripUpdatesURL: tripURL,
		alertsURL:      alertsURL,
		route:          strings.ToUpper(cfg.Route),
		station:        strings.ToUpper(cfg.Station),
		destination:    strings.ToUpper(cfg.Destination),
		loc:            loc,
		httpClient:     &http.Client{Timeout: 15 * time.Second},
		maxDepartures:  3,
	}
}

func (m *Metra) Name() string            { return "metra" }
func (m *Metra) Interval() time.Duration { return 60 * time.Second }

func (m *Metra) Fetch(ctx context.Context) ([]cards.Card, error) {
	var result []cards.Card

	feed, err := m.fetchFeed(ctx, m.tripUpdatesURL)
	if err != nil {
		return nil, fmt.Errorf("trip updates: %w", err)
	}

	departures := m.extractDepartures(feed)
	result = append(result, m.buildDepartureCards(departures)...)

	alertFeed, err := m.fetchFeed(ctx, m.alertsURL)
	if err != nil {
		// Alerts are non-critical; return what we have along with the error.
		return result, fmt.Errorf("alerts: %w", err)
	}
	result = append(result, m.buildAlertCards(alertFeed)...)

	return result, nil
}

func (m *Metra) fetchFeed(ctx context.Context, url string) (*gtfs.FeedMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	// Metra's public GTFS API accepts Laravel-style API tokens via the standard
	// Authorization: Bearer <token> header.
	req.Header.Set("Authorization", "Bearer "+m.apiToken)
	req.Header.Set("Accept", "application/x-protobuf")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	feed := &gtfs.FeedMessage{}
	if err := proto.Unmarshal(body, feed); err != nil {
		return nil, fmt.Errorf("decoding protobuf: %w", err)
	}
	return feed, nil
}

// upcomingDeparture is a minimal record of a predicted station departure.
type upcomingDeparture struct {
	tripID      string
	headsign    string
	departsAt   time.Time
	delaySecs   int32
	scheduleRel gtfs.TripUpdate_StopTimeUpdate_ScheduleRelationship
}

func (m *Metra) extractDepartures(feed *gtfs.FeedMessage) []upcomingDeparture {
	if feed == nil {
		return nil
	}
	now := time.Now()
	var out []upcomingDeparture

	for _, entity := range feed.GetEntity() {
		tu := entity.GetTripUpdate()
		if tu == nil {
			continue
		}
		trip := tu.GetTrip()
		if !m.matchesRoute(trip.GetRouteId()) {
			continue
		}
		if !m.matchesDestination(tu) {
			continue
		}

		for _, stu := range tu.GetStopTimeUpdate() {
			if !stationMatches(stu.GetStopId(), m.station) {
				continue
			}
			if stu.GetScheduleRelationship() == gtfs.TripUpdate_StopTimeUpdate_SKIPPED {
				continue
			}
			event := stu.GetDeparture()
			if event == nil {
				event = stu.GetArrival()
			}
			if event == nil || event.GetTime() == 0 {
				continue
			}
			t := time.Unix(event.GetTime(), 0)
			if t.Before(now.Add(-1 * time.Minute)) {
				continue
			}
			out = append(out, upcomingDeparture{
				tripID:      trip.GetTripId(),
				headsign:    m.destination,
				departsAt:   t,
				delaySecs:   event.GetDelay(),
				scheduleRel: stu.GetScheduleRelationship(),
			})
			break // one station-stop per trip
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].departsAt.Before(out[j].departsAt) })
	if len(out) > m.maxDepartures {
		out = out[:m.maxDepartures]
	}
	return out
}

// matchesRoute compares the provided route_id to the configured route with
// case-insensitive prefix matching, since Metra route IDs in the GTFS feed
// sometimes include variant suffixes (e.g. "BNSF", "UP-N").
func (m *Metra) matchesRoute(routeID string) bool {
	if m.route == "" {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(routeID), m.route)
}

// matchesDestination checks whether the trip terminates at the configured
// destination stop (i.e. the last StopTimeUpdate's stop_id matches).
func (m *Metra) matchesDestination(tu *gtfs.TripUpdate) bool {
	if m.destination == "" {
		return true
	}
	updates := tu.GetStopTimeUpdate()
	if len(updates) == 0 {
		return false
	}
	last := updates[len(updates)-1]
	return stationMatches(last.GetStopId(), m.destination)
}

func stationMatches(stopID, target string) bool {
	if target == "" || stopID == "" {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(stopID), target)
}

func (m *Metra) buildDepartureCards(deps []upcomingDeparture) []cards.Card {
	if len(deps) == 0 {
		return nil
	}
	now := time.Now()
	next := deps[0]

	minutesUntil := int(next.departsAt.Sub(now).Round(time.Minute).Minutes())
	if minutesUntil < 0 {
		minutesUntil = 0
	}

	delayMin := int(next.delaySecs / 60)
	status := "ok"
	color := "#4a90d9"
	statusLabel := "On time"
	if delayMin >= 2 {
		status = "warning"
		color = "#d4943a"
		statusLabel = fmt.Sprintf("%d min late", delayMin)
	} else if delayMin <= -2 {
		statusLabel = fmt.Sprintf("%d min early", -delayMin)
	}
	if next.scheduleRel == gtfs.TripUpdate_StopTimeUpdate_SKIPPED {
		status = "warning"
		color = "#e86f5a"
		statusLabel = "Skipped"
	}

	title := fmt.Sprintf("%s → %s in %d min", m.station, m.destination, minutesUntil)
	if minutesUntil == 0 {
		title = fmt.Sprintf("%s → %s now", m.station, m.destination)
	}

	metrics := []cards.Metric{
		{Label: "Depart", Value: next.departsAt.In(m.loc).Format("3:04 PM")},
		{Label: "Status", Value: statusLabel},
	}
	if len(deps) > 1 {
		var following []string
		for _, d := range deps[1:] {
			following = append(following, d.departsAt.In(m.loc).Format("3:04 PM"))
		}
		metrics = append(metrics, cards.Metric{
			Label: "Next",
			Value: strings.Join(following, ", "),
		})
	}

	return []cards.Card{{
		ID:       fmt.Sprintf("metra-%s-%s", m.route, next.tripID),
		Source:   "metra",
		Type:     "status",
		Priority: 3,
		Icon:     "\U0001f686", // 🚆
		Title:    title,
		Subtitle: fmt.Sprintf("%s line", m.route),
		Status:   status,
		Color:    color,
		Metrics:  metrics,
		TimeWindow: &cards.TimeWindow{
			ActiveFrom:  "05:00",
			ActiveUntil: "22:00",
		},
	}}
}

func (m *Metra) buildAlertCards(feed *gtfs.FeedMessage) []cards.Card {
	if feed == nil {
		return nil
	}
	now := time.Now()
	var out []cards.Card

	for _, entity := range feed.GetEntity() {
		alert := entity.GetAlert()
		if alert == nil {
			continue
		}
		if !m.alertAffectsRoute(alert) {
			continue
		}
		if !alertActive(alert, now) {
			continue
		}

		header := translatedText(alert.GetHeaderText())
		desc := translatedText(alert.GetDescriptionText())
		if header == "" && desc == "" {
			continue
		}
		if header == "" {
			header = "Service alert"
		}

		level := "info"
		color := "#d4943a"
		switch alert.GetSeverityLevel() {
		case gtfs.Alert_SEVERE:
			level = "severe"
			color = "#e86f5a"
		case gtfs.Alert_WARNING:
			level = "warning"
			color = "#d4943a"
		}

		out = append(out, cards.Card{
			ID:         "metra-alert-" + entity.GetId(),
			Source:     "metra",
			Type:       "alert",
			Priority:   1,
			Icon:       "\u26a0\ufe0f",
			Title:      header,
			Body:       truncate(desc, 240),
			Subtitle:   fmt.Sprintf("%s line", m.route),
			AlertLevel: level,
			Color:      color,
		})
	}
	return out
}

func (m *Metra) alertAffectsRoute(alert *gtfs.Alert) bool {
	if m.route == "" {
		return true
	}
	for _, sel := range alert.GetInformedEntity() {
		if strings.EqualFold(strings.TrimSpace(sel.GetRouteId()), m.route) {
			return true
		}
	}
	return false
}

func alertActive(alert *gtfs.Alert, now time.Time) bool {
	periods := alert.GetActivePeriod()
	if len(periods) == 0 {
		return true
	}
	nowUnix := uint64(now.Unix())
	for _, p := range periods {
		start := p.GetStart()
		end := p.GetEnd()
		if (start == 0 || nowUnix >= start) && (end == 0 || nowUnix <= end) {
			return true
		}
	}
	return false
}

func translatedText(ts *gtfs.TranslatedString) string {
	if ts == nil {
		return ""
	}
	for _, t := range ts.GetTranslation() {
		lang := strings.ToLower(t.GetLanguage())
		if lang == "" || strings.HasPrefix(lang, "en") {
			return t.GetText()
		}
	}
	// Fall back to the first translation if no English variant is present.
	if tr := ts.GetTranslation(); len(tr) > 0 {
		return tr[0].GetText()
	}
	return ""
}
