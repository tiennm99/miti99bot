// Package thoitiet is the weather module: /thoitiethomnay (alias /thoitiet),
// /thoitietngaymai, and /thoitiettuannay show today's, tomorrow's, and the
// next 7 days' forecast for a location, Ho Chi Minh City by default. Data
// comes from Open-Meteo, which needs no API key.
package thoitiet

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/tiennm99/miti99bot/internal/log"
	"github.com/tiennm99/miti99bot/internal/modules"
	"github.com/tiennm99/miti99bot/internal/modules/util/chathelper"
)

const (
	locationParameter = "[location...]"
	fetchErrorText    = "Không lấy được dữ liệu thời tiết. Thử lại sau nhé."
)

// errPlaceNotFound means the geocoder had no match for the user's location.
var errPlaceNotFound = errors.New("place not found")

// view is one forecast command's renderer plus the daily rows it reads.
type view struct {
	command string
	minDays int
	render  func(place, forecast) string
}

var (
	todayView    = view{command: "thoitiethomnay", minDays: 1, render: formatToday}
	tomorrowView = view{command: "thoitietngaymai", minDays: 2, render: formatTomorrow}
	weekView     = view{command: "thoitiettuannay", minDays: 1, render: formatWeek}
)

// New is the thoitiet module Factory. The module keeps no state.
func New(_ modules.Deps) modules.Module {
	client := &http.Client{Timeout: httpTimeout}
	command := func(name, description string, v view) modules.Command {
		return modules.Command{
			Name:        name,
			Visibility:  modules.VisibilityPublic,
			Description: description,
			Parameters:  locationParameter,
			Handler:     handler(client, v),
		}
	}
	return modules.Module{
		Commands: []modules.Command{
			command("thoitiethomnay", "Thời tiết hôm nay (mặc định TP.HCM)", todayView),
			command("thoitiet", "Thời tiết hôm nay (mặc định TP.HCM)", todayView),
			command("thoitietngaymai", "Thời tiết ngày mai (mặc định TP.HCM)", tomorrowView),
			command("thoitiettuannay", "Thời tiết 7 ngày tới (mặc định TP.HCM)", weekView),
		},
	}
}

func handler(client *http.Client, v view) modules.CommandHandler {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) error {
		msg := update.Message
		if msg == nil {
			return nil
		}
		query := chathelper.ArgAfterCommand(msg.Text)
		fetchCtx, cancel := chathelper.FetchContext(ctx)
		p, f, err := lookup(fetchCtx, client, query)
		cancel()
		if errors.Is(err, errPlaceNotFound) {
			return chathelper.Reply(ctx, b, msg, fmt.Sprintf("Không tìm thấy địa điểm %q.", query))
		}
		if err == nil && f.Daily.days() < v.minDays {
			err = fmt.Errorf("forecast: %d daily rows, need %d", f.Daily.days(), v.minDays)
		}
		if err != nil {
			log.Error("weather fetch failed", "module", "thoitiet", "command", v.command, "err", err)
			return chathelper.Reply(ctx, b, msg, fetchErrorText)
		}
		return chathelper.Reply(ctx, b, msg, v.render(p, f))
	}
}

// lookup resolves the user's location and fetches its forecast. An empty
// query or a Ho Chi Minh City alias skips geocoding.
func lookup(ctx context.Context, client *http.Client, query string) (place, forecast, error) {
	p, err := resolvePlace(ctx, client, query)
	if err != nil {
		return place{}, forecast{}, err
	}
	f, err := fetchForecast(ctx, client, p)
	return p, f, err
}

func resolvePlace(ctx context.Context, client *http.Client, query string) (place, error) {
	q := normalizeQuery(query)
	if q == "" || hcmAliases[q] {
		return hcmPlace, nil
	}
	if expanded, ok := queryAliases[q]; ok {
		q = expanded
	}
	results, err := geocode(ctx, client, q)
	if err != nil {
		return place{}, err
	}
	p, ok := pickPlace(results)
	if !ok {
		return place{}, errPlaceNotFound
	}
	return p, nil
}
