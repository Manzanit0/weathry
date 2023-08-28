# Weathry

A v2 of [weatherwarnbot](https://github.com/Manzanit0/weatherwarnbot), because
I'm not too much of a fan of Javascript.

## 🤖 Commands

- `/hourly`, get's the day's forcast broken down by hour windows.
- `/daily`, gets the week's forecast by day windows.
- `/home`, sets your home location so you get proactive notifications.

## 📬 Notifications

When you have you _HOME_ set, you will get notifications on the following events:
- When the temperature raises above 32ºC
- When the temperature decreases below 10ºC
- When it's going to rain

## Features

### Why not allow to save multiple locations?

In my experience, as the main user of the bot, I don't really care about
multiple locations. At least not enough to be getting notifications on a daily
basis.

Sure, it's nice to have some shortcuts to quickly query... but at the same
time, I don't check the weather for other places all so often, so more thought
needs to be put into that UX before actually making a choice and
overcomplicating things.

For what it's worth, the data model _does_ support multiple locations, so
adding support to the bot shouldn't be a big deal. It's all about nailing the
user experience.
