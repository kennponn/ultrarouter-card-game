package model

import (
	_ "embed"
	"errors"
	"slices"
	"time"

	"github.com/QuantumNous/new-api/common"
)

var (
	ErrGachaFestivalDate    = errors.New("festival rotation changed")
	ErrGachaFestivalAttempt = errors.New("festival attempt unavailable")
	ErrGachaFestivalAlbum   = errors.New("festival album reward unavailable")
)

type GachaFestivalEvent struct {
	ID     string         `json:"id"`
	Name   string         `json:"name"`
	Kind   string         `json:"kind"`
	Story  string         `json:"story"`
	Labels []string       `json:"labels,omitempty"`
	Values [][]int        `json:"values"`
	Limit  int            `json:"limit,omitempty"`
	Goal   int            `json:"goal,omitempty"`
	Reward map[string]int `json:"reward"`
}

type GachaFestivalRules struct {
	Week   string                 `json:"week"`
	Events []GachaFestivalEvent   `json:"events"`
	Album  map[int]map[string]int `json:"album"`
}

type GachaFestivalLog struct {
	Week    string          `json:"week"`
	Day     int             `json:"day"`
	Tries   map[string]int  `json:"tries"`
	Stamps  map[string]bool `json:"stamps"`
	Claimed map[int]bool    `json:"claimed"`
}

//go:embed gacha_festival.json
var gachaFestivalJSON []byte

var festivalEvents = func() []GachaFestivalEvent {
	var events []GachaFestivalEvent
	if err := common.Unmarshal(gachaFestivalJSON, &events); err != nil {
		panic(err)
	}
	if len(events) != 14 {
		panic("invalid festival calendar")
	}
	return events
}()

var festivalAlbum = gachaCatalog.ActivityPayouts.FestivalAlbum

func syncGachaFestival(state *GachaGameState, now time.Time) error {
	week := gachaWeekKey(now)
	if state.Festival == nil || state.Festival.Week != week {
		state.Festival = &GachaFestivalLog{Week: week, Tries: map[string]int{}, Stamps: map[string]bool{}, Claimed: map[int]bool{}}
	}
	log := state.Festival
	if log.Tries == nil || log.Stamps == nil || log.Claimed == nil {
		return ErrGachaRunState
	}
	for id, tries := range log.Tries {
		if festivalEntry(id) < 0 || tries < 1 || tries > 3 {
			return ErrGachaRunState
		}
	}
	for id, won := range log.Stamps {
		if !won || log.Tries[id] == 0 {
			return ErrGachaRunState
		}
	}
	for threshold, claimed := range log.Claimed {
		if !claimed || festivalAlbum[threshold] == nil || len(log.Stamps) < threshold {
			return ErrGachaRunState
		}
	}
	log.Day = (int(now.In(gameZone).Weekday()) + 6) % 7
	return nil
}

func festivalRotate(cell int) int { return cell%3*3 + 2 - cell/3 }

func festivalEdition(now time.Time) *GachaFestivalRules {
	_, week := now.In(gameZone).ISOWeek()
	edition := &GachaFestivalRules{Week: gachaWeekKey(now), Events: slices.Clone(festivalEvents), Album: festivalAlbum}
	for index := range edition.Events {
		event := &edition.Events[index]
		rows := event.Values
		event.Values = make([][]int, len(rows))
		for pos, row := range rows {
			event.Values[pos] = slices.Clone(row)
		}
		switch event.Kind {
		case "cargo":
			for pos := range rows {
				event.Values[pos] = rows[(pos+week)%len(rows)]
			}
		case "sound":
			for _, buoy := range event.Values {
				for turn := 0; turn < week%4; turn++ {
					buoy[0] = festivalRotate(buoy[0])
				}
			}
		case "relay":
			for pos := range rows[0] {
				event.Values[0][pos] = rows[0][(pos+week)%3]
			}
		case "tide":
			for _, gate := range event.Values {
				gate[1] = (gate[1] + week) % gate[0]
			}
		}
	}
	return edition
}

func festivalEntry(key string) int {
	for index, event := range festivalEvents {
		if event.ID == key {
			return index
		}
	}
	return -1
}

func festivalGuess(event GachaFestivalEvent, choice string) (valid, win bool) {
	var picks []int
	for _, char := range choice {
		if char < '0' || char > '8' {
			return false, false
		}
		picks = append(picks, int(char-'0'))
	}
	switch event.Kind {
	case "cargo":
		if len(picks) != 2 || picks[0] >= 4 || picks[1] >= 4 || picks[0] == picks[1] {
			return false, false
		}
		first, second := event.Values[picks[0]], event.Values[picks[1]]
		return true, first[0]+second[0] <= event.Limit && first[1]+second[1] >= event.Goal
	case "sound":
		if len(picks) != 1 {
			return false, false
		}
		for _, buoy := range event.Values {
			row, column := picks[0]/3-buoy[0]/3, picks[0]%3-buoy[0]%3
			if max(row, -row)+max(column, -column) != buoy[1] {
				return true, false
			}
		}
		return true, true
	case "relay":
		if len(picks) != 3 || picks[0]+picks[1]+picks[2] != event.Limit {
			return false, false
		}
		for pos, target := range event.Values[0] {
			if picks[pos]+picks[(pos+2)%3] != target {
				return true, false
			}
		}
		return true, true
	case "tide":
		if len(picks) != 1 || picks[0] >= 6 {
			return false, false
		}
		for _, gate := range event.Values {
			if (picks[0]+gate[2])%gate[0] != gate[1] {
				return true, false
			}
		}
		return true, true
	}
	return false, false
}

func validateGachaFestivalIntent(intent GachaActionIntent) error {
	if intent.ItemID == nil || intent.Count == nil || intent.GirlID != nil || intent.Squad != nil || intent.Enabled != nil || intent.OpponentID != nil || intent.StakeE != nil || intent.RoomID != nil || intent.QueueID != nil || intent.Config != nil || intent.Materials != nil {
		return ErrGachaIntent
	}
	if intent.Action == "festival_album" && validGachaWeek(*intent.ItemID) && festivalAlbum[*intent.Count] != nil && intent.Choice == nil {
		return nil
	}
	key := *intent.ItemID
	if intent.Action != "festival_play" || intent.Choice == nil || len(*intent.Choice) > 3 || len(key) < 12 || len(key) > 30 || key[10] != ':' || *intent.Count < 1 || *intent.Count > 3 {
		return ErrGachaIntent
	}
	if _, err := time.Parse("2006-01-02", key[:10]); err != nil {
		return ErrGachaIntent
	}
	index := festivalEntry(key[11:])
	if index < 0 {
		return ErrGachaIntent
	}
	if valid, _ := festivalGuess(festivalEvents[index], *intent.Choice); !valid {
		return ErrGachaIntent
	}
	return nil
}

func applyGachaFestival(state *GachaGameState, intent GachaActionIntent, receipt *GachaActionResult, now time.Time) error {
	if err := validateGachaFestivalIntent(intent); err != nil {
		return err
	}
	if err := syncGachaFestival(state, now); err != nil {
		return err
	}
	log, key, count := state.Festival, *intent.ItemID, *intent.Count
	if intent.Action == "festival_album" {
		if key != log.Week {
			return ErrGachaFestivalDate
		}
		if log.Claimed[count] || len(log.Stamps) < count {
			return ErrGachaFestivalAlbum
		}
		if err := grantGachaDispatch(state, festivalAlbum[count], receipt); err != nil {
			return err
		}
		log.Claimed[count] = true
		return nil
	}
	id := key[11:]
	index := festivalEntry(id)
	if key[:10] != gachaDay(now) || index/2 != log.Day {
		return ErrGachaFestivalDate
	}
	if log.Stamps[id] || log.Tries[id]+1 != count {
		return ErrGachaFestivalAttempt
	}
	event := festivalEdition(now).Events[index]
	_, win := festivalGuess(event, *intent.Choice)
	if win {
		if err := grantGachaDispatch(state, event.Reward, receipt); err != nil {
			return err
		}
		log.Stamps[id] = true
	}
	log.Tries[id] = count
	reconcileGachaCaptainDay(state)
	return nil
}
