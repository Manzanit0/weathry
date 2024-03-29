package msg

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/manzanit0/weathry/cmd/bot/location"
	"github.com/manzanit0/weathry/pkg/weather"
	"github.com/olekukonko/tablewriter"
)

const (
	MsgLocationQuestionGeneric = "What location do you want me to check the weather for?"
	MsgLocationQuestionWeek    = "What location do you want me to check this week\\'s weather for?"
	MsgLocationQuestionDay     = "What location do you want me to check today\\'s weather for?"
	MsgHomeQuestion            = "What location do you want to save as your home?"
	MsgUnknownText             = "I\\'m not sure what you mean with that\\. Try hitting me up with the /hourly or /daily commands if you need me to check the weather for you ☔️"
	MsgUnableToGetReport       = "I\\'m sorry, the network isn\\'t doing it\\'s best job and I can\\'t get your report just now\\. Please try again in a bit\\."
	MsgUnsupportedInteraction  = "Unsupported type of interaction"
	MsgUnexpectedError         = "Whops\\! Something\\'s not working like it should\\. Try again in a bit\\."
	MsgHelp                    = "👋 Hi %s\\! My name is weathry, great to meet you\\!\n\nI\\'ve been programmed to pretty much help you with any of your weather needs\\. These are some of the things I can do\\:\n\n1\\. /hourly, Check the hourly forcast for you\\.\n2\\. /daily, Check the whole week's forcast for you\\.\n3\\. /home, Keep track of your home so I can send you timely reminders of when there's going to be a weather change\\.\n\nWith regards to the reminders I can send, I just track low and high temperatures and rain\\. This means that if the temperature drops or increases too much in an upcoming day, or it\\'s simply going to rain, then I\\'ll let you know\\."
)

func NewEmojifiedDailyMessage(f []*weather.Forecast) string {
	if len(f) == 0 {
		return "hey, not sure why but I couldn't get any forecasts ¯\\_(ツ)_/¯"
	}

	// TODO: extract from here...
	// we just want the next 3 forecasts
	ff := make([]*weather.Forecast, 3)
	for i := 0; i < 3; i++ {
		ff[i] = f[i]
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Weather Report for %s", f[0].Location))
	for _, v := range f {
		ts := v.FormattedDateTime()
		sb.WriteString(fmt.Sprintf(`
- - - - - - - - - - - - - - - - - - - - - -
📅 %s
🏷 %s
🌡 %0.2f°C - %0.2f°C
💨 %0.2f m/s
💧 %d%%`, ts, v.Description, v.MinimumTemperature, v.MaximumTemperature, v.WindSpeed, v.Humidity))
	}

	sb.WriteString("\n- - - - - - - - - - - - - - - - - - - - - -")

	return sb.String()
}

func NewEmojifiedHourlyMessage(f []*weather.Forecast) string {
	if len(f) == 0 {
		return "hey, not sure why but I couldn't get any forecasts ¯\\_(ツ)_/¯"
	}

	// TODO: extract from here...
	// we just want the next 9 forecasts
	ff := make([]*weather.Forecast, 9)
	for i := 0; i < 9; i++ {
		ff[i] = f[i]
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Weather Report for %s", f[0].Location))
	for _, v := range ff {
		ts, err := v.LocalTime("Europe/Madrid")
		if err != nil {
			ts = v.FormattedDateTime()
		}

		sb.WriteString(fmt.Sprintf(`
- - - - - - - - - - - - - - - - - - - - - -
📅 %s
🏷 %s
🌡 %0.2f°C
💨 %0.2f m/s
💧 %d%%`, ts, v.Description, v.MinimumTemperature, v.WindSpeed, v.Humidity))
	}

	sb.WriteString("\n- - - - - - - - - - - - - - - - - - - - - -")

	return sb.String()
}

type messageOptions struct {
	withTempDiff bool
	withTime     bool
}

type MessageOption func(*messageOptions)

func WithTemperatureDiff() MessageOption {
	return func(config *messageOptions) {
		config.withTempDiff = true
	}
}

func WithTime() MessageOption {
	return func(config *messageOptions) {
		config.withTime = true
	}
}

func NewForecastTableMessage(loc *location.Location, f []*weather.Forecast, opts ...MessageOption) string {
	if len(f) == 0 {
		return "hey, not sure why but I couldn't get any forecasts ¯\\_(ツ)_/¯"
	}

	options := messageOptions{withTempDiff: false, withTime: false}
	for _, f := range opts {
		f(&options)
	}

	b := bytes.NewBuffer([]byte{})
	table := tablewriter.NewWriter(b)

	if options.withTime {
		table.SetHeader([]string{"Time", "Report"})
	} else {
		table.SetHeader([]string{"Date", "Report"})
	}

	for _, v := range f {
		temp := fmt.Sprintf("%.0fºC", v.MinimumTemperature)
		if options.withTempDiff {
			temp = fmt.Sprintf("%.0fºC - %.0fºC", v.MinimumTemperature, v.MaximumTemperature)
		}

		dt := v.FormattedDate()
		if options.withTime {
			dt = v.FormattedTime()
		}

		table.Append([]string{dt, fmt.Sprintf("%s\n%s", v.Description, temp)})
	}

	table.SetRowLine(true)
	table.SetRowSeparator("-")
	table.SetAutoFormatHeaders(false)

	table.Render()

	// we're making the assumption here that all forecasts belong to the same day.
	if options.withTime {
		return fmt.Sprintf("```\n%s  \n%s  \n%s```",
			time.Unix(int64(f[0].DateTimeTS), 0).Format("Mon, 02 Jan 2006"),
			loc.Name,
			b.String(),
		)
	}

	return fmt.Sprintf("```\n%s  \n%s```",
		loc.Name,
		b.String(),
	)
}
